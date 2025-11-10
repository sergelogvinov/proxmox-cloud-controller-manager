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

package proxmox_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	proxmox "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmox"
)

func TestParseCIDRRuleset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg                string
		cidrs              string
		expectedAllowList  []*net.IPNet
		expectedIgnoreList []*net.IPNet
		expectedError      []any
	}{
		{
			msg:                "Empty CIDR ruleset",
			cidrs:              "",
			expectedAllowList:  []*net.IPNet{},
			expectedIgnoreList: []*net.IPNet{},
			expectedError:      []any{},
		},
		{
			msg:                "Conflicting CIDRs",
			cidrs:              "192.168.0.1/16,!192.168.0.1/24",
			expectedAllowList:  []*net.IPNet{},
			expectedIgnoreList: []*net.IPNet{},
			expectedError:      []any{"192.168.0.0/16", "192.168.0.0/24"},
		},
		{
			msg:                "Ignores invalid CIDRs",
			cidrs:              "722.887.0.1/16,!588.0.1/24",
			expectedAllowList:  []*net.IPNet{},
			expectedIgnoreList: []*net.IPNet{},
			expectedError:      []any{},
		},
		{
			msg:                "Valid CIDRs with ignore",
			cidrs:              "192.168.0.1/16,!10.0.0.5/8,144.0.0.7/16,!13.0.0.9/8",
			expectedAllowList:  []*net.IPNet{mustParseCIDR("192.168.0.0/16"), mustParseCIDR("144.0.0.0/16")},
			expectedIgnoreList: []*net.IPNet{mustParseCIDR("10.0.0.0/8"), mustParseCIDR("13.0.0.0/8")},
			expectedError:      []any{},
		},
	}

	for _, testCase := range tests {
		t.Run(fmt.Sprint(testCase.msg), func(t *testing.T) {
			t.Parallel()

			allowList, ignoreList, err := proxmox.ParseCIDRRuleset(testCase.cidrs)

			assert.Equal(t, len(testCase.expectedAllowList), len(allowList), "Allow list length mismatch")
			assert.Equal(t, len(testCase.expectedIgnoreList), len(ignoreList), "Allow list length mismatch")

			if len(testCase.expectedError) != 0 {
				assert.EqualError(t, err, fmt.Sprintf(proxmox.ErrorCIDRConflict, testCase.expectedError...), "Error mismatch")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func mustParseCIDR(cidr string) *net.IPNet {
	_, parsedCIDR, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse CIDR %s: %v", cidr, err))
	}

	return parsedCIDR
}
