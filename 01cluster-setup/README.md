# Cluster Setup
- Create a four-node (three workers) cluster 
    - `kind create cluster --config config.yaml`
- Create a `kubecomp` namespace
    - `kubectl create -f ns.yaml`