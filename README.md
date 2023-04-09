# Proxmox Cloud Controller Manager

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

## Install

### kubectl

```shell
kubectl apply -f https://raw.githubusercontent.com/sergelogvinov/proxmox-cloud-controller-manager/main/docs/deploy/cloud-controller-manager.yml
```

### Helm install

```shell
helm upgrade -i --namespace=kube-system proxmox-cloud-controller-manager charts/proxmox-cloud-controller-manager
```
