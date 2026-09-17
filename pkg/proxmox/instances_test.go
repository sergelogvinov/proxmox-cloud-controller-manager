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
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/sergelogvinov/go-proxmox-rest/fakeapi"
	"github.com/sergelogvinov/go-proxmox-rest/nodes/qemu"
	providerconfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
)

// ternary is a tiny stand-in for samber/lo.Ternary (not a dependency of
// this module) used below to pick expected values that differ between the
// default and capmox providers.
func ternary[T any](cond bool, t, f T) T {
	if cond {
		return t
	}

	return f
}

// newFakeProxmoxClusters builds two independent fakeapi clusters modeling
// the two regions ("cluster-1", "cluster-2") that test/config/cluster-config-*.yaml
// describe, and returns their base URLs so a test can point a ProxmoxPool at
// them in place of the (unreachable) URLs baked into those config files.
//
// cluster-1 has four Proxmox nodes (pve-1..pve-4); pve-4 is down (see
// FailNode below) and its guest (VMID 104, k8s node cluster-1-node-4)
// reports status "unknown", the same signal a real cluster reports for a
// guest whose host is unreachable. cluster-2 has a single node (pve-3)
// hosting a stopped guest (VMID 103, k8s node cluster-2-node-1).
func newFakeProxmoxClusters(t *testing.T) (cluster1URL, cluster2URL string) {
	t.Helper()

	cluster1 := fakeapi.NewCluster(t,
		fakeapi.WithNodes("pve-1", "pve-2", "pve-3", "pve-4"),
		fakeapi.WithHAGroup("ha-group-1", "pve-2:1"),
	)

	cluster1.Node("pve-1").AddVM(100, &qemu.Config{
		Name:    "cluster-1-node-1",
		Cores:   new(4),
		Memory:  &qemu.Memory{Current: new(10240)},
		SMBios1: &qemu.SMBios1{UUID: "11833f4c-341f-4bd3-aad7-f7abed000000"},
	}, fakeapi.WithStatus(qemu.VMStatusRunning))

	cluster1.Node("pve-2").AddVM(101, &qemu.Config{
		Name:    "cluster-1-node-2",
		Cores:   new(2),
		Memory:  &qemu.Memory{Current: new(5120)},
		SMBios1: &qemu.SMBios1{UUID: "11833f4c-341f-4bd3-aad7-f7abed000001"},
	}, fakeapi.WithStatus(qemu.VMStatusRunning))

	// pve-4 is down: its guest is seeded directly with status "unknown"
	// (fakeapi does not itself flip a failed node's guest resources to
	// "unknown" - see Cluster.FailNode's doc comment) and the node is
	// also failed so any direct request to it fails too.
	cluster1.Node("pve-4").AddVM(104, &qemu.Config{
		Name: "cluster-1-node-4",
	}, fakeapi.WithStatus(qemu.VMStatus("unknown")))

	cluster1.FailNode("pve-4", fakeapi.FailureUnreachable)

	cluster2 := fakeapi.NewCluster(t, fakeapi.WithNodes("pve-3"))

	cluster2.Node("pve-3").AddVM(103, &qemu.Config{
		Name:   "cluster-2-node-1",
		Cores:  new(1),
		Memory: &qemu.Memory{Current: new(2048)},
		SMBios1: &qemu.SMBios1{
			UUID: "11833f4c-341f-4bd3-aad7-f7abea000000",
			SKU:  base64.StdEncoding.EncodeToString([]byte("c1.medium")),
		},
	})

	return cluster1.Client(t).ToRESTConfig().BaseURL, cluster2.Client(t).ToRESTConfig().BaseURL
}

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
	cluster1URL, cluster2URL := newFakeProxmoxClusters(ts.T())

	cfg, err := providerconfig.ReadCloudConfigFromFile(ts.configCase.config)
	if err != nil {
		ts.T().Fatalf("failed to read config: %v", err)
	}

	fakeURLs := map[string]string{
		"cluster-1": cluster1URL,
		"cluster-2": cluster2URL,
	}

	for _, c := range cfg.Clusters {
		if u, ok := fakeURLs[c.Region]; ok {
			c.URL = u
		}
	}

	px, err := proxmoxpool.NewProxmoxPool(cfg.Clusters)
	if err != nil {
		ts.T().Fatalf("failed to create cluster client: %v", err)
	}

	client := &client{
		pxpool:  px,
		kclient: fake.NewClientset(),
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
	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      bool
	}{
		{
			msg: "NodeEmptyProviderID",
			node: &v1.Node{
				Name: "test-node-1",
			},
			expected: true,
		},
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				Name: "test-node-1",
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: true,
		},
		{
			msg: "NodeWrongCluster",
			node: &v1.Node{
				Name: "cluster-3-node-1",
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
				Name: "cluster-1-node-500",
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/500",
				},
			},
			expected: false,
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				Name: "cluster-1-node-1",
				Spec: v1.NodeSpec{
					ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
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
				Name: "cluster-1-node-3",
				Spec: v1.NodeSpec{
					ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
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
				Name: "cluster-1-node-1",
				Spec: v1.NodeSpec{
					ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
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
			expected: ternary(ts.i.provider == providerconfig.ProviderCapmox, true, false),
		},
		{
			msg: "NodeExistsWithDifferentNameAndUUID",
			node: &v1.Node{
				Name: "cluster-1-node-3",
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
				Name: "cluster-1-node-4",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					AnnotationProxmoxInstanceID:                    "104",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abea000002",
						"proxmox://cluster-1/104"),
				},
			},
			expected: true,
		},
		{
			msg: "NodeExistsOfflinePVENodeUninitialized",
			node: &v1.Node{
				Name: "cluster-1-node-4",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
				Name: "node-rqa-u7y",
				Annotations: map[string]string{
					AnnotationProxmoxInstanceID: "105",
				},
				Labels: map[string]string{
					LabelTopologyRegion: "cluster-1",
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
				Name: "cluster-1-node-1",
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
	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      bool
	}{
		{
			msg: "NodeEmptyProviderID",
			node: &v1.Node{
				Name: "test-node-1",
			},
			expected: false,
		},
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				Name: "test-node-1",
				Spec: v1.NodeSpec{
					ProviderID: "foreign://provider-id",
				},
			},
			expected: false,
		},
		{
			msg: "NodeWrongCluster",
			node: &v1.Node{
				Name: "cluster-3-node-1",
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-3/100",
				},
			},
			expected: false,
		},
		{
			msg: "NodeNotExists",
			node: &v1.Node{
				Name: "cluster-1-node-500",
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-1/500",
				},
			},
			expected:      false,
			expectedError: proxmoxpool.ErrInstanceNotFound.Error(),
		},
		{
			msg: "NodeExists",
			node: &v1.Node{
				Name: "cluster-1-node-1",
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
				Name: "cluster-2-node-1",
				Spec: v1.NodeSpec{
					ProviderID: "proxmox://cluster-2/103",
				},
			},
			expected: true,
		},
		{
			msg: "NodeExistsWithDifferentName",
			node: &v1.Node{
				Name: "cluster-1-node-3",
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
				Name: "cluster-1-node-1",
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
				Name: "cluster-1-node-3",
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
				Name: "cluster-1-node-4",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abea000002",
						"proxmox://cluster-1/104"),
				},
			},
			expected: false,
		},
		{
			msg: "NodeExistsOfflinePVENodeUninitialized",
			node: &v1.Node{
				Name: "cluster-1-node-4",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
	tests := []struct {
		msg           string
		node          *v1.Node
		expectedError string
		expected      *cloudprovider.InstanceMetadata
	}{
		{
			msg: "NodeUndefined",
			node: &v1.Node{
				Name: "test-node-1",
			},
			expected: &cloudprovider.InstanceMetadata{},
		},
		{
			msg: "NodeForeignProviderID",
			node: &v1.Node{
				Name: "test-node-1",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
				Name: "cluster-1-node-1",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					AnnotationProxmoxInstanceID:                    "100",
				},
				Labels: map[string]string{
					LabelTopologyRegion: "cluster-1",
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
				Name: "cluster-3-node-1",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
				Name: "cluster-1-node-500",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
				Name: "cluster-1-node-1",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
				ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
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
				Name: "cluster-1-node-1",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4,2001::1",
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
				ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
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
			msg: "NodeExistsWithHAGroup",
			node: &v1.Node{
				Name: "cluster-1-node-2",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abed000001",
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
				ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
					"proxmox://11833f4c-341f-4bd3-aad7-f7abed000001",
					"proxmox://cluster-1/101",
				),
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeHostName,
						Address: "cluster-1-node-2",
					},
					{
						Type:    v1.NodeInternalIP,
						Address: "1.2.3.4",
					},
				},
				InstanceType: "2VCPU-5GB",
				Region:       "cluster-1",
				Zone:         "pve-2",
				AdditionalLabels: map[string]string{
					"topology.proxmox.sinextra.dev/region":           "cluster-1",
					"topology.proxmox.sinextra.dev/zone":             "pve-2",
					"group.topology.proxmox.sinextra.dev/ha-group-1": "",
				},
			},
		},
		{
			msg: "NodeExistsOfflinePVENode",
			node: &v1.Node{
				Name: "cluster-1-node-4",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
					AnnotationProxmoxInstanceID:                    "104",
				},
				Status: v1.NodeStatus{
					NodeInfo: v1.NodeSystemInfo{
						SystemUUID: "11833f4c-341f-4bd3-aad7-f7abea000002",
					},
				},
				Spec: v1.NodeSpec{
					ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
						"proxmox://11833f4c-341f-4bd3-aad7-f7abea000002",
						"proxmox://cluster-1/104"),
				},
			},
			expected: &cloudprovider.InstanceMetadata{},
		},
		{
			msg: "NodeExistsOfflinePVENodeUninitialized",
			node: &v1.Node{
				Name: "cluster-1-node-4",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
				Name: "cluster-2-node-1",
				Annotations: map[string]string{
					cloudproviderapi.AnnotationAlphaProvidedIPAddr: "1.2.3.4",
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
				ProviderID: ternary(ts.i.provider == providerconfig.ProviderCapmox,
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
