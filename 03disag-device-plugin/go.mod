module my-device-plugin

go 1.13

replace (
	k8s.io/api => k8s.io/api v0.26.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.26.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.2-rc.0
	k8s.io/apiserver => k8s.io/apiserver v0.26.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.26.1
	k8s.io/client-go => k8s.io/client-go v0.26.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.26.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.26.1
	k8s.io/code-generator => k8s.io/code-generator v0.26.2-rc.0
	k8s.io/component-base => k8s.io/component-base v0.26.1
	k8s.io/cri-api => k8s.io/cri-api v0.26.2-rc.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.26.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.26.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.26.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.26.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.26.1
	k8s.io/kubelet => k8s.io/kubelet v0.26.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.26.1
	k8s.io/metrics => k8s.io/metrics v0.26.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.26.1
)

require (
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	google.golang.org/grpc v1.49.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/kubelet v0.26.1
)

replace k8s.io/component-helpers => k8s.io/component-helpers v0.26.1

replace k8s.io/controller-manager => k8s.io/controller-manager v0.26.1

replace k8s.io/dynamic-resource-allocation => k8s.io/dynamic-resource-allocation v0.26.1

replace k8s.io/kms => k8s.io/kms v0.26.2-rc.0

replace k8s.io/kubectl => k8s.io/kubectl v0.26.1

replace k8s.io/mount-utils => k8s.io/mount-utils v0.26.2-rc.0

replace k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.26.1

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.26.1

replace k8s.io/sample-controller => k8s.io/sample-controller v0.26.1
