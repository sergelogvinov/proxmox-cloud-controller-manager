# Fast answers to common questions

## Dose CCM support online VM migration?

No.
Proxmox CCM uses [Cloud-Provider](https://github.com/kubernetes/cloud-provider.git) framework, which does not support label updates after the node initialization.

Kuernetes has node drain feature, which can be used to move pods from one node to another.
