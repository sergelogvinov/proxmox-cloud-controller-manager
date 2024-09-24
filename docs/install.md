# Install

Proxmox Cloud Controller Manager (CCM) supports controllers:
* cloud-node
* cloud-node-lifecycle

`cloud-node` - detects new node launched in the cluster and registers them in the cluster.
Assigns labels and taints based on Proxmox VM configuration.

`cloud-node-lifecycle` - detects node deletion on Proxmox side and removes them from the cluster.

## Requirements

You need to set `--cloud-provider=external` in the kubelet argument for all nodes in the cluster.
The flag informs the kubelet to offload cloud-specific responsibilities to this external component like Proxmox CCM.

```shell
kubelet --cloud-provider=external
```

Otherwise, kubelet will attempt to manage the node's lifecycle by itself, which can cause issues in environments using an external Cloud Controller Manager (CCM).

If your node has __multiple IP addresses__, you may need to set the `--node-ip` flag in the kubelet arguments to specify which IP address the kubelet should use. This ensures that the correct IP address is used for communication between the node and other components in the Kubernetes cluster, especially in environments where multiple network interfaces or IP addresses are present.

```shell
kubelet --node-ip=${IP}
```

IP can be single or comma-separated list of two IPs (dual stack).

## Create a Proxmox token

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

Create the proxmox credentials config file:

```yaml
clusters:
  # List of Proxmox clusters, region mast be unique
  - url: https://cluster-api-1.exmple.com:8006/api2/json
    insecure: false
    token_id: "kubernetes@pve!ccm"
    # Token from the previous step
    token_secret: "secret"
    # Region name, can be any string, it will use as for kubernetes topology.kubernetes.io/region label
    region: cluster-1
```

### Method 1: kubectl

Upload it to the kubernetes:

```shell
kubectl -n kube-system create secret generic proxmox-cloud-controller-manager --from-file=config.yaml
```

Deploy Proxmox CCM with `cloud-node,cloud-node-lifecycle` controllers

```shell
kubectl apply -f https://raw.githubusercontent.com/sergelogvinov/proxmox-cloud-controller-manager/main/docs/deploy/cloud-controller-manager.yml
```

Deploy Proxmox CCM with `cloud-node-lifecycle` controller (for Talos)

```shell
kubectl apply -f https://raw.githubusercontent.com/sergelogvinov/proxmox-cloud-controller-manager/main/docs/deploy/cloud-controller-manager-talos.yml
```

### Method 2: helm chart

Create the config file

```yaml
# proxmox-ccm.yaml
config:
  clusters:
    - url: https://cluster-api-1.exmple.com:8006/api2/json
      insecure: false
      token_id: "kubernetes@pve!ccm"
      token_secret: "secret"
      region: cluster-1
```

Deploy Proxmox CCM (deployment mode)

```shell
helm upgrade -i --namespace=kube-system -f proxmox-ccm.yaml \
    proxmox-cloud-controller-manager \
    oci://ghcr.io/sergelogvinov/charts/proxmox-cloud-controller-manager
```

Deploy Proxmox CCM (daemonset mode)

It makes sense to deploy on all control-plane nodes. Do not forget to set the nodeSelector.

```shell
helm upgrade -i --namespace=kube-system -f proxmox-ccm.yaml \
    --set useDaemonSet=true \
    proxmox-cloud-controller-manager \
    oci://ghcr.io/sergelogvinov/charts/proxmox-cloud-controller-manager
```

More options you can find [here](charts/proxmox-cloud-controller-manager)

## Deploy CCM (Rancher)

Official [documentation](https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/kubernetes-clusters-in-rancher-setup/node-requirements-for-rancher-managed-clusters)

Rancher RKE2 configuration:

```yaml
machineGlobalConfig:
  # Kubelet predefined value --cloud-provider=external
  cloud-provider-name: external
  # Disable Rancher CCM
  disable-cloud-controller: true
```

Create the helm values file:

```yaml
# proxmox-ccm.yaml
config:
  clusters:
    - url: https://cluster-api-1.exmple.com:8006/api2/json
      insecure: false
      token_id: "kubernetes@pve!ccm"
      token_secret: "secret"
      region: cluster-1

# Use host resolv.conf to resolve proxmox connection url
useDaemonSet: true

# Set nodeSelector in daemonset mode is required
nodeSelector:
  node-role.kubernetes.io/control-plane: ""
```

Deploy Proxmox CCM (daemondset mode)

```shell
helm upgrade -i --namespace=kube-system -f proxmox-ccm.yaml \
    proxmox-cloud-controller-manager \
    oci://ghcr.io/sergelogvinov/charts/proxmox-cloud-controller-manager
```

## Deploy CCM with load balancer (optional)

This optional setup to improve the Proxmox API availability.

See [load balancer](loadbalancer.md) for installation instructions.
