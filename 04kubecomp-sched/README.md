# KubeComp Scheduler
KubeComp Scheduler is designed to apply specific scheduling policies based on the disaggregated device.

## Quick Start
```shell
make # compile locally
make buildImage # build the docker image
make pushImage # push the image to the kind cluster
make deploy # deploy in K8S
make remove # remove from K8S
```

## Usage
Specify KubeComp Scheduler for pods by
```yaml
spec:
  schedulerName: kubecomp-scheduler
```