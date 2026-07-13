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

package proxmoxpool_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	pxpool "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"
)

func newClusterEnv() []*pxpool.ProxmoxCluster {
	cfg := []*pxpool.ProxmoxCluster{
		{
			URL:         "https://127.0.0.1:8006/api2/json",
			Insecure:    false,
			TokenID:     "user!token-id",
			TokenSecret: "secret",
			Region:      "cluster-1",
		},
		{
			URL:         "https://127.0.0.2:8006/api2/json",
			Insecure:    false,
			TokenID:     "user!token-id",
			TokenSecret: "secret",
			Region:      "cluster-2",
		},
	}

	return cfg
}

func newClusterEnvWithFiles(tokenIDPath, tokenSecretPath string) []*pxpool.ProxmoxCluster {
	cfg := []*pxpool.ProxmoxCluster{
		{
			URL:             "https://127.0.0.1:8006/api2/json",
			Insecure:        false,
			TokenIDFile:     tokenIDPath,
			TokenSecretFile: tokenSecretPath,
			Region:          "cluster-1",
		},
	}

	return cfg
}

func TestNewClient(t *testing.T) {
	cfg := newClusterEnv()
	assert.NotNil(t, cfg)

	pxClient, err := pxpool.NewProxmoxPool([]*pxpool.ProxmoxCluster{})
	assert.NotNil(t, err)
	assert.Nil(t, pxClient)

	pxClient, err = pxpool.NewProxmoxPool(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, pxClient)
}

func TestNewClientWithCredentialsFromFile(t *testing.T) {
	tempDir := t.TempDir()

	tokenIDFile, err := os.CreateTemp(tempDir, "token_id")
	assert.Nil(t, err)

	tokenSecretFile, err := os.CreateTemp(tempDir, "token_secret")
	assert.Nil(t, err)

	_, err = tokenIDFile.WriteString("user!token-id")
	assert.Nil(t, err)
	_, err = tokenSecretFile.WriteString("secret")
	assert.Nil(t, err)

	cfg := newClusterEnvWithFiles(tokenIDFile.Name(), tokenSecretFile.Name())

	pxClient, err := pxpool.NewProxmoxPool(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, pxClient)
	assert.Equal(t, "user!token-id", cfg[0].TokenID)
	assert.Equal(t, "secret", cfg[0].TokenSecret)
}

func TestCheckClusters(t *testing.T) {
	cfg := newClusterEnv()
	assert.NotNil(t, cfg)

	pxClient, err := pxpool.NewProxmoxPool(cfg)
	assert.Nil(t, err)
	assert.NotNil(t, pxClient)

	pxapi, err := pxClient.GetProxmoxCluster("test")
	assert.NotNil(t, err)
	assert.Nil(t, pxapi)
	assert.Equal(t, pxpool.ErrRegionNotFound, err)

	pxapi, err = pxClient.GetProxmoxCluster("cluster-1")
	assert.Nil(t, err)
	assert.NotNil(t, pxapi)

	err = pxClient.CheckClusters(t.Context())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "failed to initialized proxmox client in region")
}

func TestGetNodeHAGroups(t *testing.T) {
	pve8 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/cluster/ha/groups":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data":[
				{"group":"gpu-nodes","type":"group","nodes":"pve1:2,pve2"},
				{"group":"storage-nodes","type":"group","nodes":"pve3"}
			]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer pve8.Close()

	// Proxmox VE 9 migrated HA groups to HA rules
	pve9 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/cluster/ha/groups":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(w, `{"data":null,"message":"cannot index groups: ha groups have been migrated to rules"}`)
		case "/api2/json/cluster/ha/rules":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"data":[
				{"rule":"gpu-nodes","type":"node-affinity","nodes":"pve1:2,pve2","resources":"vm:100","strict":0},
				{"rule":"disabled-rule","type":"node-affinity","nodes":"pve1","disable":1},
				{"rule":"keep-apart","type":"resource-affinity","resources":"vm:100,vm:101"}
			]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer pve9.Close()

	for _, tt := range []struct {
		name string
		url  string
	}{
		{name: "ha-groups", url: pve8.URL},
		{name: "ha-rules", url: pve9.URL},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := []*pxpool.ProxmoxCluster{
				{
					URL:         tt.url + "/api2/json",
					TokenID:     "user!token-id",
					TokenSecret: "secret",
					Region:      "cluster-1",
				},
			}

			pxClient, err := pxpool.NewProxmoxPool(cfg)
			assert.Nil(t, err)
			assert.NotNil(t, pxClient)

			groups, err := pxClient.GetNodeHAGroups(t.Context(), "cluster-1", "pve1")
			assert.Nil(t, err)
			assert.Equal(t, []string{"gpu-nodes"}, groups)

			groups, err = pxClient.GetNodeHAGroups(t.Context(), "cluster-1", "pve5")
			assert.Equal(t, pxpool.ErrHAGroupNotFound, err)
			assert.Nil(t, groups)
		})
	}
}
