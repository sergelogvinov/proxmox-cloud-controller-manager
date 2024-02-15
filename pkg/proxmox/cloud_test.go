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

package proxmox

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/cluster"
	provider "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/provider"
)

func TestNewCloudError(t *testing.T) {
	cloud, err := newCloud(&cluster.ClustersConfig{})
	assert.NotNil(t, err)
	assert.Nil(t, cloud)
	assert.EqualError(t, err, "no Proxmox clusters found")
}

func TestCloud(t *testing.T) {
	cfg, err := cluster.ReadCloudConfig(strings.NewReader(`
clusters:
  - url: https://example.com
    insecure: false
    token_id: "user!token-id"
    token_secret: "secret"
    region: cluster-1
`))
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	cloud, err := newCloud(&cfg)
	assert.Nil(t, err)
	assert.NotNil(t, cloud)

	lb, res := cloud.LoadBalancer()
	assert.Nil(t, lb)
	assert.Equal(t, res, false)

	ins, res := cloud.Instances()
	assert.Nil(t, ins)
	assert.Equal(t, res, false)

	ins2, res := cloud.InstancesV2()
	assert.NotNil(t, ins2)
	assert.Equal(t, res, true)

	zone, res := cloud.Zones()
	assert.Nil(t, zone)
	assert.Equal(t, res, false)

	cl, res := cloud.Clusters()
	assert.Nil(t, cl)
	assert.Equal(t, res, false)

	route, res := cloud.Routes()
	assert.Nil(t, route)
	assert.Equal(t, res, false)

	pName := cloud.ProviderName()
	assert.Equal(t, pName, provider.ProviderName)

	clID := cloud.HasClusterID()
	assert.Equal(t, clID, true)
}
