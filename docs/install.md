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

### Optional

```shell
# ${IP} can be single or comma-separated list of two IPs (dual stack).
kubelet --node-ip=${IP}
```
If your node has __multiple IP addresses__, you may need to set the `--node-ip` flag in the kubelet arguments to specify which IP address the kubelet should use.
This ensures that the correct IP address is used for communication between the node and other components in the Kubernetes cluster, especially in environments where multiple network interfaces or IP addresses are present.

```shell
# ${ID} has format proxmox://$REGION/$VMID.
kubelet --provider-id=${ID}
```
If CCM cannot define VMID, you may need to set the `--provider-id` flag in the kubelet arguments to specify the VM ID in Proxmox. This ensures that the CCM can manage the node by VM ID.

```shell
# ${NODENAME} is the name of the node.
kubelet --hostname-override=${NODENAME}
```
If your node has a different hostname than the one registered in the cluster, you may need to set the `--hostname-override` flag in the kubelet arguments to specify the correct hostname.


## Create a Proxmox token

Official [documentation](https://pve.proxmox.com/wiki/User_Management)

```shell
# Create role CCM
pveum role add CCM -privs "VM.Audit VM.GuestAgent.Audit Sys.Audit"
# Create user and grant permissions
pveum user add kubernetes@pve
pveum aclmod / -user kubernetes@pve -role CCM
pveum user token add kubernetes@pve ccm -privsep 0
```

Or through terraform:

```hcl
# Plugin: bpg/proxmox

resource "proxmox_virtual_environment_role" "ccm" {
  role_id = "CCM"

  privileges = [
    "Sys.Audit",
    "VM.Audit",
    "VM.GuestAgent.Audit",
  ]
}

resource "proxmox_virtual_environment_user" "kubernetes" {
  acl {
    path      = "/"
    propagate = true
    role_id   = proxmox_virtual_environment_role.ccm.role_id
  }

  comment = "Kubernetes"
  user_id = "kubernetes@pve"
}

resource "proxmox_virtual_environment_user_token" "ccm" {
  comment    = "Kubernetes CCM"
  token_name = "ccm"
  user_id    = proxmox_virtual_environment_user.kubernetes.user_id
}

resource "proxmox_virtual_environment_acl" "ccm" {
  token_id = proxmox_virtual_environment_user_token.ccm.id
  role_id  = proxmox_virtual_environment_role.ccm.role_id

  path      = "/"
  propagate = true
}
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

See [configuration documentation](config.md) for more details.

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

## Troubleshooting

How `kubelet` works with flag `cloud-provider=external`:

1. kubelet join the cluster and send the `Node` object to the API server.
Node object has values:
    * `node.cloudprovider.kubernetes.io/uninitialized` taint.
    * `alpha.kubernetes.io/provided-node-ip` annotation with the node IP.
    * `nodeInfo` field with system information.
2. CCM detects the new node and sends a request to the Proxmox API to get the VM configuration. Like VMID, hostname, etc.
3. CCM updates the `Node` object with labels, taints and `providerID` field. The `providerID` is immutable and has the format `proxmox://$REGION/$VMID`, it cannot be changed after the first update.
4. CCM removes the `node.cloudprovider.kubernetes.io/uninitialized` taint.

If `kubelet` does not have `cloud-provider=external` flag, kubelet will expect that no external CCM is running and will try to manage the node lifecycle by itself.
This can cause issues with Proxmox CCM.
So, CCM will skip the node and will not update the `Node` object.

If you modify the `kubelet` flags, it's recommended to check all workloads in the cluster.
Please __delete__ the node resource first, and __restart__ the kubelet.

The steps to troubleshoot the Proxmox CCM:
1. scale down the CCM deployment to 1 replica.
2. set log level to `--v=5` in the deployment.
3. check the logs
4. check kubelet flag `--cloud-provider=external`, delete the node resource and restart the kubelet.
5. check the logs
6. wait for 1 minute. If CCM cannot reach the Proxmox API, it will log the error.
7. check tains, labels, and providerID in the `Node` object.
