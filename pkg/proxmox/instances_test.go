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
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"

	goproxmox "github.com/sergelogvinov/go-proxmox"
	providerconfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"
	testcluster "github.com/sergelogvinov/proxmox-cloud-controller-manager/test/cluster"

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

type configTestCase struct {
	name   string
	config string
}

func getTestConfigs() []configTestCase {
	return []configTestCase{
		{
			name:   "DefaultProvider",
			config: "../../test/config/cluster-config-1.yaml",
		},
		{
			name:   "CapMoxProvider",
			config: "../../test/config/cluster-config-2.yaml",
		},
	}
}

// configuredTestSuite wraps the base suite with a specific configuration
type configuredTestSuite struct {
	*ccmTestSuite

	configCase configTestCase
}

func (ts *configuredTestSuite) SetupTest() {
	testcluster.SetupMockResponders()

	cfg, err := providerconfig.ReadCloudConfigFromFile(ts.configCase.config)
	if err != nil {
		ts.T().Fatalf("failed to read config: %v", err)
	}

	px, err := proxmoxpool.NewProxmoxPool(cfg.Clusters)
	if err != nil {
		ts.T().Fatalf("failed to create cluster client: %v", err)
	}

	client := &client{
		pxpool:  px,
		kclient: fake.NewSimpleClientset(),
	}

	features := providerconfig.ClustersFeatures{
		Provider: cfg.Features.Provider,
		Network:  providerconfig.NetworkOpts{},
	}

	ts.i = newInstances(client, features)
}

func TestSuiteCCM(t *testing.T) {
	configs := getTestConfigs()
	for _, cfg := range configs {
		// Create a new test suite for each configuration
		ts := &ccmTestSuite{}

		// Run the suite with the current configuration
		suite.Run(t, &configuredTestSuite{
			ccmTestSuite: ts,
			configCase:   cfg,
		})
	}
}

// nolint:dupl
func (ts *configuredTestSuite) TestInstanceExists() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset() //nolint: wsl_v5

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
			expectedError: "region not found",
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
					ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abed000000",
						"proxmox://cluster-1/100",
					),
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abed000000",
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
					ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abed000000",
						"proxmox://cluster-1/100",
					),
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abed000000",
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
					ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://8af7110d-0000-0000-0000-9527d10a6583",
						"proxmox://cluster-1/100",
					),
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "8af7110d-0000-0000-0000-9527d10a6583",
					},
				},
			},
			expected: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox, true, false),
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
		{
			msg: "NodeExistsOfflinePVENode",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-4",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
						AnnotationProxmoxInstanceID:                    "104",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abea000002",
						"proxmox://cluster-1/104"),
				},
			},
			expected: true,
		},
		{
			msg: "NodeExistsOfflinePVENodeUninitialized",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-4",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
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
			expected: true,
		},
		{
			msg: "NodeUUIDNotFoundCAPMox",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-rqa-u7y",
					Annotations: map[string]string{
						AnnotationProxmoxInstanceID: "105",
					},
					Labels: map[string]string{
						LabelTopologyRegion: "cluster-1",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://d290d7f2-b179-404c-b627-6e4dccb59066",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "d290d7f2-b179-404c-b627-6e4dccb59066",
					},
				},
			},
			expected: false,
		},
		{
			msg: "NodeUUIDFoundCAPMox",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://11833f4c-341f-4bd3-aad7-f7abed000000",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abed000000",
					},
				},
			},
			expected: true,
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprintf("%s/%s", ts.configCase.name, testCase.msg), func() {
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
func (ts *configuredTestSuite) TestInstanceShutdown() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset() //nolint: wsl_v5

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
			expectedError: goproxmox.ErrVirtualMachineNotFound.Error(),
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
					Name: "cluster-2-node-1",
				},
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-2/103",
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
		{
			msg: "NodeExistsOfflinePVENode",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-4",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abea000002",
						"proxmox://cluster-1/104"),
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsOfflinePVENodeUninitialized",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-4",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
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
			expected: false,
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprintf("%s/%s", ts.configCase.name, testCase.msg), func() {
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

func (ts *configuredTestSuite) TestInstanceMetadata() {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset() //nolint: wsl_v5

	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      *cloudprovider.InstanceMetadata
	}{
		{
			msg: "NodeUndefined",
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
			msg: "NodeForeignProviderIDWithAnnotationAndLabel",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-1",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
						AnnotationProxmoxInstanceID:                    "100",
					},
					Labels: map[string]string{
						LabelTopologyRegion: "cluster-1",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abed000000",
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
			expectedError: "region not found",
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
			expected: &cloudprovider.InstanceMetadata{},
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
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abed000000",
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
				ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
					"proxmox://11833f4c-341f-4bd3-aad7-f7abed000000",
					"proxmox://cluster-1/100",
				),
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
				AdditionalLabels: map[string]string{
					"topology.proxmox.sinextra.dev/region": "cluster-1",
					"topology.proxmox.sinextra.dev/zone":   "pve-1",
				},
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
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abed000000",
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
				ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
					"proxmox://11833f4c-341f-4bd3-aad7-f7abed000000",
					"proxmox://cluster-1/100",
				),
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
				AdditionalLabels: map[string]string{
					"topology.proxmox.sinextra.dev/region": "cluster-1",
					"topology.proxmox.sinextra.dev/zone":   "pve-1",
				},
			},
		},
		{
			msg: "NodeExistsOfflinePVENode",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-4",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
						AnnotationProxmoxInstanceID:                    "104",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abea000002",
						"proxmox://cluster-1/104"),
				},
			},
			expected: &cloudprovider.InstanceMetadata{},
		},
		{
			msg: "NodeExistsOfflinePVENodeUninitialized",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-1-node-4",
					Annotations: map[string]string{
						cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					},
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
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
			expected: &cloudprovider.InstanceMetadata{},
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
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000000",
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
				ProviderID: lo.Ternary(ts.i.provider == providerconfig.ProviderCapmox,
					"proxmox://11833f4c-341f-4bd3-aad7-f7abea000000",
					"proxmox://cluster-2/103",
				),
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
				AdditionalLabels: map[string]string{
					"topology.proxmox.sinextra.dev/region": "cluster-2",
					"topology.proxmox.sinextra.dev/zone":   "pve-3",
				},
			},
		},
	}

	for _, testCase := range tests {
		ts.Run(fmt.Sprintf("%s/%s", ts.configCase.name, testCase.msg), func() {
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
