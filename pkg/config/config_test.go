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

package config_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	providerconfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
)

func TestReadCloudConfig(t *testing.T) {
	cfg, err := providerconfig.ReadCloudConfig(nil)
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	// Empty config
	cfg, err = providerconfig.ReadCloudConfig(strings.NewReader(`
clusters:
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	// Wrong config
	cfg, err = providerconfig.ReadCloudConfig(strings.NewReader(`
clusters:
  test: false
`))

	assert.NotNil(t, err)
	assert.NotNil(t, cfg)

	// Non full config
	cfg, err = providerconfig.ReadCloudConfig(strings.NewReader(`
clusters:
- url: abcd
  region: cluster-1
`))

	assert.NotNil(t, err)
	assert.NotNil(t, cfg)

	// Valid config with one cluster
	cfg, err = providerconfig.ReadCloudConfig(strings.NewReader(`
clusters:
  - url: https://example.com
    insecure: false
    token_id: "user!token-id"
    token_secret: "secret"
    region: cluster-1
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, len(cfg.Clusters))
	assert.Equal(t, "user!token-id", cfg.Clusters[0].TokenID)

	// Valid config with one cluster (username/password), implicit default provider
	cfg, err = providerconfig.ReadCloudConfig(strings.NewReader(`
clusters:
  - url: https://example.com
    insecure: false
    username: "user@pam"
    password: "secret"
    region: cluster-1
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, len(cfg.Clusters))
	assert.Equal(t, providerconfig.ProviderDefault, cfg.Features.Provider)

	// Valid config with one cluster (username/password), explicit provider default
	cfg, err = providerconfig.ReadCloudConfig(strings.NewReader(`
features:
  provider: 'default'
clusters:
  - url: https://example.com
    insecure: false
    username: "user@pam"
    password: "secret"
    region: cluster-1
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, len(cfg.Clusters))
	assert.Equal(t, providerconfig.ProviderDefault, cfg.Features.Provider)

	// Valid config with one cluster (username/password), explicit provider capmox
	cfg, err = providerconfig.ReadCloudConfig(strings.NewReader(`
features:
  provider: 'capmox'
clusters:
  - url: https://example.com
    insecure: false
    username: "user@pam"
    password: "secret"
    region: cluster-1
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, len(cfg.Clusters))
	assert.Equal(t, providerconfig.ProviderCapmox, cfg.Features.Provider)

	// Errors when username/password are set with token_id/token_secret
	_, err = providerconfig.ReadCloudConfig(strings.NewReader(`
features:
  provider: 'capmox'
clusters:
  - url: https://example.com
    insecure: false
    username: "user@pam"
    password: "secret"
    token_id: "ha"
    token_secret: "secret"
    region: cluster-1
`))
	assert.NotNil(t, err)

	// Errors when no region
	_, err = providerconfig.ReadCloudConfig(strings.NewReader(`
features:
  provider: 'capmox'
clusters:
  - url: https://example.com
    insecure: false
    username: "user@pam"
    password: "secret"
`))
	assert.NotNil(t, err)

	// Errors when empty url
	_, err = providerconfig.ReadCloudConfig(strings.NewReader(`
features:
  provider: 'capmox'
clusters:
  - url: ""
    region: test
    insecure: false
    username: "user@pam"
    password: "secret"
`))
	assert.NotNil(t, err)

	// Errors when invalid url protocol
	_, err = providerconfig.ReadCloudConfig(strings.NewReader(`
features:
  provider: 'capmox'
clusters:
  - url: quic://example.com
    insecure: false
    region: test
    username: "user@pam"
    password: "secret"
`))
	assert.NotNil(t, err)
}

func TestNetworkConfig(t *testing.T) {
	// Empty config results in default network mode
	cfg, err := providerconfig.ReadCloudConfig(strings.NewReader(`---`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, providerconfig.NetworkModeDefault, cfg.Features.Network.Mode)

	// Invalid network mode value results in error
	_, err = providerconfig.ReadCloudConfig(strings.NewReader(`
features:
  network:
    mode: 'invalid-mode'
`))
	assert.NotNil(t, err)
}

func TestReadCloudConfigFromFile(t *testing.T) {
	cfg, err := providerconfig.ReadCloudConfigFromFile("testdata/cloud-config.yaml")
	assert.NotNil(t, err)
	assert.EqualError(t, err, "error reading testdata/cloud-config.yaml: open testdata/cloud-config.yaml: no such file or directory")
	assert.NotNil(t, cfg)

	cfg, err = providerconfig.ReadCloudConfigFromFile("../../hack/proxmox-config.yaml")
	assert.Nil(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 2, len(cfg.Clusters))
}
