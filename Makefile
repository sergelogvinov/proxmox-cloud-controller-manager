REGISTRY ?= ghcr.io
USERNAME ?= sergelogvinov
PROJECT ?= proxmox-cloud-controller-manager
IMAGE ?= $(REGISTRY)/$(USERNAME)/$(PROJECT)
PLATFORM ?= linux/arm64,linux/amd64
PUSH ?= false

SHA ?= $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG ?= $(shell git describe --tag --always --match v[0-9]\*)

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

######

# Help Menu

define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- golang 1.19
- golangci-lint

endef

export HELP_MENU_HEADER

help: ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z0-9%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Build Abstractions

.PHONY: build
build: ## Build
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build $(GO_LDFLAGS) \
		-o proxmox-cloud-controller-manager-$(ARCH) ./cmd/proxmox-cloud-controller-manager

.PHONY: run
run: build
	./proxmox-cloud-controller-manager-$(ARCH) --v=4 --kubeconfig=kubeconfig --cloud-config=hack/proxmox-config.yaml --controllers=cloud-node \
		--use-service-account-credentials --leader-elect=false --bind-address=127.0.0.1

.PHONY: lint
lint: ## Lint
	golangci-lint run --config .golangci.yml

.PHONY: unit
unit:
	go test -tags=unit $(shell go list ./...) $(TESTARGS)
