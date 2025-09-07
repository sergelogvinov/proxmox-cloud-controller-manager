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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/suite"

	providerconfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

type ccmTestSuite struct {
	suite.Suite

	i *instances
}

func (ts *ccmTestSuite) SetupTest() {
	cfg, err := providerconfig.ReadCloudConfig(strings.NewReader(`
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
	if err != nil {
		ts.T().Fatalf("failed to read config: %v", err)
	}

	httpmock.RegisterResponderWithQuery("GET", "https://127.0.0.1:8006/api2/json/cluster/resources", "type=vm",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"node":   "pve-1",
						"type":   "qemu",
						"vmid":   100,
						"name":   "cluster-1-node-1",
						"maxcpu": 4,
						"maxmem": 10 * 1024 * 1024 * 1024,
					},
					map[string]interface{}{
						"node":   "pve-2",
						"type":   "qemu",
						"vmid":   101,
						"name":   "cluster-1-node-2",
						"maxcpu": 2,
						"maxmem": 5 * 1024 * 1024 * 1024,
					},
				},
			})
		},
	)

	httpmock.RegisterResponderWithQuery("GET", "https://127.0.0.2:8006/api2/json/cluster/resources", "type=vm",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": []interface{}{
					map[string]interface{}{
						"node":   "pve-3",
						"type":   "qemu",
						"vmid":   100,
						"name":   "cluster-2-node-1",
						"maxcpu": 1,
						"maxmem": 2 * 1024 * 1024 * 1024,
					},
				},
			})
		},
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.1:8006/api2/json/nodes/pve-1/qemu/100/config",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": map[string]interface{}{
					"name":    "cluster-1-node-1",
					"node":    "pve-1",
					"type":    "qemu",
					"vmid":    100,
					"cores":   4,
					"memory":  "10240",
					"smbios1": "uuid=8af7110d-bfad-407a-a663-9527d10a6583",
				},
			})
		},
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.1:8006/api2/json/nodes/pve-2/qemu/101/config",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": map[string]interface{}{
					"name":    "cluster-1-node-2",
					"node":    "pve-2",
					"type":    "qemu",
					"vmid":    101,
					"cores":   2,
					"memory":  "5120",
					"smbios1": "uuid=5d04cb23-ea78-40a3-af2e-dd54798dc887",
				},
			})
		},
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.2:8006/api2/json/nodes/pve-3/qemu/100/config",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": map[string]interface{}{
					"name":    "cluster-2-node-1",
					"node":    "pve-3",
					"type":    "qemu",
					"vmid":    100,
					"cores":   1,
					"memory":  "2048",
					"smbios1": "uuid=3d3db687-89dd-473e-8463-6599f25b36a8,sku=YzEubWVkaXVt",
				},
			})
		},
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.1:8006/api2/json/nodes/pve-1/qemu/100/status/current",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": map[string]interface{}{
					"status": "running",
				},
			})
		},
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.2:8006/api2/json/nodes/pve-3/qemu/100/status/current",
		func(_ *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": map[string]interface{}{
					"status": "stopped",
				},
			})
		},
	)

	px, err := proxmoxpool.NewProxmoxPool(cfg.Clusters, &http.Client{})
	if err != nil {
		ts.T().Fatalf("failed to create cluster client: %v", err)
	}

	client := &client{
		pxpool:  px,
		kclient: fake.NewSimpleClientset(),
	}

	ts.i = newInstances(client, providerconfig.ProviderDefault, providerconfig.NetworkOpts{})
}

func (ts *ccmTestSuite) TearDownTest() {
}

func TestSuiteCCM(t *testing.T) {
	suite.Run(t, new(ccmTestSuite))
}

// nolint:dupl
func (ts *ccmTestSuite) TestInstanceExists() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      bool
	}{
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: true,
		},
		{
			msg: "NodeWrongCluster",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-3-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-3/100",
				},
			},
			expected:      false,
			expectedError: "instances.getInstanceInfo() error: region not found",
		},
		{
			msg: "NodeNotExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-500",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/500",
				},
			},
			expected: false,
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-bfad-407a-a663-9527d10a6583",
					},
				},
			},
			expected: true,
		},
		{
			msg: "NodeExistsWithDifferentName",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-3",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-bfad-407a-a663-9527d10a6583",
					},
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsWithDifferentUUID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-0000-0000-0000-9527d10a6583",
					},
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsWithDifferentNameAndUUID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-3",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-0000-0000-0000-9527d10a6583",
					},
				},
			},
			expected: false,
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprint(testCase.msg), func() {
			exists, err := ts.i.InstanceExists(ts.T().Context(), testCase.node)

			if testCase.expectedError != "" {
				ts.Require().Error(err)
				ts.Require().False(exists)
				ts.Require().Contains(err.Error(), testCase.expectedError)
			} else {
				ts.Require().NoError(err)
				ts.Require().Equal(testCase.expected, exists)
			}
		})
	}
}

// nolint:dupl
func (ts *ccmTestSuite) TestInstanceShutdown() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      bool
	}{
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: false,
		},
		{
			msg: "NodeWrongCluster",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-3-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-3/100",
				},
			},
			expected: false,
		},
		{
			msg: "NodeNotExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-500",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/500",
				},
			},
			expected:      false,
			expectedError: "vm '500' not found",
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-bfad-407a-a663-9527d10a6583",
					},
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsStopped",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-3",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-2/100",
				},
			},
			expected: true,
		},
		{
			msg: "NodeExistsWithDifferentName",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-3",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-bfad-407a-a663-9527d10a6583",
					},
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsWithDifferentUUID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-0000-0000-0000-9527d10a6583",
					},
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsWithDifferentNameAndUUID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-3",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/100",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-0000-0000-0000-9527d10a6583",
					},
				},
			},
			expected: false,
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprint(testCase.msg), func() {
			exists, err := ts.i.InstanceShutdown(ts.T().Context(), testCase.node)

			if testCase.expectedError != "" {
				ts.Require().Error(err)
				ts.Require().False(exists)
				ts.Require().Contains(err.Error(), testCase.expectedError)
			} else {
				ts.Require().NoError(err)
				ts.Require().Equal(testCase.expected, exists)
			}
		})
	}
}

func (ts *ccmTestSuite) TestInstanceMetadata() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      *cloudprovider.InstanceMetadata
	}{
		{
			msg: "NodeAnnotations",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
				},
			},
			expected: &cloudprovider.InstanceMetadata{},
		},
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: &cloudprovider.InstanceMetadata{},
		},
		{
			msg: "NodeWrongCluster",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-3-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-3/100",
				},
			},
			expected:      &cloudprovider.InstanceMetadata{},
			expectedError: "instances.getInstanceInfo() error: region not found",
		},
		{
			msg: "NodeNotExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-500",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/500",
				},
			},
			expected:      &cloudprovider.InstanceMetadata{},
			expectedError: cloudprovider.InstanceNotFound.Error(),
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-bfad-407a-a663-9527d10a6583",
					},
				},
				Spec: v1.NodeSpec{
					Taints: []v1.Taint{
						{
							Key:    cloudproviderapi.TaintExternalCloudProvider,
							Value:  "true",
							Effect: v1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "proxmox://cluster-1/100",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeHostName,
						Address: "cluster-1-node-1",
					},
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
				},
				InstanceType: "4VCPU-10GB",
				Region:       "cluster-1",
				Zone:         "pve-1",
			},
		},
		{
			msg: "NodeExistsDualstack",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4,2001::1",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-bfad-407a-a663-9527d10a6583",
					},
				},
				Spec: v1.NodeSpec{
					Taints: []v1.Taint{
						{
							Key:    cloudproviderapi.TaintExternalCloudProvider,
							Value:  "true",
							Effect: v1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "proxmox://cluster-1/100",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeHostName,
						Address: "cluster-1-node-1",
					},
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
					{
						Type:    v1.NodeInternalIP,
						Address: "2001::1",
					},
				},
				InstanceType: "4VCPU-10GB",
				Region:       "cluster-1",
				Zone:         "pve-1",
			},
		},
		{
			msg: "NodeExistsCluster2",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-2-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "3d3db687-89dd-473e-8463-6599f25b36a8",
					},
				},
				Spec: v1.NodeSpec{
					Taints: []v1.Taint{
						{
							Key:    cloudproviderapi.TaintExternalCloudProvider,
							Value:  "true",
							Effect: v1.TaintEffectNoSchedule,
						},
					},
				},
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "proxmox://cluster-2/100",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeHostName,
						Address: "cluster-2-node-1",
					},
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
				},
				InstanceType: "c1.medium",
				Region:       "cluster-2",
				Zone:         "pve-3",
			},
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprint(testCase.msg), func() {
			meta, err := ts.i.InstanceMetadata(ts.T().Context(), testCase.node)

			if testCase.expectedError != "" {
				ts.Require().Error(err)
				ts.Require().Contains(err.Error(), testCase.expectedError)
			} else {
				ts.Require().NoError(err)
				ts.Require().Equal(testCase.expected, meta)
			}
		})
	}
}
