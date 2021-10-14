# Everoute CNI QuickStart

# Background
Everoute CNI provides standard K8S CNI function, 
including network support for POD communication,
and standard NetworkPolicy network strategy

## Prerequisites
### Kubernetes cluster
+ support version: `v1.16.0 ~ v1.21.5`.
+ When deploying a cluster with kubeadm the `--pod-network-cidr <cidr>` option must be specified.
+ Open vSwitch kernel module must be present on every Kubernetes node.

### Remove old CNI (if exist)
1. follow the uninstall spec from old CNI
2. Check resources have been removed, pods, crds etc.
3. Check config file in `/etc/cni/net.d/` has been removed. `!important`

## Image
### Build image
```shell
git clone https://github.com/everoute/everoute.git
cd everoute
make images
```
images need to be manually distributed to each node.

### Public docker hub (recommended)
hub.docker.io - everoute/release

The latest version of everoute CNI is `v0.9.1` current

## Deployment
By default, we will get the latest version of everoute.
```shell
wget https://raw.githubusercontent.com/everoute/everoute/main/deploy/everoute.yaml
kubectl apply -f everoute.yaml
```

## Check
By default, everoute has one controller in whole cluster and one agent on each node
use `kubectl get pods -n kube-system | grep everoute` to check if all the related pods are running
### Sample output
```text
everoute-agent-7v596                   2/2     Running   0          4h30m
everoute-agent-kmzl8                   2/2     Running   0          4h30m
everoute-agent-q8qq2                   2/2     Running   0          4h30m
everoute-controller-57fc7bc784-xcm9s   1/1     Running   0          4h30m
```
### Check case
Pending