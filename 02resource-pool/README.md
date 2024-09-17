# Resource Pool Simulator
This is a single resource pool simulator. Devices inside the resource pool can be dynamically assigned to any hosts through APIs. The server listens on port 8000 by default.
## Configuration File
Each line should follow the form `devid,hostport`. The example below shows that device 1, 2 are connected to host 1 while device 3, 4 are connected to host 2. If `hostport` is not specified, for examle, device 5, then it means the device is not connected to any host yet.
```
1,1
2,1
3,2
4,2
5,
```
## Deployment
- Quick Start
    - `go run resource-pool.go resource-config.txt`
- Deploy on K8S
    - `docker build -t resource-pool .`
    - `kind load docker-image resource-pool`
    - `kubectl create -f deploy.yaml`
## API
- GET /resources
    - This API shows all the devices and host ports.
- POST /allocation
    - This API allows the clients to assign the devices to the host port.
    - keys
        - devid
        - hostport
- DELETE /allocation
    - This API allows the clients to unassign the devices from the host port.
    - keys
        - devid
