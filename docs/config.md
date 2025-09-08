# Cloud controller manager configuration file

This file is used to configure the Proxmox CCM.

```yaml
features:
  # Provider type
  provider: default|capmox
  # Network mode
  network: default|qemu|auto
  # Enable or disable the IPv6 support
  ipv6_support_disabled: true|false
  # External IP address CIDRs list, comma-separated
  # Use `!` to exclude a CIDR
  external_ip_cidrs: '192.168.0.0/16,2001:db8:85a3::8a2e:370:7334/112,!fd00:1234:5678::/64'
  # IP addresses sort order, comma-separated
  # The IPs that do not match the CIDRs will be kept in the order they
  # were detected.
  ip_sort_order: '192.168.0.0/16,2001:db8:85a3::8a2e:370:7334/112'

clusters:
  # List of Proxmox clusters
  - url: https://cluster-api-1.exmple.com:8006/api2/json
    # Skip the certificate verification, if needed
    insecure: false
    # Proxmox api token
    token_id: "kubernetes-csi@pve!csi"
    token_secret: "secret"
    # (optional) Proxmox api token from separate file (s. Helm README.md)
    # token_id_file: /run/secrets/region-1/token_id
    # token_secret_file: /run/secrets/region-1/token_secret
    # Region name, which is cluster name
    region: Region-1

  # Add more clusters if needed
  - url: https://cluster-api-2.exmple.com:8006/api2/json
    insecure: false
    token_id: "kubernetes-csi@pve!csi"
    token_secret: "secret"
    region: Region-2
```

## Cluster list

You can define multiple clusters in the `clusters` section.

* `url` - The URL of the Proxmox cluster API.
* `insecure` - Set to `true` to skip TLS certificate verification.
* `token_id` - The Proxmox API token ID.
* `token_secret` - The name of the Kubernetes Secret that contains the Proxmox API token.
* `region` - The name of the region, which is also used as `topology.kubernetes.io/region` label.

## Feature flags

* `provider` - Set the provider type. The default is `default`, which uses provider-id format `proxmox://<region>/<vm-id>`. The `capmox` value is used for working with the Cluster API for Proxmox (CAPMox), which uses provider-id format `proxmox://<SystemUUID>`.
* `network` - Defines how the network addresses are handled by the CCM. The default value is `default`, which uses the kubelet argument `--node-ips` to assign IPs to the node resource. The `qemu` mode uses the QEMU agent API to retrieve network addresses from the virtual machine, while auto attempts to detect the best mode automatically.
* `ipv6_support_disabled` - Set to `true` to ignore any IPv6 addresses. The default is `false`.
* `external_ip_cidrs` - A comma-separated list of external IP address CIDRs. You can use `!` to exclude a CIDR from the list. This is useful for defining which IPs should be considered external and not included in the node addresses.


For more information about the network modes, see the [Networking documentation](networking.md).
