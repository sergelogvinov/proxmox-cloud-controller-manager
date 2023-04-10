# Kubernetes cloud controller manager for Proxmox

To me, it seems like Proxmox is a bit old-fashioned when it comes to creating virtual machines.
It doesn't have a lot of automation built-in, so you have to do a lot of things manually.
Proxmox is a good option if you have a static infrastructure or don't create new virtual machines very often.

I use Terraform to launch my Kubernetes nodes.
However, when I need to scale down the cluster, I have to manually delete the corresponding node resource in Kubernetes.
That's why I created the CCM (Cloud Controller Manager) for Proxmox.
Originally, it was designed to work with [Talos CCM](https://github.com/siderolabs/talos-cloud-controller-manager), but it was not difficult to make it as standalone solution.

The CCM does a few things: it initialises new nodes, applies common labels to them, and removes them when they're deleted. It also supports multiple clusters, meaning you can have one kubernetes cluster across multiple Proxmox clusters.

The basic definitions:
* kubernetes `region` is a Proxmox cluster `clusters[].region`
* kubernetes `zone` is a hypervisor host machine name

This makes it possible for me to use pods affinity/anti-affinity.

## Example

```yaml
# cloud provider config
clusters:
  - url: https://cluster-api-1.exmple.com:8006/api2/json
    insecure: false
    token_id: "user!token-id"
    token_secret: "secret"
    region: cluster-1
  - url: https://cluster-api-2.exmple.com:8006/api2/json
    insecure: false
    token_id: "user!token-id"
    token_secret: "secret"
    region: cluster-2
```

Node spec result:

```yaml
apiVersion: v1
kind: Node
metadata:
  labels:
    ...
    node.kubernetes.io/instance-type: 2VCPU-2GB
    topology.kubernetes.io/region: cluster-1
    topology.kubernetes.io/zone: pve-node-1
  name: worker-1
spec:
  ...
  providerID: proxmox://cluster-1/123
status:
  addresses:
  - address: 172.16.0.31
    type: InternalIP
  - address: worker-1
    type: Hostname
```

# Install

## Create a token

Official [documentation](https://pve.proxmox.com/wiki/User_Management)

```shell
# Create role CCM
pveum role add CCM -privs "VM.Audit"
# Create user and grant permissions
pveum user add kubernetes@pve
pveum aclmod / -user kubernetes@pve -role CCM
pveum user token add kubernetes@pve ccm -privsep 0
```

## Deploy CCM

### Method 1: kubectl

Deploy Proxmox CCM

```shell
kubectl apply -f https://raw.githubusercontent.com/sergelogvinov/proxmox-cloud-controller-manager/main/docs/deploy/cloud-controller-manager.yml
```

Change the proxmox credentials

```shell
kubectl -n kube-system edit secrets proxmox-cloud-controller-manager
```

### Method 2: helm chart

Set the proxmox credentials

```yaml
# clusters.yaml
config:
  clusters:
    - url: https://cluster-api-1.exmple.com:8006/api2/json
      insecure: false
      token_id: "kubernetes@pve!ccm"
      token_secret: "secret"
      region: cluster-1
```

Deploy Proxmox CCM

```shell
helm upgrade -i --namespace=kube-system -f clusters.yaml proxmox-cloud-controller-manager charts/proxmox-cloud-controller-manager
```
