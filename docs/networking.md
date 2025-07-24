# Networking

## Node Addressing modes

There are three node addressing modes that Proxmox CCM supports:
 - Default mode (only mode available till v0.9.0)
 - Auto mode (available from vX.X.X)
 - QEMU-only Mode

In Default mode Proxmox CCM expects nodes to be provided with their private IP Address via the `--node-ip` kubelet flag. Default mode
*does not* set the External IP of the node.

In Auto mode, Proxmox CCM makes use of both the host-networking access (if available) and the QEMU guest agent API (if available) to determine the available IP Addresses. At a minimum Auto mode will set only the Internal IP addresses of the node but can be configured to know which IP Addresses should be treated as external based on provided CIDRs and what order ALL IP addresses should be sorted in according to a sort order CIDR.

> [!NOTE]
> All modes, including Default Mode, will use any IPs provided via the `alpha.kubernetes.io/provided-node-ip` annotation, unless they are part of the ignored cidrs list (non-default modes only).

### Default Mode

In Default Mode, Proxmox CCM assumes that the private IP of the node will be set using the kubelet arg `--node-ip`. Setting this flag adds an annotation to the node `alpha.kubernetes.io/provided-node-ip` which is used to then set the Node's `status.Addresses` field.

In this mode there is no validation of the IP address.

### Auto Mode

In Auto mode, Proxmox CCM uses access to the QEMU guest agent API (if available) to get a list of interfaces and IP Addresses as well as any IP addresses provided via `--node-ip`. From there depending on configuration it will setup all detected addresses as private and set any addresses matching a configured set of external CIDRs as external.

Enabling auto mode is done by setting the network feature mode to `auto`:

```yaml
features:
  network:
    mode: auto
```

### QEMU-only Mode

In QEMU Mode, Proxmox CCM uses the QEMU guest agent API to retrieve a list of IP addresses and set them as Node Addresses. Any node addresses provided via the `alpha.kubernetes.io/provided-node-ip` node annotation will also be available.

Enabling qemu-only mode is done by setting the network feature mode to `qemu`:

```yaml
features:
  network:
    mode: qemu
```

## Example configuration

The following is example configuration which sets IP addresses from 192.168.0.1 - 192.168.255.254 and 2001:0db8:85a3:0000:0000:8a2e:0370:0000 - 2001:0db8:85a3:0000:0000:8a2e:0370:ffff as "external" addresses. All other IPs from subnet 10.0.0.0/8 will be ignored.

To use any mode other than default specify the following configuration:

```yaml
features:
  network:
    mode: auto
    external_ip_cidrs: '192.168.0.0/16,2001:db8:85a3::8a2e:370:7334/112,!10.0.0.0/8'
```

Further configuration options are available as well. We can disable ipv6 support entirely and provide an order to sort IP addresses in (with any that don't match just being kept in whatever order the make it into the list):

```yaml
features:
  network:
    mode: auto
    ipv6_support_disabled: true
    ip_sort_order: '192.168.0.0/16,2001:db8:85a3::8a2e:370:7334/112'
```
