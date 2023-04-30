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

Create the proxmox credentials

```yaml
# config.yaml
config:
  clusters:
    - url: https://cluster-api-1.exmple.com:8006/api2/json
      insecure: false
      token_id: "kubernetes@pve!ccm"
      token_secret: "secret"
      region: cluster-1
```

Upload it to the kubernetes:

```shell
kubectl -n kube-system create secret proxmox-cloud-controller-manager --from-file=config.yaml
```

### Method 1: kubectl

Deploy Proxmox CCM with `cloud-node,cloud-node-lifecycle` controllers

```shell
kubectl apply -f https://raw.githubusercontent.com/sergelogvinov/proxmox-cloud-controller-manager/main/docs/deploy/cloud-controller-manager.yml
```

Deploy Proxmox CCM with `cloud-node-lifecycle` controller (for Talos)

```shell
kubectl apply -f https://raw.githubusercontent.com/sergelogvinov/proxmox-cloud-controller-manager/main/docs/deploy/cloud-controller-manager-talos.yml
```

### Method 2: helm chart

Deploy Proxmox CCM

```shell
helm upgrade -i --namespace=kube-system -f proxmox-ccm.yaml \
		proxmox-cloud-controller-manager charts/proxmox-cloud-controller-manager
```

More options can find [here](charts/proxmox-cloud-controller-manager)

## Contributing

Contributions are welcomed and appreciated!
See [Contributing](CONTRIBUTING.md) for our guidelines.

## License

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
