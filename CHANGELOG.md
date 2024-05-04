
<a name="v0.4.2"></a>
## [v0.4.2](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.4.0...v0.4.2) (2024-05-04)

Welcome to the v0.4.2 release of Kubernetes cloud controller manager for Proxmox!

### Features

- **chart:** add daemonset mode
- **chart:** add hostAliases and initContainers

### Changelog

* c02bc2f chore: release v0.4.1
* ce92b3e feat(chart): add daemonset mode
* 4771769 chore: bump deps
* 12d2858 ci: update multi arch build init
* 3c7cd44 ci: update multi arch build init
* 36757fc ci: update multi arch build init
* c1ab34c chore: bump deps
* d1e6e70 docs: update helm install command
* 9ba9ff2 feat(chart): add hostAliases and initContainers

<a name="v0.4.0"></a>
## [v0.4.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.3.0...v0.4.0) (2024-02-16)

Welcome to the v0.4.0 release of Kubernetes cloud controller manager for Proxmox!

### Bug Fixes

- init provider

### Features

- kubelet dualstack support

### Changelog

* 677e6cc chore: release v0.4.0
* a752d10 feat: kubelet dualstack support
* de55986 fix: init provider
* 10592d1 chore: bump deps
* 7b73b5f refactor: move providerID to the package
* 6f0c667 chore: release v0.3.0

<a name="v0.3.0"></a>
## [v0.3.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.2.0...v0.3.0) (2024-01-03)

Welcome to the v0.3.0 release of Kubernetes cloud controller manager for Proxmox!

### Bug Fixes

- namespace for extension-apiserver-authentication rolebinding

### Features

- can use user/password
- **chart:** add extraVolumes + extraVolumeMounts

### Changelog

* ac2f564 feat: can use user/password
* 41a7f8d chore: bump deps
* 74d8c78 chore: bump deps
* a76b7c2 chore: replace nodeSelector with nodeAffinity in chart + manifests
* 93d8edc chore: bump deps
* 4f7aaeb chore: bump deps
* eef9c9c chore: bump deps
* d54368e feat(chart): add extraVolumes + extraVolumeMounts
* 3a3c070 chore: bump deps
* 5c1a382 fix: namespace for extension-apiserver-authentication rolebinding
* 75ead90 chore: bump deps

<a name="v0.2.0"></a>
## [v0.2.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.1.1...v0.2.0) (2023-09-20)

Welcome to the v0.2.0 release of Kubernetes cloud controller manager for Proxmox!

### Features

- cosign images
- helm oci release

### Changelog

* d2da2e8 chore: release v0.2.0
* 4e641a1 chore: bump deps
* 591b88d chore: bump actions/checkout from 3 to 4
* 45e3aeb chore: bump sigstore/cosign-installer from 3.1.1 to 3.1.2
* 8076eee chore: bump github actions deps
* bc879ab feat: cosign images
* abd63a2 chore: bump deps
* f8d1712 feat: helm oci release
* dfd7c5f chore: bump deps
* 38da18f ci: fix git tag
* d8c6bed chore: bump deps
* 6d79605 chore: release v0.1.1

<a name="v0.1.1"></a>
## [v0.1.1](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.1.0...v0.1.1) (2023-05-08)

Welcome to the v0.1.1 release of Kubernetes cloud controller manager for Proxmox!

### Changelog

* f8c32e1 test: cloud config
* c051d38 ci: build trigger
* a1e7cd0 chore: bump deps
* f813f30 ci: add git version

<a name="v0.1.0"></a>
## [v0.1.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.0.1...v0.1.0) (2023-05-07)

Welcome to the v0.1.0 release of Kubernetes cloud controller manager for Proxmox!

### Changelog

* 3796b9a chore: release v0.1.0
* 2fb410d docs: update readme
* fb96218 test: more tests
* b776e54 test: mock proxmox api
* 641509b doc: helm chart readme
* 90b66dc test: basic test
* bf10985 chore: release v0.0.1

<a name="v0.0.1"></a>
## v0.0.1 (2023-04-29)

Welcome to the v0.0.1 release of Kubernetes cloud controller manager for Proxmox!

### Features

- add controllers

### Changelog

* 0d89bf5 ci: add github checks
* cc2dc17 refactor: proxmox cloud config
* 850dcd4 chore: bump deps
* 0173d67 doc: update readme
* 5677ba3 doc: deploy
* d99a5f0 doc: update
* 8212493 feat: add controllers
