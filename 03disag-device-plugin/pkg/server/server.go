package server

import (
	"context"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"my-device-plugin/pkg/inter"
)

const (
	resourceName           string = "falcon.com/gpu"
	falconSocket           string = "falcon.sock"
	KubeletSocket          string = "kubelet.sock"
	DevicePluginPath       string = "/var/lib/kubelet/device-plugins/"
	DevicePluginConfigPath string = "/etc/kubernetes/device-plugin-config.yaml"
)

// DisagDevServer is a device plugin server
type DisagDevServer struct {
	srv                 *grpc.Server
	devices             map[string]*pluginapi.Device
	ctx                 context.Context
	cancel              context.CancelFunc
	gpuLookUp           map[string]string
	deviceCheckInterval time.Duration
	devIF               *inter.FalconInterface
}

func NewDisagDevServer() *DisagDevServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &DisagDevServer{
		devices:             make(map[string]*pluginapi.Device),
		srv:                 grpc.NewServer(grpc.EmptyServerOption{}),
		ctx:                 ctx,
		cancel:              cancel,
		gpuLookUp:           make(map[string]string),
		deviceCheckInterval: 1 * time.Second,
		devIF:               inter.NewDevInterface(),
	}
}

func (s *DisagDevServer) Run() error {
	if err := s.listDevice(); err != nil {
		log.Fatalf("Failed to list devices: %v", err)
	}

	pluginapi.RegisterDevicePluginServer(s.srv, s)
	if err := syscall.Unlink(filepath.Join(DevicePluginPath, falconSocket)); err != nil && !os.IsNotExist(err) {
		return err
	}

	l, err := net.Listen("unix", filepath.Join(DevicePluginPath, falconSocket))
	if err != nil {
		return err
	}

	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			log.Printf("Start GPPC server for '%s'", resourceName)
			err = s.srv.Serve(l)
			if err == nil {
				break
			}

			log.Printf("GRPC server for '%s' crashed with error: %v", resourceName, err)

			if restartCount > 5 {
				log.Fatalf("GRPC server for '%s' has repeatedly crashed recently. Quitting", resourceName)
			}

			if time.Since(lastCrashTime).Seconds() > 3600 {
				restartCount = 1
			} else {
				restartCount++
			}
			lastCrashTime = time.Now()
		}
	}()

	// Wait for server to start by lauching a blocking connection
	conn, err := s.dial(falconSocket, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	return nil
}

// Registers to Kubelet
func (s *DisagDevServer) RegisterToKubelet() error {
	socketFile := filepath.Join(DevicePluginPath + KubeletSocket)
	conn, err := s.dial(socketFile, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(DevicePluginPath + falconSocket),
		ResourceName: resourceName,
	}
	log.Infof("Register to kubelet with endpoint %s", req.Endpoint)

	if _, err := client.Register(context.Background(), req); err != nil {
		return err
	}

	return nil
}

// GetDevicePluginOptions returns options to be communicated with Device Manager
func (s *DisagDevServer) GetDevicePluginOptions(ctx context.Context, e *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{PreStartRequired: true}, nil
}

// ListAndWatch returns a stream of List of Devices
func (s *DisagDevServer) ListAndWatch(e *pluginapi.Empty, srv pluginapi.DevicePlugin_ListAndWatchServer) error {
	devs := make([]*pluginapi.Device, 0, len(s.devices))
	for _, dev := range s.devices {
		devs = append(devs, dev)
	}

	if err := srv.Send(&pluginapi.ListAndWatchResponse{Devices: devs}); err != nil {
		log.Errorf("Failed to send devices: %v", err)
		return err
	}

	old_devs := make([]*pluginapi.Device, len(s.devices))
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			if err := s.listDevice(); err != nil {
				continue
			}

			// Updates the device only when the devices change
			devs := make([]*pluginapi.Device, len(s.devices))
			keys := make([]string, len(s.devices))

			i := 0
			for k := range s.devices {
				keys[i] = k
				i++
			}
			sort.Strings(keys)
			for i, k := range keys {
				devs[i] = s.devices[k]
			}

			if !reflect.DeepEqual(devs, old_devs) {
				if err := srv.Send(&pluginapi.ListAndWatchResponse{Devices: devs}); err != nil {
					log.Errorf("Failed to send updated device list: %v", err)
				}
				copy(old_devs, devs)
			}
		}
		<-time.After(s.deviceCheckInterval)
	}
}

// Allocate is called during container creation so that the Device
// Plugin can run device specific operations and instruct Kubelet
// of the steps to make the Device available in the container
func (s *DisagDevServer) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	log.Infoln("Allocate called")
	resps := &pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		log.Infof("Received request: %v", strings.Join(req.DevicesIDs, ","))
		gpuIDs := make([]string, len(req.DevicesIDs))
		for i, devID := range req.DevicesIDs {
			gpuIDs[i] = s.gpuLookUp[devID]
		}

		resp := pluginapi.ContainerAllocateResponse{
			Envs: map[string]string{
				"DISAG_DEVICES":          strings.Join(req.DevicesIDs, ","),
				"NVIDIA_VISIBLE_DEVICES": strings.Join(gpuIDs, ","),
			},
		}
		resps.ContainerResponses = append(resps.ContainerResponses, &resp)
	}
	return resps, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registeration phase,
// before each container start. Device plugin can run device specific operations
// such as reseting the device before making devices available to the container
func (s *DisagDevServer) PreStartContainer(ctx context.Context, req *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (s *DisagDevServer) GetPreferredAllocation(context.Context, *v1beta1.PreferredAllocationRequest) (*v1beta1.PreferredAllocationResponse, error) {
	return &v1beta1.PreferredAllocationResponse{}, nil
}

func (s *DisagDevServer) listDevice() error {
	s.devices = make(map[string]*pluginapi.Device)
	devices, _ := s.devIF.GetResource()

	for _, dp := range devices {
		s.gpuLookUp[dp.DevID] = dp.GpuUUID
		s.devices[dp.GpuUUID] = &pluginapi.Device{
			ID:     dp.DevID,
			Health: pluginapi.Healthy,
		}
	}
	return nil
}

func (s *DisagDevServer) dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}
