/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package config is the configuration for the cloud provider.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"
)

// Provider specifies the provider. Can be 'default' or 'capmox'
type Provider string

// ProviderDefault is the default provider
const ProviderDefault Provider = "default"

// ProviderCapmox is the Provider for capmox
const ProviderCapmox Provider = "capmox"

// NetworkMode specifies the network mode.
type NetworkMode string

const (
	// NetworkModeDefault 'default' mode uses ips provided to the kubelet via --node-ip flags
	NetworkModeDefault NetworkMode = "default"
	// NetworkModeOnlyQemu 'qemu' mode tries to determine the ip addresses via the QEMU agent.
	NetworkModeOnlyQemu NetworkMode = "qemu"
	// NetworkModeAuto 'auto' attempts to use a combination of the above modes
	NetworkModeAuto NetworkMode = "auto"
)

// ValidNetworkModes is a list of valid network modes.
var ValidNetworkModes = []NetworkMode{NetworkModeDefault, NetworkModeOnlyQemu, NetworkModeAuto}

// NetworkOpts specifies the network options for the cloud provider.
type NetworkOpts struct {
	ExternalIPCIDRS     string      `yaml:"external_ip_cidrs,omitempty"`
	IPv6SupportDisabled bool        `yaml:"ipv6_support_disabled,omitempty"`
	IPSortOrder         string      `yaml:"ip_sort_order,omitempty"`
	Mode                NetworkMode `yaml:"mode,omitempty"`
}

// ClustersConfig is proxmox multi-cluster cloud config.
type ClustersConfig struct {
	Features struct {
		Provider Provider    `yaml:"provider,omitempty"`
		Network  NetworkOpts `yaml:"network,omitempty"`
	} `yaml:"features,omitempty"`
	Clusters []*proxmoxpool.ProxmoxCluster `yaml:"clusters,omitempty"`
}

// Errors for Reading Cloud Config
var (
	ErrMissingPVERegion       = errors.New("missing PVE region in cloud config")
	ErrMissingPVEAPIURL       = errors.New("missing PVE API URL in cloud config")
	ErrAuthCredentialsMissing = errors.New("user, token or file credentials are required")
	ErrInvalidAuthCredentials = errors.New("must specify one of user, token or file credentials, not multiple")
	ErrInvalidCloudConfig     = errors.New("invalid cloud config")
	ErrInvalidNetworkMode     = fmt.Errorf("invalid network mode, valid modes are %v", ValidNetworkModes)
)

// ReadCloudConfig reads cloud config from a reader.
func ReadCloudConfig(config io.Reader) (ClustersConfig, error) {
	cfg := ClustersConfig{}

	if config != nil {
		if err := yaml.NewDecoder(config).Decode(&cfg); err != nil {
			return ClustersConfig{}, errors.Join(ErrInvalidCloudConfig, err)
		}
	}

	for idx, c := range cfg.Clusters {
		hasTokenAuth := c.TokenID != "" || c.TokenSecret != ""
		hasTokenFileAuth := c.TokenIDFile != "" || c.TokenSecretFile != ""

		hasUserAuth := c.Username != "" && c.Password != ""
		if (hasTokenAuth && hasUserAuth) || (hasTokenFileAuth && hasUserAuth) || (hasTokenAuth && hasTokenFileAuth) {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: %w", idx+1, ErrInvalidAuthCredentials)
		}

		if !hasTokenAuth && !hasTokenFileAuth && !hasUserAuth {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: %w", idx+1, ErrAuthCredentialsMissing)
		}

		if c.Region == "" {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: %w", idx+1, ErrMissingPVERegion)
		}

		if c.URL == "" || !strings.HasPrefix(c.URL, "http") {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: %w", idx+1, ErrMissingPVEAPIURL)
		}
	}

	if cfg.Features.Provider == "" {
		cfg.Features.Provider = ProviderDefault
	}

	if cfg.Features.Network.Mode == "" {
		cfg.Features.Network.Mode = NetworkModeDefault
	}

	// Validate network mode is valid
	if !slices.Contains(ValidNetworkModes, cfg.Features.Network.Mode) {
		return ClustersConfig{}, ErrInvalidNetworkMode
	}

	return cfg, nil
}

// ReadCloudConfigFromFile reads cloud config from a file.
func ReadCloudConfigFromFile(file string) (ClustersConfig, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return ClustersConfig{}, fmt.Errorf("error reading %s: %v", file, err)
	}
	defer f.Close() // nolint: errcheck

	return ReadCloudConfig(f)
}
