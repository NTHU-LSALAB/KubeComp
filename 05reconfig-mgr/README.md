# Reconfig-Mgr
Reconfigure Manager handles the reconfiguration requests (events) triggered by the KubeComp Scheduler.

## Quick Start
```shell
make # compile locally
make buildImage # build the docker image
make pushImage # push the image to the kind cluster
make deploy # deploy in K8S
make remove # remove from K8S
```

## Configuration
written in `chart/values.yaml`
- get_rec_endpoint: the endpoint to get all the resource allocation
- reconfig_endpoint: the endpoint to reconfigure the resource
- node_names: name of Kubernetes nodes, which can be figured out by `kubectl get node`
- host_ports: the ports that the Kubernetes nodes connected to

For example, the definition below indicates that tkind-worker is connected to host port 1.

```
node_names: kind-worker,kind-worker2,kind-worker3
host_ports: 1,2,3
```