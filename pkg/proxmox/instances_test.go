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
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/cluster"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

type ccmTestSuite struct {
	suite.Suite

	i *instances
}

func (ts *ccmTestSuite) SetupTest() {
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
	if err != nil {
		ts.T().Fatalf("failed to read config: %v", err)
	}

	httpmock.RegisterResponder("GET", "https://127.0.0.1:8006/api2/json/cluster/resources",
		func(req *http.Request) (*http.Response, error) {
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

	httpmock.RegisterResponder("GET", "https://127.0.0.2:8006/api2/json/cluster/resources",
		func(req *http.Request) (*http.Response, error) {
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

	httpmock.RegisterResponder("GET", "https://127.0.0.1:8006/api2/json/nodes/pve-1/qemu/100/status/current",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": map[string]interface{}{
					"status": "running",
				},
			})
		},
	)

	httpmock.RegisterResponder("GET", "https://127.0.0.2:8006/api2/json/nodes/pve-3/qemu/100/status/current",
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"data": map[string]interface{}{
					"status": "stopped",
				},
			})
		},
	)

	cluster, err := cluster.NewCluster(&cfg, &http.Client{})
	if err != nil {
		ts.T().Fatalf("failed to create cluster client: %v", err)
	}

	ts.i = newInstances(cluster)
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
			expectedError: "instances.getInstance() error: proxmox cluster cluster-3 not found",
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
			},
			expectedError: "vm.name(cluster-1-node-1) != node.name(cluster-1-node-3)",
			expected:      false,
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		ts.Run(fmt.Sprint(testCase.msg), func() {
			exists, err := ts.i.InstanceExists(context.Background(), testCase.node)

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
	}

	for _, testCase := range tests {
		testCase := testCase

		ts.Run(fmt.Sprint(testCase.msg), func() {
			exists, err := ts.i.InstanceShutdown(context.Background(), testCase.node)

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
			expectedError: "instances.getInstance() error: proxmox cluster cluster-3 not found",
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
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "proxmox://cluster-1/100",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
					{
						Type:    v1.NodeHostName,
						Address: "cluster-1-node-1",
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
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID: "proxmox://cluster-2/100",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
					{
						Type:    v1.NodeHostName,
						Address: "cluster-2-node-1",
					},
				},
				InstanceType: "1VCPU-2GB",
				Region:       "cluster-2",
				Zone:         "pve-3",
			},
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		ts.Run(fmt.Sprint(testCase.msg), func() {
			meta, err := ts.i.InstanceMetadata(context.Background(), testCase.node)

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

func TestGetProviderID(t *testing.T) {
	t.Parallel()

	i := newInstances(nil)

	tests := []struct {
		msg      string
		region   string
		vmr      *pxapi.VmRef
		expected string
	}{
		{
			msg:      "empty region",
			region:   "",
			vmr:      pxapi.NewVmRef(100),
			expected: "proxmox:///100",
		},
		{
			msg:      "region",
			region:   "cluster1",
			vmr:      pxapi.NewVmRef(100),
			expected: "proxmox://cluster1/100",
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			expected := i.getProviderID(testCase.region, testCase.vmr)
			assert.Equal(t, expected, testCase.expected)
		})
	}
}

func TestParseProviderID(t *testing.T) {
	t.Parallel()

	i := newInstances(nil)

	tests := []struct {
		msg             string
		magic           string
		expectedCluster string
		expectedVmr     *pxapi.VmRef
		expectedError   error
	}{
		{
			msg:           "Empty magic string",
			magic:         "",
			expectedError: fmt.Errorf("foreign providerID or empty \"\""),
		},
		{
			msg:           "Wrong provider",
			magic:         "provider://region/100",
			expectedError: fmt.Errorf("foreign providerID or empty \"provider://region/100\""),
		},
		{
			msg:             "Empty region",
			magic:           "proxmox:///100",
			expectedCluster: "",
			expectedVmr:     pxapi.NewVmRef(100),
		},
		{
			msg:           "Empty region",
			magic:         "proxmox://100",
			expectedError: fmt.Errorf("providerID \"proxmox://100\" didn't match expected format \"proxmox://region/InstanceID\""),
		},
		{
			msg:             "Cluster and InstanceID",
			magic:           "proxmox://cluster/100",
			expectedCluster: "cluster",
			expectedVmr:     pxapi.NewVmRef(100),
		},
		{
			msg:           "Cluster and wrong InstanceID",
			magic:         "proxmox://cluster/name",
			expectedError: fmt.Errorf("providerID \"proxmox://cluster/name\" didn't match expected format \"proxmox://region/InstanceID\""),
		},
	}

	for _, testCase := range tests {
		testCase := testCase

		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			vmr, cluster, err := i.parseProviderID(testCase.magic)

			if testCase.expectedError != nil {
				assert.Equal(t, testCase.expectedError, err)
			} else {
				assert.Equal(t, testCase.expectedVmr, vmr)
				assert.Equal(t, testCase.expectedCluster, cluster)
			}
		})
	}
}
