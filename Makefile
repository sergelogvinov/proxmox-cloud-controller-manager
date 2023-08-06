REGISTRY ?= ghcr.io
USERNAME ?= sergelogvinov
PROJECT ?= proxmox-cloud-controller-manager
IMAGE ?= $(REGISTRY)/$(USERNAME)/$(PROJECT)
HELMREPO ?= $(REGISTRY)/$(USERNAME)/charts
PLATFORM ?= linux/arm64,linux/amd64
PUSH ?= false

VERSION ?= $(shell git describe --dirty --tag --match='v*')
SHA ?= $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG ?= $(VERSION)

GO_LDFLAGS := -s -w
GO_LDFLAGS += -X k8s.io/component-base/version.gitVersion=$(VERSION)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
ARCHS = amd64 arm64

TESTARGS ?= "-v"

BUILD_ARGS := --platform=$(PLATFORM)
ifeq ($(PUSH),true)
BUILD_ARGS += --push=$(PUSH)
else
BUILD_ARGS += --output type=docker
endif

COSING_ARGS ?=

############

# Help Menu

define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- golang 1.20+
- golangci-lint

endef

export HELP_MENU_HEADER

help: ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z0-9%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

############
#
# Build Abstractions
#

build-all-archs:
	@for arch in $(ARCHS); do $(MAKE) ARCH=$${arch} build ; done

.PHONY: clean
clean: ## Clean
	rm -rf bin

.PHONY: build
build: ## Build
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(GO_LDFLAGS)" \
		-o bin/proxmox-cloud-controller-manager-$(ARCH) ./cmd/proxmox-cloud-controller-manager

.PHONY: run
run: build ## Run
	./bin/proxmox-cloud-controller-manager-$(ARCH) --v=5 --kubeconfig=kubeconfig --cloud-config=proxmox-config.yaml --controllers=cloud-node,cloud-node-lifecycle \
		--use-service-account-credentials --leader-elect=false --bind-address=127.0.0.1

.PHONY: lint
lint: ## Lint Code
	golangci-lint run --config .golangci.yml

.PHONY: unit
unit: ## Unit Tests
	go test -tags=unit $(shell go list ./...) $(TESTARGS)

############

.PHONY: helm-unit
helm-unit: ## Helm Unit Tests
	@helm lint charts/proxmox-cloud-controller-manager
	@helm template -f charts/proxmox-cloud-controller-manager/ci/values.yaml \
		proxmox-cloud-controller-manager charts/proxmox-cloud-controller-manager >/dev/null

.PHONY: helm-login
helm-login: ## Helm Login
	@echo "${HELM_TOKEN}" | helm registry login $(REGISTRY) --username $(USERNAME) --password-stdin

.PHONY: helm-release
helm-release: ## Helm Release
	@rm -rf dist/
	@helm package charts/proxmox-cloud-controller-manager -d dist
	@helm push dist/proxmox-cloud-controller-manager-*.tgz oci://$(HELMREPO) 2>&1 | tee dist/.digest
	@cosign sign --yes $(COSING_ARGS) $(HELMREPO)/proxmox-cloud-controller-manager@$$(cat dist/.digest | awk -F "[, ]+" '/Digest/{print $$NF}')

############

.PHONY: docs
docs:
	helm template -n kube-system proxmox-cloud-controller-manager \
		-f charts/proxmox-cloud-controller-manager/values.edge.yaml \
		--set-string image.tag=$(TAG) \
		charts/proxmox-cloud-controller-manager > docs/deploy/cloud-controller-manager.yml
	helm template -n kube-system proxmox-cloud-controller-manager \
		-f charts/proxmox-cloud-controller-manager/values.talos.yaml \
		--set-string image.tag=$(TAG) \
		charts/proxmox-cloud-controller-manager > docs/deploy/cloud-controller-manager-talos.yml
	helm-docs --sort-values-order=file charts/proxmox-cloud-controller-manager

release-update:
	git-chglog --config hack/chglog-config.yml -o CHANGELOG.md

############
#
# Docker Abstractions
#

docker-init:
	docker run --rm --privileged multiarch/qemu-user-static:register --reset

	docker context create multiarch ||:
	docker buildx create --name multiarch --driver docker-container --use ||:
	docker context use multiarch
	docker buildx inspect --bootstrap multiarch

.PHONY: images
images: ## Build images
	docker buildx build $(BUILD_ARGS) \
		--build-arg VERSION="$(VERSION)" \
		--build-arg TAG="$(TAG)" \
		--build-arg SHA="$(SHA)" \
		-t $(IMAGE):$(TAG) \
		-f Dockerfile .

.PHONY: images-cosign
images-cosign:
	@cosign sign --yes $(COSING_ARGS) --recursive $(IMAGE):$(TAG)
