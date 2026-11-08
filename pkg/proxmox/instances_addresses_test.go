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
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
)

func TestRemoveFromNodeAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg       string
		addresses []v1.NodeAddress
		remove    []v1.NodeAddress
		expected  []v1.NodeAddress
	}{
		{
			msg: "remove one address",
			addresses: []v1.NodeAddress{
				{Type: v1.NodeHostName, Address: "node"},
				{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
				{Type: v1.NodeInternalIP, Address: "192.168.0.1"},
			},
			remove: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
			},
			expected: []v1.NodeAddress{
				{Type: v1.NodeHostName, Address: "node"},
				{Type: v1.NodeInternalIP, Address: "192.168.0.1"},
			},
		},
		{
			msg: "remove addresses that are not next to each other",
			addresses: []v1.NodeAddress{
				{Type: v1.NodeHostName, Address: "node"},
				{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
				{Type: v1.NodeInternalIP, Address: "192.168.0.1"},
				{Type: v1.NodeInternalIP, Address: "10.0.0.2"},
				{Type: v1.NodeInternalIP, Address: "192.168.0.2"},
			},
			remove: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
				{Type: v1.NodeInternalIP, Address: "10.0.0.2"},
			},
			expected: []v1.NodeAddress{
				{Type: v1.NodeHostName, Address: "node"},
				{Type: v1.NodeInternalIP, Address: "192.168.0.1"},
				{Type: v1.NodeInternalIP, Address: "192.168.0.2"},
			},
		},
		{
			msg: "remove an address listed with two types",
			addresses: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
				{Type: v1.NodeInternalIP, Address: "192.168.0.1"},
				{Type: v1.NodeExternalIP, Address: "10.0.0.1"},
			},
			remove: []v1.NodeAddress{
				{Address: "10.0.0.1"},
			},
			expected: []v1.NodeAddress{
				{Type: v1.NodeInternalIP, Address: "192.168.0.1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			t.Parallel()

			addresses := tt.addresses
			removeFromNodeAddresses(&addresses, tt.remove...)

			assert.Equal(t, tt.expected, addresses)
		})
	}
}

func TestProcessIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		msg      string
		address  string
		expected []v1.NodeAddress
	}{
		{
			msg:      "private ipv4",
			address:  "10.0.0.1",
			expected: []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: "10.0.0.1"}},
		},
		{
			msg:      "global ipv6",
			address:  "2001:db8::1",
			expected: []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: "2001:db8::1"}},
		},
		{
			msg:     "loopback ipv4",
			address: "127.0.0.1",
		},
		{
			msg:     "link-local ipv4 from nodelocaldns",
			address: "169.254.20.10",
		},
		{
			msg:     "link-local ipv6",
			address: "fe80::1",
		},
		{
			msg:     "unique local ipv6",
			address: "fd00::1",
		},
		{
			msg:     "invalid address",
			address: "not-an-ip",
		},
	}

	i := &instances{}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			t.Parallel()

			var addresses []v1.NodeAddress

			i.processIP(t.Context(), &addresses, tt.address)

			assert.Equal(t, tt.expected, addresses)
		})
	}
}
