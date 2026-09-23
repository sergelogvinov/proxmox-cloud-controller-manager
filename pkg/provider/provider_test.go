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

package provider_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	provider "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/provider"
)

func TestGetProviderIDFromID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg                string
		region             string
		vmID               int
		expectedProviderID string
	}{
		{
			msg:                "Valid providerID",
			region:             "region",
			vmID:               123,
			expectedProviderID: "proxmox://region/123",
		},
		{
			msg:                "No region",
			region:             "",
			vmID:               123,
			expectedProviderID: "proxmox:///123",
		},
	}

	for _, testCase := range tests {
		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			providerID := provider.GetProviderIDFromID(testCase.region, testCase.vmID)

			assert.Equal(t, testCase.expectedProviderID, providerID)
		})
	}
}

func TestGetProviderIDFromUUID(t *testing.T) {
	t.Parallel()

	uuid := "4c4c4544-0044-4210-804a-c7c04f503432"

	assert.Equal(t, "proxmox://"+uuid, provider.GetProviderIDFromUUID(uuid))
}

func TestParseProviderID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg            string
		providerID     string
		expectedError  error
		expectedResult *provider.ID
	}{
		{
			msg:        "Valid VMID",
			providerID: "proxmox://region/123",
			expectedResult: &provider.ID{
				Type:   provider.ProviderIDTypeDefault,
				VMID:   123,
				Region: "region",
			},
		},
		{
			msg:        "Valid VMID with empty region",
			providerID: "proxmox:///123",
			expectedResult: &provider.ID{
				Type:   provider.ProviderIDTypeDefault,
				VMID:   123,
				Region: "",
			},
		},
		{
			msg:        "Valid UUID",
			providerID: "proxmox://4c4c4544-0044-4210-804a-c7c04f503432",
			expectedResult: &provider.ID{
				Type: provider.ProviderIDTypeCapmox,
				UUID: "4c4c4544-0044-4210-804a-c7c04f503432",
			},
		},
		{
			msg:           "Invalid providerID format",
			providerID:    "proxmox://123",
			expectedError: fmt.Errorf("providerID \"proxmox://123\" didn't match expected format \"proxmox://region/InstanceID\" or \"proxmox://UUID\""),
		},
		{
			msg:           "Non proxmox providerID",
			providerID:    "cloud:///123",
			expectedError: fmt.Errorf("foreign providerID or empty \"cloud:///123\""),
		},
		{
			msg:           "Non proxmox providerID",
			providerID:    "cloud://123",
			expectedError: fmt.Errorf("foreign providerID or empty \"cloud://123\""),
		},
		{
			msg:           "InValid VMID",
			providerID:    "proxmox://region/abc",
			expectedError: fmt.Errorf("InstanceID have to be a number, but got \"abc\""),
		},
	}

	for _, testCase := range tests {
		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			pid, err := provider.ParseProviderID(testCase.providerID)

			if testCase.expectedError != nil {
				assert.NotNil(t, err)
				assert.EqualError(t, err, testCase.expectedError.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedResult, pid)
			}
		})
	}
}
