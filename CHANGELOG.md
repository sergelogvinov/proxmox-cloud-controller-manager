
<a name="v0.11.0"></a>
## [v0.11.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.10.0...v0.11.0) (2025-09-08)

Welcome to the v0.11.0 release of Kubernetes cloud controller manager for Proxmox!

### Features

- use proxmox ha-group as zone name
- add extra labels
- add config options token_id_file & token_secret_file
- add named errors to cloud config

### Changelog

* 27c3e62 feat: use proxmox ha-group as zone name
* 229be14 feat: add extra labels
* b77455a refactor: instance metadata
* 2066aa8 chore: bump deps
* 8ef4bce feat: add config options token_id_file & token_secret_file
* 144b1c7 feat: add named errors to cloud config

<a name="v0.10.0"></a>
## [v0.10.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.9.0...v0.10.0) (2025-08-01)

Welcome to the v0.10.0 release of Kubernetes cloud controller manager for Proxmox!

### Bug Fixes

- makefile conformance stage

### Features

- add new network addressing features

### Changelog

* 1ce4ade chore: release v0.10.0
* e1b8e9b feat: add new network addressing features
* a8183c8 refactor: split cloud config module
* 60f953d chore: bump deps
* 2ebbf7a fix: makefile conformance stage
* 628e7d6 chore: clearer error message

<a name="v0.9.0"></a>
## [v0.9.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.8.0...v0.9.0) (2025-06-05)

Welcome to the v0.9.0 release of Kubernetes cloud controller manager for Proxmox!

### Bug Fixes

- cluster vm list

### Changelog

* 7aba467 chore: release v0.9.0
* e664b24 chore: bump deps
* efb753c fix: cluster vm list
* 5a645a2 chore: bump deps

<a name="v0.8.0"></a>
## [v0.8.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.7.0...v0.8.0) (2025-04-12)

Welcome to the v0.8.0 release of Kubernetes cloud controller manager for Proxmox!

### Bug Fixes

- find node by name

### Features

- custom instance type
- **chart:** extra envs values

### Changelog

* 2e35df2 chore: release v0.8.0
* 646d776 feat(chart): extra envs values
* 19e1f44 chore: bump deps
* 0f0374c feat: custom instance type
* 3a34fb9 fix: find node by name
* 8a2f518 chore: bump deps
* ca452ad chore: bump deps

<a name="v0.7.0"></a>
## [v0.7.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.6.0...v0.7.0) (2025-01-08)

Welcome to the v0.7.0 release of Kubernetes cloud controller manager for Proxmox!

### Features

- enable support for capmox This makes ccm compatible with cluster api and cluster api provider proxmox (capmox)

### Changelog

* bb868bc chore: release v0.7.0
* 956a30a feat: enable support for capmox This makes ccm compatible with cluster api and cluster api provider proxmox (capmox)

<a name="v0.6.0"></a>
## [v0.6.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.5.1...v0.6.0) (2025-01-01)

Welcome to the v0.6.0 release of Kubernetes cloud controller manager for Proxmox!

### Changelog

* 63eef87 chore: release v0.6.0
* 710dc1b chore: bump deps
* 5ea7b73 chore: bump deps
* 2bfb088 chore: bump deps
* 87baa50 docs: add faq
* 7ec2617 docs: install
* 64fc662 docs: kubelet flags

<a name="v0.5.1"></a>
## [v0.5.1](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.5.0...v0.5.1) (2024-09-23)

Welcome to the v0.5.1 release of Kubernetes cloud controller manager for Proxmox!

### Bug Fixes

- instance type

### Changelog

* b3767b5 chore: release v0.5.1
* 10f3e36 fix: instance type
* 2b64352 chore(chart): update readme

<a name="v0.5.0"></a>
## [v0.5.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.4.2...v0.5.0) (2024-09-16)

Welcome to the v0.5.0 release of Kubernetes cloud controller manager for Proxmox!

### Features

- find node by uuid
- prometheus metrics

### Changelog

* 63b6907 chore: release v0.5.0
* 4d79e4e docs: install instruction
* 5876cd4 feat: find node by uuid
* b81ad14 feat: prometheus metrics
* e31b24c refactor: contextual logging
* e1e5263 chore: bump deps

<a name="v0.4.2"></a>
## [v0.4.2](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.4.1...v0.4.2) (2024-05-04)

Welcome to the v0.4.2 release of Kubernetes cloud controller manager for Proxmox!

### Changelog

* 76dae87 chore: release v0.4.2

<a name="v0.4.1"></a>
## [v0.4.1](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.4.0...v0.4.1) (2024-05-04)

Welcome to the v0.4.1 release of Kubernetes cloud controller manager for Proxmox!

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

<a name="v0.3.0"></a>
## [v0.3.0](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.2.0...v0.3.0) (2024-01-03)

Welcome to the v0.3.0 release of Kubernetes cloud controller manager for Proxmox!

### Bug Fixes

- namespace for extension-apiserver-authentication rolebinding

### Features

- can use user/password
- **chart:** add extraVolumes + extraVolumeMounts

### Changelog

* 6f0c667 chore: release v0.3.0
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

<a name="v0.1.1"></a>
## [v0.1.1](https://github.com/sergelogvinov/proxmox-cloud-controller-manager/compare/v0.1.0...v0.1.1) (2023-05-12)

Welcome to the v0.1.1 release of Kubernetes cloud controller manager for Proxmox!

### Changelog

* 6d79605 chore: release v0.1.1
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

<a name="v0.0.1"></a>
## v0.0.1 (2023-04-29)

Welcome to the v0.0.1 release of Kubernetes cloud controller manager for Proxmox!

### Features

- add controllers

### Changelog

* bf10985 chore: release v0.0.1
* 0d89bf5 ci: add github checks
* cc2dc17 refactor: proxmox cloud config
* 850dcd4 chore: bump deps
* 0173d67 doc: update readme
* 5677ba3 doc: deploy
* d99a5f0 doc: update
* 8212493 feat: add controllers
