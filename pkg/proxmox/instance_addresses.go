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
	"bytes"
	"context"
	"fmt"
	"net"
	"slices"
	"sort"
	"strings"

	"github.com/luthermonson/go-proxmox"

	providerconfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
	metrics "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/metrics"

	v1 "k8s.io/api/core/v1"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"k8s.io/klog/v2"
)

const (
	noSortPriority = 0
)

func (i *instances) addresses(ctx context.Context, node *v1.Node, info *instanceInfo) []v1.NodeAddress {
	var (
		providedIP string
		ok         bool
	)

	if providedIP, ok = node.ObjectMeta.Annotations[cloudproviderapi.AnnotationAlphaProvidedIPAddr]; !ok {
		klog.ErrorS(ErrKubeletExternalProvider, fmt.Sprintf(
			"instances.InstanceMetadata() called: annotation %s missing from node. Was kubelet started without --cloud-provider=external or --node-ip?",
			cloudproviderapi.AnnotationAlphaProvidedIPAddr),
			"node", klog.KRef("", node.Name))
	}

	// providedIP is supposed to be a single IP but some kubelets might set a comma separated list of IPs.
	providedAddresses := []string{}
	if providedIP != "" {
		providedAddresses = strings.Split(providedIP, ",")
	}

	addresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: node.Name},
	}

	for _, address := range providedAddresses {
		if address = strings.TrimSpace(address); address != "" {
			parsedAddress := net.ParseIP(address)
			if parsedAddress != nil {
				addresses = append(addresses, v1.NodeAddress{
					Type:    v1.NodeInternalIP,
					Address: parsedAddress.String(),
				})
			} else {
				klog.Warningf("Ignoring invalid provided address '%s' for node %s", address, node.Name)
			}
		}
	}

	if i.networkOpts.Mode == providerconfig.NetworkModeDefault {
		klog.V(4).InfoS("instances.addresses() returning provided IPs", "node", klog.KObj(node))

		return addresses
	}

	if i.networkOpts.Mode == providerconfig.NetworkModeOnlyQemu || i.networkOpts.Mode == providerconfig.NetworkModeAuto {
		newAddresses, err := i.retrieveQemuAddresses(ctx, info)
		if err != nil {
			klog.ErrorS(err, "Failed to retrieve host addresses")
		}

		addToNodeAddresses(&addresses, newAddresses...)
	}

	// Remove addresses that match the ignored CIDRs
	if len(i.networkOpts.IgnoredCIDRs) > 0 {
		var removableAddresses []v1.NodeAddress

		for _, addr := range addresses {
			ip := net.ParseIP(addr.Address)
			if ip != nil && isAddressInCIDRList(i.networkOpts.IgnoredCIDRs, ip) {
				removableAddresses = append(removableAddresses, addr)
			}
		}

		removeFromNodeAddresses(&addresses, removableAddresses...)
	}

	sortNodeAddresses(addresses, i.networkOpts.SortOrder)

	klog.V(4).InfoS("instances.addresses() returning addresses", "addresses", addresses, "node", klog.KObj(node))

	return addresses
}

// retrieveQemuAddresses retrieves the addresses from the QEMU agent
func (i *instances) retrieveQemuAddresses(ctx context.Context, info *instanceInfo) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress

	nics, err := i.getInstanceNics(ctx, info)
	if err != nil {
		return nil, err
	}

	for _, nic := range nics {
		if slices.Contains([]string{"lo", "cilium_net", "cilium_host"}, nic.Name) ||
			strings.HasPrefix(nic.Name, "dummy") {
			continue
		}

		for _, ip := range nic.IPAddresses {
			i.processIP(ctx, &addresses, ip.IPAddress)
		}
	}

	return addresses, nil
}

func (i *instances) processIP(_ context.Context, addresses *[]v1.NodeAddress, addr string) {
	ip := net.ParseIP(addr)
	if ip == nil || ip.IsLoopback() {
		return
	}

	if ip.To4() == nil {
		if i.networkOpts.IPv6SupportDisabled {
			klog.V(4).InfoS("Skipping IPv6 address due to IPv6 support being disabled", "address", ip.String())

			return
		}

		if ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return
		}
	}

	addressType := v1.NodeInternalIP
	if len(i.networkOpts.ExternalCIDRs) != 0 && isAddressInCIDRList(i.networkOpts.ExternalCIDRs, ip) {
		addressType = v1.NodeExternalIP
	}

	*addresses = append(*addresses, v1.NodeAddress{
		Type:    addressType,
		Address: ip.String(),
	})
}

func (i *instances) getInstanceNics(ctx context.Context, info *instanceInfo) ([]*proxmox.AgentNetworkIface, error) {
	result := make([]*proxmox.AgentNetworkIface, 0)

	px, err := i.c.pxpool.GetProxmoxCluster(info.Region)
	if err != nil {
		return result, err
	}

	vm, err := px.GetVMConfig(ctx, info.ID)
	if err != nil {
		return nil, err
	}

	mc := metrics.NewMetricContext("getVmInfo")

	nicset, err := vm.AgentGetNetworkIFaces(ctx)
	if mc.ObserveRequest(err) != nil {
		return result, err
	}

	klog.V(4).InfoS("getInstanceNics() retrieved IP set", "nicset", nicset)

	return nicset, nil
}

// getSortPriority returns the priority as int of an address.
//
// The priority depends on the index of the CIDR in the list the address is matching,
// where the first item of the list has higher priority than the last.
//
// If the address does not match any CIDR or is not an IP address the function returns noSortPriority.
func getSortPriority(list []*net.IPNet, address string) int {
	parsedAddress := net.ParseIP(address)
	if parsedAddress == nil {
		return noSortPriority
	}

	for i, cidr := range list {
		if cidr.Contains(parsedAddress) {
			return len(list) - i
		}
	}

	return noSortPriority
}

// sortNodeAddresses sorts node addresses based on comma separated list of CIDRs represented by addressSortOrder.
//
// The function only sorts addresses which match the CIDR and leaves the other addresses in the same order they are in.
// Essentially, it will also group the addresses matching a CIDR together and sort them ascending in this group,
// whereas the inter-group sorting depends on the priority.
//
// The priority depends on the order of the item in addressSortOrder, where the first item has higher priority than the last.
func sortNodeAddresses(addresses []v1.NodeAddress, addressSortOrder []*net.IPNet) {
	sort.SliceStable(addresses, func(i int, j int) bool {
		addressLeft := addresses[i]
		addressRight := addresses[j]

		priorityLeft := getSortPriority(addressSortOrder, addressLeft.Address)
		priorityRight := getSortPriority(addressSortOrder, addressRight.Address)

		// ignore priorities of value 0 since this means the address has noSortPriority and we need to sort by priority
		if priorityLeft > noSortPriority && priorityLeft == priorityRight {
			return bytes.Compare(net.ParseIP(addressLeft.Address), net.ParseIP(addressRight.Address)) < 0
		}

		return priorityLeft > priorityRight
	})
}

// addToNodeAddresses appends the NodeAddresses to the passed-by-pointer slice,
// only if they do not already exist
func addToNodeAddresses(addresses *[]v1.NodeAddress, addAddresses ...v1.NodeAddress) {
	for _, add := range addAddresses {
		exists := false

		for _, existing := range *addresses {
			if existing.Address == add.Address && existing.Type == add.Type {
				exists = true

				break
			}
		}

		if !exists {
			*addresses = append(*addresses, add)
		}
	}
}

// removeFromNodeAddresses removes the NodeAddresses from the passed-by-pointer
// slice if they already exist.
func removeFromNodeAddresses(addresses *[]v1.NodeAddress, removeAddresses ...v1.NodeAddress) {
	var indexesToRemove []int

	for _, remove := range removeAddresses {
		for i := len(*addresses) - 1; i >= 0; i-- {
			existing := (*addresses)[i]
			if existing.Address == remove.Address && (existing.Type == remove.Type || remove.Type == "") {
				indexesToRemove = append(indexesToRemove, i)
			}
		}
	}

	for _, i := range indexesToRemove {
		if i < len(*addresses) {
			*addresses = append((*addresses)[:i], (*addresses)[i+1:]...)
		}
	}
}

// isAddressInCIDRList checks if the given address is contained in any of the CIDRs in the list.
func isAddressInCIDRList(cidrs []*net.IPNet, address net.IP) bool {
	for _, cidr := range cidrs {
		if cidr.Contains(address) {
			return true
		}
	}

	return false
}
