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

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"
	"github.com/stretchr/testify/assert"
)

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
