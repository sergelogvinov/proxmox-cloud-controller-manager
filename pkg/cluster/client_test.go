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

package cluster_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"

	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/cluster"
)

func newClusterEnv() (*cluster.ClustersConfig, error) {
	cfg, err := cluster.ReadCloudConfig(strings.NewReader(`
clusters:
  - url: https://127.0.0.1:8006/api2/json
    insecure: false
    token_id: "user!token-id"
    token_secret: "secret"
    region: cluster-1
  - url: https://127.0.0.2:8006/api2/json
    insecure: false
    token_id: "user!token-id"
    token_secret: "secret"
    region: cluster-2
`))

	return &cfg, err
}

func TestNewClient(t *testing.T) {
	cfg, err := newClusterEnv()
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	client, err := cluster.NewCluster(&cluster.ClustersConfig{}, nil)
	assert.NotNil(t, err)
	assert.Nil(t, client)

	client, err = cluster.NewCluster(cfg, nil)
	assert.Nil(t, err)
	assert.NotNil(t, client)
}

func TestCheckClusters(t *testing.T) {
	cfg, err := newClusterEnv()
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	client, err := cluster.NewCluster(cfg, nil)
	assert.Nil(t, err)
	assert.NotNil(t, client)

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

func TestFindVMByNameNonExist(t *testing.T) {
	cfg, err := newClusterEnv()
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://127.0.0.1:8006/api2/json/cluster/resources",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"node": "node-1",
						"type": "qemu",
						"vmid": 100,
						"name": "test1-vm",
					},
				},
			})
		},
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.2:8006/api2/json/cluster/resources",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"node": "node-2",
						"type": "qemu",
						"vmid": 100,
						"name": "test2-vm",
					},
				},
			})
		},
	)

	client, err := cluster.NewCluster(cfg, &http.Client{})
	assert.Nil(t, err)
	assert.NotNil(t, client)

	vmr, cluster, err := client.FindVMByName("non-existing-vm")
	assert.NotNil(t, err)
	assert.Equal(t, "", cluster)
	assert.Nil(t, vmr)
	assert.Contains(t, err.Error(), "vm 'non-existing-vm' not found")
}

func TestFindVMByNameExist(t *testing.T) {
	cfg, err := newClusterEnv()
	assert.Nil(t, err)
	assert.NotNil(t, cfg)

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "https://127.0.0.1:8006/api2/json/cluster/resources",
		httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"node": "node-1",
					"type": "qemu",
					"vmid": 100,
					"name": "test1-vm",
				},
			},
		}),
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.2:8006/api2/json/cluster/resources",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"node": "node-2",
						"type": "qemu",
						"vmid": 100,
						"name": "test2-vm",
					},
				},
			})
		},
	)

	client, err := cluster.NewCluster(cfg, &http.Client{})
	assert.Nil(t, err)
	assert.NotNil(t, client)

	tests := []struct {
		msg             string
		vmName          string
		expectedError   error
		expectedVMID    int
		expectedCluster string
	}{
		{
			msg:           "vm not found",
			vmName:        "non-existing-vm",
			expectedError: fmt.Errorf("vm 'non-existing-vm' not found"),
		},
		{
			msg:             "Test1-VM",
			vmName:          "test1-vm",
			expectedVMID:    100,
			expectedCluster: "cluster-1",
		},
		{
			msg:             "Test2-VM",
			vmName:          "test2-vm",
			expectedVMID:    100,
			expectedCluster: "cluster-2",
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			vmr, cluster, err := client.FindVMByName(testCase.vmName)

			if testCase.expectedError == nil {
				assert.Nil(t, err)
				assert.NotNil(t, vmr)
				assert.Equal(t, testCase.expectedVMID, vmr.VmId())
				assert.Equal(t, testCase.expectedCluster, cluster)
			} else {
				assert.NotNil(t, err)
				assert.Equal(t, "", cluster)
				assert.Nil(t, vmr)
				assert.Contains(t, err.Error(), "vm 'non-existing-vm' not found")
			}
		})
	}
}
