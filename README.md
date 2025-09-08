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
* kubernetes label `topology.kubernetes.io/region` is a Proxmox cluster `clusters[].region`
* kubernetes label `topology.kubernetes.io/zone` is a hypervisor host machine name

This makes it possible for me to use pods affinity/anti-affinity.

## Example

```yaml
# cloud provider config
clusters:
  - url: https://cluster-api-1.exmple.com:8006/api2/json
    insecure: false
    # Proxox auth token
    token_id: "user!token-id"
    token_secret: "secret"
    # Uniq region name
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
    # Type generated base on CPU and RAM
    node.kubernetes.io/instance-type: 2VCPU-2GB
    # Proxmox cluster name as in the config
    topology.kubernetes.io/region: cluster-1
    # Proxmox hypervisor host machine name
    topology.kubernetes.io/zone: pve-node-1

    # Proxmox specific labels
    topology.proxmox.sinextra.dev/region: cluster-1
    topology.proxmox.sinextra.dev/node: pve-node-1
    topology.proxmox.sinextra.dev/ha-group: default

  name: worker-1
spec:
  ...
  # providerID - magic string:
  #   cluster-1 - cluster name as in the config
  #   123 - Proxmox VM ID
  providerID: proxmox://cluster-1/123
status:
  addresses:
  - address: 172.16.0.31
    type: InternalIP
  - address: worker-1
    type: Hostname
```

## Install

See [Install](docs/install.md) for installation instructions.

## Controllers

Support controllers:

* cloud-node
  * Updates node resource.
  * Assigns labels and taints based on Proxmox VM configuration.
* cloud-node-lifecycle
  * Cleans up node resource when Proxmox VM is deleted.

## FAQ

See [FAQ](docs/faq.md) for answers to common questions.

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

---

`Proxmox®` is a registered trademark of [Proxmox Server Solutions GmbH](https://www.proxmox.com/en/about/company).
