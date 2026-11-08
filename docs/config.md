# Cloud controller manager configuration file

This file is used to configure the Proxmox CCM.

```yaml
features:
  # Provider type
  provider: default|capmox
  # Network options
  network:
    # Network mode
    mode: default|qemu|auto
    # Enable or disable the IPv6 support
    ipv6_support_disabled: true|false
    # External IP address CIDRs list, comma-separated
    # Use `!` to exclude a CIDR
    external_ip_cidrs: '192.168.0.0/16,2001:db8:85a3::8a2e:370:7334/112,!fd00:1234:5678::/64'
    # IP addresses sort order, comma-separated
    # The IPs that do not match the CIDRs will be kept in the order they
    # were detected.
    ip_sort_order: '192.168.0.0/16,2001:db8:85a3::8a2e:370:7334/112'
  # Enable use of Proxmox HA group as a zone label
  ha_group: true|false
  # Update the topology region/zone labels when the VM moves to another zone
  force_update_labels: true|false

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
* `ca_file` - (optional) Path to a CA certificate file used to verify the Proxmox API certificate.
* `token_id` - The Proxmox API token ID.
* `token_secret` - The Proxmox API token secret.
* `token_id_file`, `token_secret_file` - Paths to files that contain the token ID and secret, instead of `token_id` and `token_secret`.
* `username`, `password` - Proxmox user credentials, as an alternative to an API token.
* `region` - The name of the region, which is also used as `topology.kubernetes.io/region` label.

## Feature flags

* `provider` - Set the provider type. The default is `default`, which uses provider-id format `proxmox://<region>/<vm-id>`. The `capmox` value is used for working with the Cluster API for Proxmox (CAPMox), which uses provider-id format `proxmox://<SystemUUID>`.
* `network.mode` - Defines how the network addresses are handled by the CCM. The default value is `default`, which uses the kubelet argument `--node-ip` to assign IPs to the node resource. The `qemu` mode uses the QEMU agent API to retrieve network addresses from the virtual machine, while auto attempts to detect the best mode automatically.
* `network.ipv6_support_disabled` - Set to `true` to ignore any IPv6 addresses. The default is `false`.
* `network.external_ip_cidrs` - A comma-separated list of external IP address CIDRs. You can use `!` to exclude a CIDR from the list. This is useful for defining which IPs should be considered external and not included in the node addresses.
* `network.ip_sort_order` - A comma-separated list defining the order in which IP addresses should be sorted. The IPs that do not match the CIDRs will be kept in the order they were detected. You can use `!` to exclude a CIDR from the node addresses.
* `ha_group` - Set to `true` to enable the use of Proxmox HA group as a zone label. The default is `false`.
* `force_update_labels` - Set to `true` to update the `topology.kubernetes.io/region` and `topology.kubernetes.io/zone` labels on initialized nodes when the VM is migrated to a different zone. The default is `false`.

For more information about the network modes, see the [Networking documentation](networking.md).
