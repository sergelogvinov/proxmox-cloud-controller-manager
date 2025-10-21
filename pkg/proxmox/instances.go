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
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	providerconfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
	metrics "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/metrics"
	provider "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/provider"
	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

type instanceNetops struct {
	ExternalCIDRs       []*net.IPNet
	SortOrder           []*net.IPNet
	IgnoredCIDRs        []*net.IPNet
	Mode                providerconfig.NetworkMode
	IPv6SupportDisabled bool
}

type instanceInfo struct {
	ID     int
	UUID   string
	Name   string
	Type   string
	Node   string
	Region string
	Zone   string
}

type instances struct {
	c             *client
	zoneAsHAGroup bool
	provider      providerconfig.Provider
	networkOpts   instanceNetops
}

var instanceTypeNameRegexp = regexp.MustCompile(`(^[a-zA-Z0-9_.-]+)$`)

func newInstances(client *client, features providerconfig.ClustersFeatures) *instances {
	externalIPCIDRs := ParseCIDRList(features.Network.ExternalIPCIDRS)
	if len(features.Network.ExternalIPCIDRS) > 0 && len(externalIPCIDRs) == 0 {
		klog.Warningf("Failed to parse external CIDRs: %v", features.Network.ExternalIPCIDRS)
	}

	sortOrderCIDRs, ignoredCIDRs, err := ParseCIDRRuleset(features.Network.IPSortOrder)
	if err != nil {
		klog.Errorf("Failed to parse sort order CIDRs: %v", err)
	}

	if len(features.Network.IPSortOrder) > 0 && (len(sortOrderCIDRs)+len(ignoredCIDRs)) == 0 {
		klog.Warningf("Failed to parse sort order CIDRs: %v", features.Network.IPSortOrder)
	}

	netOps := instanceNetops{
		ExternalCIDRs:       externalIPCIDRs,
		SortOrder:           sortOrderCIDRs,
		IgnoredCIDRs:        ignoredCIDRs,
		Mode:                features.Network.Mode,
		IPv6SupportDisabled: features.Network.IPv6SupportDisabled,
	}

	return &instances{
		c:             client,
		zoneAsHAGroup: features.HAGroup,
		provider:      features.Provider,
		networkOpts:   netOps,
	}
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(4).InfoS("instances.InstanceExists() called", "node", klog.KRef("", node.Name))

	if node.Spec.ProviderID == "" {
		klog.V(4).InfoS("instances.InstanceExists() empty providerID, omitting unmanaged node", "node", klog.KObj(node))

		return true, nil
	}

	if !strings.HasPrefix(node.Spec.ProviderID, provider.ProviderName) {
		klog.V(4).InfoS("instances.InstanceExists() omitting unmanaged node", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

		return true, nil
	}

	mc := metrics.NewMetricContext("getVmInfo")
	if _, err := i.getInstanceInfo(ctx, node); mc.ObserveRequest(err) != nil {
		if err == cloudprovider.InstanceNotFound {
			klog.V(4).InfoS("instances.InstanceExists() instance not found", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

			return false, nil
		}

		return false, err
	}

	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(4).InfoS("instances.InstanceShutdown() called", "node", klog.KRef("", node.Name))

	if node.Spec.ProviderID == "" {
		klog.V(4).InfoS("instances.InstanceShutdown() empty providerID, omitting unmanaged node", "node", klog.KObj(node))

		return false, nil
	}

	if !strings.HasPrefix(node.Spec.ProviderID, provider.ProviderName) {
		klog.V(4).InfoS("instances.InstanceShutdown() omitting unmanaged node", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

		return false, nil
	}

	vmID, region, err := provider.ParseProviderID(node.Spec.ProviderID)
	if err != nil {
		klog.ErrorS(err, "instances.InstanceShutdown() failed to parse providerID", "providerID", node.Spec.ProviderID)

		return false, nil
	}

	px, err := i.c.pxpool.GetProxmoxCluster(region)
	if err != nil {
		klog.ErrorS(err, "instances.InstanceShutdown() failed to get Proxmox cluster", "region", region)

		return false, nil
	}

	mc := metrics.NewMetricContext("getVmState")

	vm, err := px.GetVMStatus(ctx, vmID)
	if mc.ObserveRequest(err) != nil {
		return false, err
	}

	if vm.Status == "stopped" {
		return true, nil
	}

	return false, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields in the Node object on registration.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.V(4).InfoS("instances.InstanceMetadata() called", "node", klog.KRef("", node.Name))

	var (
		info *instanceInfo
		err  error
	)

	providerID := node.Spec.ProviderID
	if providerID != "" && !strings.HasPrefix(providerID, provider.ProviderName) {
		klog.V(4).InfoS("instances.InstanceMetadata() omitting unmanaged node", "node", klog.KObj(node), "providerID", providerID)

		return &cloudprovider.InstanceMetadata{}, nil
	}

	mc := metrics.NewMetricContext("getInstanceInfo")

	info, err = i.getInstanceInfo(ctx, node)
	if mc.ObserveRequest(err) != nil {
		klog.ErrorS(err, "instances.InstanceMetadata() failed to get instance info", "node", klog.KObj(node))

		if err == proxmoxpool.ErrInstanceNotFound {
			klog.V(4).InfoS("instances.InstanceMetadata() instance not found", "node", klog.KObj(node), "providerID", providerID)

			return &cloudprovider.InstanceMetadata{}, nil
		}

		return nil, err
	}

	additionalLabels := map[string]string{
		LabelTopologyRegion: info.Region,
		LabelTopologyNode:   info.Node,
	}

	if providerID == "" {
		if i.provider == providerconfig.ProviderCapmox {
			providerID = provider.GetProviderIDFromUUID(info.UUID)
		} else {
			providerID = provider.GetProviderIDFromID(info.Region, info.ID)
		}

		annotations := map[string]string{
			AnnotationProxmoxInstanceID: fmt.Sprintf("%d", info.ID),
		}

		if err := syncNodeAnnotations(ctx, i.c.kclient, node, annotations); err != nil {
			klog.ErrorS(err, "error updating annotations for the node", "node", klog.KRef("", node.Name))
		}
	}

	metadata := &cloudprovider.InstanceMetadata{
		ProviderID:       providerID,
		NodeAddresses:    i.addresses(ctx, node, info),
		InstanceType:     info.Type,
		Zone:             info.Zone,
		Region:           info.Region,
		AdditionalLabels: additionalLabels,
	}

	if i.zoneAsHAGroup {
		haGroup, err := i.c.pxpool.GetNodeGroup(ctx, info.Region, info.Node)
		if err != nil {
			klog.ErrorS(err, "instances.InstanceMetadata() failed to get HA group for the node", "node", klog.KRef("", node.Name), "region", info.Region)

			return nil, err
		}

		metadata.Zone = haGroup
		additionalLabels[LabelTopologyHAGroup] = haGroup
	}

	if len(additionalLabels) > 0 && !hasUninitializedTaint(node) {
		if err := syncNodeLabels(i.c, node, additionalLabels); err != nil {
			klog.ErrorS(err, "error updating labels for the node", "node", klog.KRef("", node.Name))
		}
	}

	klog.V(5).InfoS("instances.InstanceMetadata()", "info", info, "metadata", metadata)

	return metadata, nil
}

func (i *instances) getInstanceInfo(ctx context.Context, node *v1.Node) (*instanceInfo, error) {
	klog.V(4).InfoS("instances.getInstanceInfo() called", "node", klog.KRef("", node.Name), "provider", i.provider)

	var (
		vmID   int
		region string
		err    error
	)

	providerID := node.Spec.ProviderID
	if providerID == "" && node.Annotations[AnnotationProxmoxInstanceID] != "" {
		region = node.Labels[LabelTopologyRegion]
		if region == "" {
			region = node.Labels[v1.LabelTopologyRegion]
		}

		vmID, err = strconv.Atoi(node.Annotations[AnnotationProxmoxInstanceID])
		if err != nil {
			return nil, fmt.Errorf("instances.getInstanceInfo() parse annotation error: %v", err)
		}

		if _, err := i.c.pxpool.GetProxmoxCluster(region); err == nil {
			providerID = provider.GetProviderIDFromID(region, vmID)

			klog.V(4).InfoS("instances.getInstanceInfo() set providerID", "node", klog.KObj(node), "providerID", providerID)
		}
	}

	if providerID == "" {
		klog.V(4).InfoS("instances.getInstanceInfo() empty providerID, trying find node", "node", klog.KObj(node))

		mc := metrics.NewMetricContext("findVmByName")

		vmID, region, err = i.c.pxpool.FindVMByNode(ctx, node)
		if mc.ObserveRequest(err) != nil {
			mc := metrics.NewMetricContext("findVmByUUID")

			vmID, region, err = i.c.pxpool.FindVMByUUID(ctx, node.Status.NodeInfo.SystemUUID)
			if mc.ObserveRequest(err) != nil {
				return nil, err
			}
		}

		if vmID == 0 {
			return nil, cloudprovider.InstanceNotFound
		}

		providerID = provider.GetProviderIDFromID(region, vmID)
	}

	if vmID == 0 {
		vmID, region, err = provider.ParseProviderID(providerID)
		if err != nil {
			if i.provider == providerconfig.ProviderDefault {
				return nil, fmt.Errorf("instances.getInstanceInfo() error: %v", err)
			}

			vmID, region, err = i.c.pxpool.FindVMByUUID(ctx, node.Status.NodeInfo.SystemUUID)
			if err != nil {
				if errors.Is(err, proxmoxpool.ErrInstanceNotFound) {
					return nil, cloudprovider.InstanceNotFound
				}

				return nil, fmt.Errorf("instances.getInstanceInfo() error: %v", err)
			}
		}
	}

	px, err := i.c.pxpool.GetProxmoxCluster(region)
	if err != nil {
		return nil, fmt.Errorf("instances.getInstanceInfo() error: %v", err)
	}

	mc := metrics.NewMetricContext("getVmInfo")

	vm, err := px.GetVMConfig(ctx, vmID)
	if mc.ObserveRequest(err) != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, cloudprovider.InstanceNotFound
		}

		return nil, err
	}

	info := &instanceInfo{
		ID:     vmID,
		UUID:   i.c.pxpool.GetVMUUID(vm),
		Name:   vm.Name,
		Node:   vm.Node,
		Region: region,
		Zone:   vm.Node,
	}

	if info.UUID != node.Status.NodeInfo.SystemUUID {
		klog.Errorf("instances.getInstanceInfo() node %s does not match SystemUUID=%s", info.Name, node.Status.NodeInfo.SystemUUID)

		return nil, cloudprovider.InstanceNotFound
	}

	if !strings.HasPrefix(info.Name, node.Name) {
		klog.Errorf("instances.getInstanceInfo() node %s does not match VM name=%s", node.Name, info.Name)

		return nil, cloudprovider.InstanceNotFound
	}

	info.Type = i.c.pxpool.GetVMSKU(vm)
	if !instanceTypeNameRegexp.MatchString(info.Type) {
		info.Type = fmt.Sprintf("%dVCPU-%dGB", vm.CPUs, vm.MaxMem/1024/1024/1024)
	}

	return info, nil
}
