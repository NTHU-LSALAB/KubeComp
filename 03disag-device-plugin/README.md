# Disaggregated Device Plugin
The disaggregated device plugin was initially designed for the Falcon 4005 GPU chassis. To make it available to users without the specific device, we provide this framework, and the Falcon resource is substituted with our simulated resource pool.

## Quick Start
```shell
make # compile locally
make buildImage # build the docker image
make pushImage # push the image to the kind cluster
make deploy # deploy in K8S
make remove # remove from K8S
```

## Configuration
written in `charts/values.yaml`
- api_endpoint: the endpoint to get the resource
- local_ips: internal IP of Kubernetes nodes, which can be figured out by `kubectl get node -o wide`
- host_ports: the ports that the Kubernetes nodes connected to

For example, the definition below indicates that the node with IP 172.18.0.5 is connected to host port 1.

```
local_ips: 172.18.0.5,172.18.0.2,172.18.0.3
host_ports: 1,2,3
```

## Verification
If the Disaggregated Device Plugin is successfully deployed, `falcon.com/gpu` can be found in nodes' Capacity and Allocatable.