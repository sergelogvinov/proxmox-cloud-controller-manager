# proxmox-cloud-controller-manager

![Version: 0.1.2](https://img.shields.io/badge/Version-0.1.2-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.0.1](https://img.shields.io/badge/AppVersion-v0.0.1-informational?style=flat-square)

A Helm chart for Kubernetes

**Homepage:** <https://github.com/sergelogvinov/proxmox-cloud-controller-manager>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| sergelogvinov |  |  |

## Source Code

* <https://github.com/sergelogvinov/proxmox-cloud-controller-manager>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity for data pods assignment. ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity |
| config.clusters | list | `[]` |  |
| enabledControllers | list | `["cloud-node","cloud-node-lifecycle"]` | List of controllers should be enabled. Use '*' to enable all controllers. Support only `cloud-node,cloud-node-lifecycle` controllers. |
| extraArgs | list | `[]` | Any extra arguments for talos-cloud-controller-manager |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/sergelogvinov/proxmox-cloud-controller-manager"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| logVerbosityLevel | int | `2` | Log verbosity level. See https://github.com/kubernetes/community/blob/master/contributors/devel/sig-instrumentation/logging.md for description of individual verbosity levels. |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` | Node labels for data pods assignment. ref: https://kubernetes.io/docs/user-guide/node-selection/ |
| podAnnotations | object | `{}` | Annotations for data pods. ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/ |
| podSecurityContext | object | `{"fsGroup":10258,"fsGroupChangePolicy":"OnRootMismatch","runAsGroup":10258,"runAsNonRoot":true,"runAsUser":10258}` | Pods Security Context. ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod |
| priorityClassName | string | `"system-cluster-critical"` | CCM pods' priorityClassName. |
| replicaCount | int | `1` |  |
| resources.requests.cpu | string | `"10m"` |  |
| resources.requests.memory | string | `"32Mi"` |  |
| securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"seccompProfile":{"type":"RuntimeDefault"}}` | Container Security Context. ref: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod |
| serviceAccount | object | `{"annotations":{},"create":true,"name":""}` | Pods Service Account. ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/ |
| tolerations | list | `[{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane","operator":"Exists"},{"effect":"NoSchedule","key":"node.cloudprovider.kubernetes.io/uninitialized","operator":"Exists"}]` | Tolerations for data pods assignment. ref: https://kubernetes.io/docs/concepts/configuration/taint-and-toleration/ |
| updateStrategy | object | `{"rollingUpdate":{"maxUnavailable":1},"type":"RollingUpdate"}` | Deployment update stategy type. ref: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#updating-a-deployment |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.11.0](https://github.com/norwoodj/helm-docs/releases/v1.11.0)