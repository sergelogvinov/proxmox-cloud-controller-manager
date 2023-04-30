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

package cluster

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newClientEnv() (*ClustersConfig, error) {
	cfg, err := ReadCloudConfig(strings.NewReader(`
clusters:
  - url: https://127.0.0.1:8006
    insecure: false
    token_id: "user!token-id"
    token_secret: "secret"
    region: cluster-1
`))

	return &cfg, err
}

func TestNewClient(t *testing.T) {
	cfg, err := newClientEnv()
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	client, err := NewClient(&ClustersConfig{})
	assert.NotNil(t, err)
	assert.Nil(t, client)

	client, err = NewClient(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 1, len(client.proxmox))
}

func TestCheckClusters(t *testing.T) {
	cfg, err := newClientEnv()
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	client, err := NewClient(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, 1, len(client.proxmox))

	pxapi, err := client.GetProxmoxCluster("test")
	assert.NotNil(t, err)
	assert.Nil(t, pxapi)
	assert.Equal(t, "proxmox cluster test not found", err.Error())

	pxapi, err = client.GetProxmoxCluster("cluster-1")
	assert.Nil(t, err)
	assert.NotNil(t, pxapi)

	err = client.CheckClusters()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to initialized proxmox client in region")
}
