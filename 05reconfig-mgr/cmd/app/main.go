package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"reconfig-daemon/pkg/inter"
)

type PodInfo struct {
	name      string
	namespace string
	gids      []string // Global ids for devices
}

type ReconfigDaemon struct {
	deviceAlloc     map[string]string // DevID to HostPort mapping
	config          *rest.Config
	clientset       *kubernetes.Clientset
	schedulePods    sets.Set[types.UID]
	ignorePods      sets.Set[types.UID]
	schedulePodInfo map[types.UID]PodInfo
	nodeNameToPort  map[string]string // nodeName to HostPort mapping
	devIF           *inter.FalconInterface
}

func newReconfigDaemon(getResourceEndpoint string, reconfigEndpoint string) *ReconfigDaemon {
	d := &ReconfigDaemon{
		deviceAlloc:     make(map[string]string),
		schedulePods:    sets.New[types.UID](),
		ignorePods:      sets.New[types.UID](),
		schedulePodInfo: make(map[types.UID]PodInfo),
		nodeNameToPort:  make(map[string]string),
		devIF:           inter.NewDevInterface(getResourceEndpoint, reconfigEndpoint),
	}

	var err error
	d.config, err = rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v", err)
	}

	d.clientset, err = kubernetes.NewForConfig(d.config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	return d
}

func (d *ReconfigDaemon) updateDevice() error {
	devices, err := d.devIF.GetAllResource()
	if err != nil {
		return err
	}
	for _, dp := range devices {
		d.deviceAlloc[dp.DevID] = dp.HostPort
	}

	return nil
}

func (d *ReconfigDaemon) assign(portGID string, devGID string) error {
	log.Printf("Assign: devGID %s to portGID %s\n", devGID, portGID)
	ok, err := d.devIF.Assign(portGID, devGID)
	if err != nil || !ok {
		return err
	}
	return nil
}

func (d *ReconfigDaemon) unassign(devGID string) error {
	log.Printf("Unassign devGID %s\n", devGID)
	ok, err := d.devIF.Unassign(devGID)
	if err != nil || !ok {
		return err
	}
	return nil
}

func (d *ReconfigDaemon) getGID(name string, namespace string) []string {
	restClient := d.clientset.CoreV1().RESTClient()
	cmd := []string{
		"sh",
		"-c",
		"echo $DISAG_DEVICES",
	}

	req := restClient.Post().Resource("pods").Name(name).Namespace(namespace).SubResource("exec")

	option := &v1.PodExecOptions{
		Command: cmd,
		Stdin:   false,
		Stdout:  true,
		Stderr:  false,
		TTY:     false,
	}

	req.VersionedParams(
		option,
		scheme.ParameterCodec,
	)

	exec, err := remotecommand.NewSPDYExecutor(d.config, "POST", req.URL())
	if err != nil {
		log.Printf("Failed to create executor: %v", err)
		return []string{"-1"}
	}

	var stdout bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,     // os.Stdin,
		Stdout: &stdout, // os.Stdout,
		Stderr: nil,     // stderr,
	})
	if err != nil {
		log.Printf("Failed to stream command output: %v", err)
		return []string{"-1"}
	}

	envVar := stdout.String()
	if envVar == "" {
		// no gpu
		return []string{"-1"}
	}
	envVar = envVar[:len(envVar)-1] // remove \n
	gids := strings.Split(envVar, ",")
	return gids
}

func (d *ReconfigDaemon) reconfig(nodeName string, demand int) bool {
	log.Printf("Reconfiguring node: %s with demand: %d", nodeName, demand)

	if err := d.updateDevice(); err != nil {
		log.Printf("Failed to update devices: %v", err)
		return false
	}

	usedGPUs := sets.Set[string]{}
	for po := range d.schedulePods {
		usedGPUs.Insert(d.schedulePodInfo[po].gids...)
	}

	var optionGPUs []struct {
		devGID   string
		hostPort string
		score    int
	}

	for dev, nodePort := range d.deviceAlloc {
		// optionGPUs are GPUs that are not used and not connected to the target node
		if !usedGPUs.Has(dev) && nodePort != d.nodeNameToPort[nodeName] {
			optionGPUs = append(optionGPUs, struct {
				devGID   string
				hostPort string
				score    int
			}{dev, nodePort, 0})
		}
	}

	// Calculates score for each GPU option
	gpuCounts := make(map[string]int)
	for _, gpuOption := range optionGPUs {
		gpuCounts[gpuOption.hostPort]++
	}
	for i := range optionGPUs {
		optionGPUs[i].score = gpuCounts[optionGPUs[i].hostPort]
	}

	sort.Slice(optionGPUs, func(i, j int) bool {
		return optionGPUs[i].score < optionGPUs[j].score
	})

	// Performs reconfiguration
	for _, dev := range optionGPUs {
		if demand <= 0 {
			break
		}

		if err := d.unassign(dev.devGID); err != nil {
			log.Printf("Unassign failed for device %s: %v", dev.devGID, err)
			continue
		}

		if err := d.assign(d.nodeNameToPort[nodeName], dev.devGID); err != nil {
			log.Printf("Assign failed for device %s: %v", dev.devGID, err)
			continue
		}

		demand--
	}
	return demand == 0
}

func (d *ReconfigDaemon) podUseFalcon(name string, namespace string) bool {
	po, err := d.clientset.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get pod: %v", err)
		return false
	}
	return po.ObjectMeta.Annotations["use_falcon"] == "true"
}

func (d *ReconfigDaemon) podIsScheduled(name string, namespace string) bool {
	po, err := d.clientset.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get pod: %v", err)
	}
	for _, cond := range po.Status.Conditions {
		if cond.Type == "PodScheduled" {
			return cond.Status == "True"
		}
	}
	return false
}

func (d *ReconfigDaemon) getPodAnnotation(name string, namespace string, annotation string) string {
	po, err := d.clientset.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Failed to get pod: %v", err)
		return ""
	}
	return po.ObjectMeta.Annotations[annotation]
}

func (d *ReconfigDaemon) watchReconfigEvent(eventChan <-chan watch.Event) error {
	for {
		select {
		case event := <-eventChan:
			ev, ok := event.Object.(*v1.Event)
			if !ok {
				log.Printf("Unexpected event object type: %T\n", event.Object)
				continue
			}

			name := ev.InvolvedObject.Name
			namespace := ev.InvolvedObject.Namespace

			curPod, err := d.clientset.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})

			if err != nil || curPod.Status.Phase != "Pending" {
				// skip if the pod does not exist or is already scheduled
				continue
			}

			log.Printf("Reconfig event detected for pod: %s/%s, reason: %s", name, namespace, ev.Reason)

			if err := d.waitReadyToReconfig(name, namespace); err != nil {
				return fmt.Errorf("error waiting the cluster ready: %v", err)
			}
			nodeName := d.getPodAnnotation(name, namespace, "dst_node")
			demand := d.getPodAnnotation(name, namespace, "gpu_demand")
			demandCnt, err := strconv.Atoi(demand)
			if err != nil {
				log.Printf("Invalid GPU demand for pod %s/%s: %v", namespace, name, err)
				continue
			}

			if !d.reconfig(nodeName, demandCnt) {
				log.Printf("Failed to satisfy GPU demand for pod %s/%s", namespace, name)
			}
		}
	}
}

func (d *ReconfigDaemon) waitReadyToReconfig(name string, namespace string) error {
	ready := false
	d.schedulePods = sets.Set[types.UID]{}
	for !ready {
		ready = true
		allPods, err := d.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods: %v", err)
		}
		for _, po := range allPods.Items {
			uid := po.ObjectMeta.UID
			if d.ignorePods.Has(uid) {
				continue
			}
			if (po.ObjectMeta.Name == name) && (po.ObjectMeta.Namespace == namespace) {
				// current pod
				continue
			}
			if !d.podIsScheduled(po.ObjectMeta.Name, po.ObjectMeta.Namespace) {
				continue
			}
			if !d.podUseFalcon(po.ObjectMeta.Name, po.ObjectMeta.Namespace) {
				d.ignorePods.Insert(uid)
				continue
			}
			if (po.Status.Phase == "Succeeded") || (po.Status.Phase == "Failed") {
				d.ignorePods.Insert(uid)
				delete(d.schedulePodInfo, uid)
				continue
			}
			d.schedulePods.Insert(uid)

			if po.Status.Phase == "Running" && len(d.schedulePodInfo[uid].gids) == 0 {
				info := PodInfo{
					name:      po.ObjectMeta.Name,
					namespace: po.ObjectMeta.Namespace,
					gids:      d.getGID(po.ObjectMeta.Name, po.ObjectMeta.Namespace),
				}
				d.schedulePodInfo[uid] = info
			} else if po.Status.Phase == "Pending" {
				log.Printf("Not ready due to Pod %s\n", po.ObjectMeta.Name)
				ready = false
			}
		}
	}
	return nil
}

func main() {
	var configPath string = "/etc/kubernetes/reconfig-mgr-config.yaml"
	buf, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config map[string]string
	if err := yaml.Unmarshal(buf, &config); err != nil {
		log.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Extracts configuration values difined in deploy/helm/falcon/values.yaml
	nodeNamesList := strings.Split(config["node_names"], ",")
	hostPortList := strings.Split(config["host_ports"], ",")
	getResourceEndpoint := config["get_rec_endpoint"]
	reconfigEndpoint := config["reconfig_endpoint"]

	d := newReconfigDaemon(getResourceEndpoint, reconfigEndpoint)
	log.Println("Reconfig-Mgr starts.")

	if len(nodeNamesList) != len(hostPortList) {
		log.Fatalf("Host port or Node name is missing")
	}
	for i, name := range nodeNamesList {
		d.nodeNameToPort[name] = hostPortList[i]
	}

	d.updateDevice()

	opts := metav1.ListOptions{
		FieldSelector: "involvedObject.kind=Pod",
		Watch:         true,
	}
	watcher, err := d.clientset.CoreV1().Events("").Watch(context.TODO(), opts)
	if err != nil {
		panic(err.Error())
	}

	reconfigChan := make(chan watch.Event)

	go func() {
		err := d.watchReconfigEvent(reconfigChan)
		if err != nil {
			log.Printf("watchReconfigEvent err: %v\n", err)
		}
	}()

	for {
		select {
		case event := <-watcher.ResultChan():
			ev, _ := event.Object.(*v1.Event)
			if ev.Reason == "Reconfig" {
				reconfigChan <- event
			}
		}
	}
}
