package falconresources

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	typedv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// FalconResources is a plugin that see the GPU as a composable device
type FalconResources struct {
	handle framework.Handle
	k8scli *kubernetes.Clientset
}

var _ framework.PreFilterPlugin = &FalconResources{}
var _ framework.ScorePlugin = &FalconResources{}
var _ framework.PermitPlugin = &FalconResources{}

const (
	Name                  string = "FalconResources" // name of the plugin used in Registry and configurations
	perDeviceReconfigTime int    = 5
)

func (gp *FalconResources) Name() string {
	return Name
}

// Initializes and returns a new FalconResources plugin
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %v", err)
	}

	k8scli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	return &FalconResources{
		handle: h,
		k8scli: k8scli,
	}, nil
}

// Filters the pod if the gpu count in the "gpu pool" is less than the required amount
func (gp *FalconResources) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	// If the required GPUs exceed the available GPUs, return failure early.
	requiredFalconQuantity := pod.Spec.Containers[0].Resources.Requests["falcon.com/gpu"]
	requiredFalcon, _ := requiredFalconQuantity.AsInt64()

	nodeinfos, _ := gp.handle.SnapshotSharedLister().NodeInfos().List()
	totalFalcon := int64(0)
	for _, nodeinfo := range nodeinfos {
		totalFalcon += (nodeinfo.Allocatable.ScalarResources["falcon.com/gpu"] - nodeinfo.Requested.ScalarResources["falcon.com/gpu"])
	}

	log.Printf("Pod %s requires %d GPU(s), and currently has %d GPU(s) in total\n", pod.Name, requiredFalcon, totalFalcon)

	patchAnnotations := map[string]interface{}{
		"metadata": map[string]map[string]string{
			"annotations": {
				"use_falcon": "true",
			},
		},
	}

	// The pod doesn't use Falcon
	if requiredFalcon == 0 {
		patchAnnotations = map[string]interface{}{
			"metadata": map[string]map[string]string{
				"annotations": {
					"use_falcon": "false",
				},
			},
		}
	}

	// Total gpu is less than required, so it's useless to reconfigure
	if totalFalcon < requiredFalcon {
		reason := fmt.Sprintf("Pod %s requires %d GPU but only %d GPU in the pool.", pod.Name, requiredFalcon, totalFalcon)
		return nil, framework.NewStatus(framework.Unschedulable, reason)
	}

	if err := gp.patchPodAnnotations(ctx, pod.Namespace, pod.Name, patchAnnotations); err != nil {
		return nil, framework.AsStatus(fmt.Errorf("failed to patch pod annotations: %v", err))
	}

	return nil, framework.NewStatus(framework.Success, "")
}

// Returns a PreFilterExtensions interface if the plugin implements one
func (gp *FalconResources) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// Invokes at the score extension point
func (gp *FalconResources) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	nodeInfo, err := gp.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("failed to get node %q from Snapshot: %v", nodeName, err))
	}

	requiredFalconQuantity := pod.Spec.Containers[0].Resources.Requests["falcon.com/gpu"]
	requiredFalcon, _ := requiredFalconQuantity.AsInt64()
	localFalcon := (nodeInfo.Allocatable.ScalarResources["falcon.com/gpu"] - nodeInfo.Requested.ScalarResources["falcon.com/gpu"])

	var score int64 = 0
	if localFalcon > requiredFalcon {
		score = int64(requiredFalcon * 100 / localFalcon)
	} else if localFalcon == requiredFalcon {
		score = 100
	} else {
		score = localFalcon - requiredFalcon
	}

	log.Printf("Node %s has %d GPU(s), %s requires %d GPU(s) -> score: %d", nodeName, localFalcon, pod.Name, requiredFalcon, score)
	return score, nil
}

func (gp *FalconResources) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	// Finds highest and lowest scores
	var highest int64 = -math.MaxInt64
	var lowest int64 = math.MaxInt64
	for _, nodeScore := range scores {
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
	}

	// Transforms the highest to lowest score range to fit the framework's min to max node score range.
	oldRange := highest - lowest
	newRange := framework.MaxNodeScore - framework.MinNodeScore
	for i, nodeScore := range scores {
		if oldRange == 0 {
			scores[i].Score = framework.MinNodeScore
		} else {
			scores[i].Score = ((nodeScore.Score - lowest) * newRange / oldRange) + framework.MinNodeScore
		}
	}

	return nil
}

func (gp *FalconResources) ScoreExtensions() framework.ScoreExtensions {
	return gp
}

func (gp *FalconResources) getGpuDemand(ctx context.Context, pod *v1.Pod, nodeName string) int {
	node, err := gp.k8scli.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		log.Printf("failed to get node %q: %v", nodeName, err)
		return 0
	}

	allocGPUQuantity := node.Status.Allocatable["falcon.com/gpu"]
	allocGPU, _ := allocGPUQuantity.AsInt64()

	nodeInfo, err := gp.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err)
		return 0
	}
	requestGPU := nodeInfo.Requested.ScalarResources["falcon.com/gpu"]
	requiredFalconQuantity := pod.Spec.Containers[0].Resources.Requests["falcon.com/gpu"]
	requiredFalcon, _ := requiredFalconQuantity.AsInt64()

	demand := requiredFalcon - (allocGPU - requestGPU)
	if demand > 0 {
		return int(demand)
	}
	return 0
}

func (gp *FalconResources) Permit(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	retStatus := framework.NewStatus(framework.Success)
	waitTime := time.Duration(0)

	demand := gp.getGpuDemand(ctx, pod, nodeName)
	if demand <= 0 {
		return retStatus, waitTime
	}

	retStatus = framework.NewStatus(framework.Unschedulable)
	// Annotates destination node with demand info
	patchAnnotations := map[string]interface{}{
		"metadata": map[string]map[string]string{
			"annotations": {
				"dst_node":   nodeName,
				"gpu_demand": strconv.Itoa(demand),
			},
		},
	}

	if err := gp.patchPodAnnotations(ctx, pod.Namespace, pod.Name, patchAnnotations); err != nil {
		return framework.NewStatus(framework.Error, "failed to patch pod annotations"), waitTime
	}

	// Log and create an event indicating the pod needs reconfiguration
	gp.createPodEvent(pod, "Reconfig", fmt.Sprintf("Pod %v needs reconfiguration", pod.Name))

	// Wait for the reconfiguration to complete within a set time frame
	startTime := time.Now()
	for time.Since(startTime) <= (time.Duration(15+demand*perDeviceReconfigTime))*time.Second {
		num := gp.getGpuDemand(ctx, pod, nodeName)
		if num == 0 {
			return framework.NewStatus(framework.Success), waitTime
		}
		time.Sleep(1 * time.Second)
	}

	return retStatus, waitTime
}

// Helper function to patch pod annotations
func (gp *FalconResources) patchPodAnnotations(ctx context.Context, namespace, podName string, annotations map[string]interface{}) error {
	patchBytes, err := json.Marshal(annotations)
	if err != nil {
		return fmt.Errorf("failed to marshal annotations: %v", err)
	}
	_, err = gp.k8scli.CoreV1().Pods(namespace).Patch(ctx, podName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	return err
}

func (gp *FalconResources) createPodEvent(pod *v1.Pod, reason, message string) {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme)

	eventBroadcaster := record.NewBroadcaster()
	defer eventBroadcaster.Shutdown()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedv1core.EventSinkImpl{Interface: gp.k8scli.CoreV1().Events("")})
	eventRecorder := eventBroadcaster.NewRecorder(scheme, v1.EventSource{})
	eventRecorder.Event(pod, v1.EventTypeNormal, reason, message)
}
