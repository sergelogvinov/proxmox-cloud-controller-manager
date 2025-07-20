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
	"regexp"
	"strconv"
	"strings"

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"

	ccmConfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
	metrics "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/metrics"
	provider "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/provider"
	pxpool "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"k8s.io/klog/v2"
)

type instances struct {
	c        *pxpool.ProxmoxPool
	provider ccmConfig.Provider
}

var instanceTypeNameRegexp = regexp.MustCompile(`(^[a-zA-Z0-9_.-]+)$`)

func newInstances(client *pxpool.ProxmoxPool, provider ccmConfig.Provider) *instances {
	return &instances{
		c:        client,
		provider: provider,
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
	if _, _, err := i.getInstance(ctx, node); mc.ObserveRequest(err) != nil {
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

	vmr, region, err := provider.ParseProviderID(node.Spec.ProviderID)
	if err != nil {
		klog.ErrorS(err, "instances.InstanceShutdown() failed to parse providerID", "providerID", node.Spec.ProviderID)

		return false, nil
	}

	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		klog.ErrorS(err, "instances.InstanceShutdown() failed to get Proxmox cluster", "region", region)

		return false, nil
	}

	mc := metrics.NewMetricContext("getVmState")

	vmState, err := px.GetVmState(ctx, vmr)
	if mc.ObserveRequest(err) != nil {
		return false, err
	}

	if vmState["status"].(string) == "stopped" { //nolint:errcheck
		return true, nil
	}

	return false, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields in the Node object on registration.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.V(4).InfoS("instances.InstanceMetadata() called", "node", klog.KRef("", node.Name))

	if providedIP, ok := node.ObjectMeta.Annotations[cloudproviderapi.AnnotationAlphaProvidedIPAddr]; ok {
		var (
			vmRef  *pxapi.VmRef
			region string
			err    error
		)

		providerID := node.Spec.ProviderID
		if providerID == "" {
			uuid := node.Status.NodeInfo.SystemUUID

			klog.V(4).InfoS("instances.InstanceMetadata() empty providerID, trying find node", "node", klog.KObj(node), "uuid", uuid)

			mc := metrics.NewMetricContext("findVmByName")

			vmRef, region, err = i.c.FindVMByNode(ctx, node)
			if mc.ObserveRequest(err) != nil {
				mc := metrics.NewMetricContext("findVmByUUID")

				vmRef, region, err = i.c.FindVMByUUID(ctx, uuid)
				if mc.ObserveRequest(err) != nil {
					return nil, fmt.Errorf("instances.InstanceMetadata() - failed to find instance by name/uuid %s: %v, skipped", node.Name, err)
				}
			}

			if i.provider == ccmConfig.ProviderCapmox {
				providerID = provider.GetProviderIDFromUUID(uuid)
			} else {
				providerID = provider.GetProviderID(region, vmRef)
			}
		} else if !strings.HasPrefix(node.Spec.ProviderID, provider.ProviderName) {
			klog.V(4).InfoS("instances.InstanceMetadata() omitting unmanaged node", "node", klog.KObj(node), "providerID", node.Spec.ProviderID)

			return &cloudprovider.InstanceMetadata{}, nil
		}

		if vmRef == nil {
			mc := metrics.NewMetricContext("getVmInfo")

			vmRef, region, err = i.getInstance(ctx, node)
			if mc.ObserveRequest(err) != nil {
				return nil, err
			}
		}

		addresses := []v1.NodeAddress{}

		for _, ip := range strings.Split(providedIP, ",") {
			addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: ip})
		}

		addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: node.Name})

		instanceType, err := i.getInstanceType(ctx, vmRef, region)
		if err != nil {
			instanceType = vmRef.GetVmType()
		}

		return &cloudprovider.InstanceMetadata{
			ProviderID:    providerID,
			NodeAddresses: addresses,
			InstanceType:  instanceType,
			Zone:          vmRef.Node().String(),
			Region:        region,
		}, nil
	}

	klog.InfoS(fmt.Sprintf(
		"instances.InstanceMetadata() called: label %s missing from node. Was kubelet started without --cloud-provider=external?",
		cloudproviderapi.AnnotationAlphaProvidedIPAddr),
		node, klog.KRef("", node.Name))

	return &cloudprovider.InstanceMetadata{}, nil
}

func (i *instances) getInstance(ctx context.Context, node *v1.Node) (*pxapi.VmRef, string, error) {
	klog.V(4).InfoS("instances.getInstance() called", "node", klog.KRef("", node.Name), "provider", i.provider)

	if i.provider == ccmConfig.ProviderCapmox {
		uuid := node.Status.NodeInfo.SystemUUID

		vmRef, region, err := i.c.FindVMByUUID(ctx, uuid)
		if err != nil {
			return nil, "", fmt.Errorf("instances.getInstance() error: %v", err)
		}

		return vmRef, region, nil
	}

	vmRef, region, err := provider.ParseProviderID(node.Spec.ProviderID)
	if err != nil {
		return nil, "", fmt.Errorf("instances.getInstance() error: %v", err)
	}

	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		return nil, "", fmt.Errorf("instances.getInstance() error: %v", err)
	}

	mc := metrics.NewMetricContext("getVmInfo")

	vmConfig, err := px.GetVmConfig(ctx, vmRef)
	if mc.ObserveRequest(err) != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, "", cloudprovider.InstanceNotFound
		}

		return nil, "", err
	}

	if i.c.GetVMName(vmConfig) != node.Name || i.c.GetVMUUID(vmConfig) != node.Status.NodeInfo.SystemUUID {
		klog.Errorf("instances.getInstance() vm.name(%s) != node.name(%s) with uuid=%s", i.c.GetVMName(vmConfig), node.Name, node.Status.NodeInfo.SystemUUID)

		return nil, "", cloudprovider.InstanceNotFound
	}

	klog.V(5).Infof("instances.getInstance() vmConfig %+v", vmConfig)

	return vmRef, region, nil
}

func (i *instances) getInstanceType(ctx context.Context, vmRef *pxapi.VmRef, region string) (string, error) {
	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		return "", err
	}

	mc := metrics.NewMetricContext("getVmInfo")

	vmConfig, err := px.GetVmConfig(ctx, vmRef)
	if mc.ObserveRequest(err) != nil {
		return "", err
	}

	sku := i.c.GetVMSKU(vmConfig)
	if sku != "" && instanceTypeNameRegexp.MatchString(sku) {
		return sku, nil
	}

	if vmConfig["cores"] == nil || vmConfig["memory"] == nil {
		return "", fmt.Errorf("instances.getInstanceType() failed to get instance type")
	}

	memory, err := strconv.Atoi(vmConfig["memory"].(string)) //nolint:errcheck
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%.0fVCPU-%.0fGB",
		vmConfig["cores"].(float64), //nolint:errcheck
		float64(memory)/1024), nil
}
