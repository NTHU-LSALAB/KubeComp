package inter

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type FalconInterface struct {
	endpoint string
	hostPort string
}

type DevicePair struct {
	DevID   string
	GpuUUID string
}

func NewDevInterface() *FalconInterface {
	// Read device-plugin-config.yaml
	var devicePluginConfigPath string = "/etc/kubernetes/device-plugin-config.yaml"
	buf, err := os.ReadFile(devicePluginConfigPath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config map[string]string
	if err := yaml.Unmarshal(buf, &config); err != nil {
		log.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Extracts configuration values defined in charts/values.yaml
	ipList := strings.Split(config["local_ips"], ",")
	hostPortList := strings.Split(config["host_ports"], ",")
	endpoint := config["api_endpoint"]
	nodeIP := os.Getenv("NODE_IP")

	var hostPort string
	for i, ip := range ipList {
		if ip == nodeIP {
			hostPort = hostPortList[i]
			break
		}
	}

	if hostPort == "" || endpoint == "" {
		log.Fatalf("Host port or endpoint is missing")
	}

	log.Infof("Node IP: %s", nodeIP)
	log.Infof("Host Port: %s", hostPort)

	return &FalconInterface{
		endpoint: endpoint,
		hostPort: hostPort,
	}
}

// Retrieves the list of devices from the resource pool API.
func (fi *FalconInterface) GetResource() ([]DevicePair, error) {
	req, err := http.NewRequest(http.MethodGet, fi.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %v", err)
	}

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making HTTP request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	// Parses the result
	var result []map[string]string
	if err := json.Unmarshal([]byte(string(body)), &result); err != nil {
		return nil, fmt.Errorf("error unmarshalling JSON: %v", err)
	}

	// Returns the devices connected to the host
	var devices []DevicePair
	for _, res := range result {
		if res["hostport"] == fi.hostPort {
			devices = append(devices, DevicePair{
				DevID:   res["devid"],
				GpuUUID: res["uuid"],
			})
		}
	}
	return devices, nil
}
