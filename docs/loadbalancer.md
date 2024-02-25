# Loadbalancer on top of the Proxmox cluster

Set up a load balancer to distribute traffic across multiple proxmox nodes.
We use the [haproxy](https://hub.docker.com/_/haproxy) image to create a simple load balancer on top of the proxmox cluster.
First, we need to create a headless service and set endpoints.

```yaml
# proxmox-service.yaml
---
apiVersion: v1
kind: Service
metadata:
  name: proxmox
  namespace: kube-system
spec:
  clusterIP: None
  ports:
    - name: https
      protocol: TCP
      port: 8006
      targetPort: 8006
---
apiVersion: v1
kind: Endpoints
metadata:
  name: proxmox
  namespace: kube-system
subsets:
  - addresses:
      - ip: 192.168.0.1
      - ip: 192.168.0.2
    ports:
      - port: 8006
```

Apply the configuration to the cluster.

```bash
kubectl apply -f proxmox-service.yaml
```

Second, we need to deploy proxmox CCM with sidecar load balancer.
Haproxy will resolve the `proxmox.kube-system.svc.cluster.local` service and uses IPs from the endpoints to distribute traffic.
Proxmox CCM will use the `proxmox.domain.com` domain to connect to the proxmox cluster wich is resolved to the load balancer IP (127.0.0.1).

```yaml
# Helm Chart values

config:
  clusters:
    - region: cluster
      url: https://proxmox.domain.com:8006/api2/json
      insecure: true
      token_id: kubernetes@pve!ccm
      token_secret: 11111111-1111-1111-1111-111111111111

hostAliases:
  - ip: 127.0.0.1
    hostnames:
      - proxmox.domain.com

initContainers:
  - name: loadbalancer
    restartPolicy: Always
    image: ghcr.io/sergelogvinov/haproxy:2.8.6-alpine3.19
    imagePullPolicy: IfNotPresent
    env:
      - name: SVC
        value: proxmox.kube-system.svc.cluster.local
      - name: PORT
        value: "8006"
    securityContext:
      runAsUser: 99
      runAsGroup: 99
    resources:
      limits:
        cpu: 50m
        memory: 64Mi
      requests:
        cpu: 50m
        memory: 32Mi
```
