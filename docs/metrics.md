# Metrics documentation

This document is a reflection of the current state of the exposed metrics of the Proxmox CCM.

## Gather metrics

By default, the Proxmox CCM exposes metrics on the `https://localhost:10258/metrics` endpoint.

```yaml
proxmox-cloud-controller-manager --authorization-always-allow-paths="/metrics" --secure-port=10258
```

### Helm chart values

The following values can be set in the Helm chart to expose the metrics of the Talos CCM.

```yaml
podAnnotations:
  prometheus.io/scrape: "true"
  prometheus.io/scheme: "https"
  prometheus.io/port: "10258"
```

## Metrics exposed by the CCM

### Proxmox API calls

|Metric name|Metric type|Labels/tags|
|-----------|-----------|-----------|
|proxmox_api_request_duration_seconds|Histogram|`request`=<api_request>|
|proxmox_api_request_errors_total|Counter|`request`=<api_request>|

Example output:

```txt
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="0.1"} 13
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="0.25"} 172
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="0.5"} 199
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="1"} 210
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="2.5"} 210
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="5"} 210
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="10"} 210
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="30"} 210
proxmox_api_request_duration_seconds_bucket{request="getVmInfo",le="+Inf"} 210
proxmox_api_request_duration_seconds_sum{request="getVmInfo"} 39.698945394000006
proxmox_api_request_duration_seconds_count{request="getVmInfo"} 210
```
