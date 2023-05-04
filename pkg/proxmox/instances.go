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

	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/cluster"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"k8s.io/klog/v2"
)

type instances struct {
	c *cluster.Cluster
}

func newInstances(client *cluster.Cluster) *instances {
	return &instances{
		c: client,
	}
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceExists(_ context.Context, node *v1.Node) (bool, error) {
	klog.V(4).Info("instances.InstanceExists() called node: ", node.Name)

	if !strings.HasPrefix(node.Spec.ProviderID, ProviderName) {
		klog.V(4).Infof("instances.InstanceExists() node %s has foreign providerID: %s, skipped", node.Name, node.Spec.ProviderID)

		return true, nil
	}

	_, _, err := i.getInstance(node)
	if err != nil {
		if err == cloudprovider.InstanceNotFound {
			klog.V(4).Infof("instances.InstanceExists() instance %s not found", node.Name)

			return false, nil
		}

		return false, err
	}

	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceShutdown(_ context.Context, node *v1.Node) (bool, error) {
	klog.V(4).Info("instances.InstanceShutdown() called, node: ", node.Name)

	if !strings.HasPrefix(node.Spec.ProviderID, ProviderName) {
		klog.V(4).Infof("instances.InstanceShutdown() node %s has foreign providerID: %s, skipped", node.Name, node.Spec.ProviderID)

		return false, nil
	}

	vmr, region, err := i.parseProviderID(node.Spec.ProviderID)
	if err != nil {
		klog.Errorf("instances.InstanceShutdown() failed to parse providerID %s: %v", node.Spec.ProviderID, err)

		return false, nil
	}

	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		klog.Errorf("instances.InstanceShutdown() failed to get Proxmox cluster: %v", err)

		return false, nil
	}

	vmState, err := px.GetVmState(vmr)
	if err != nil {
		return false, err
	}

	if vmState["status"].(string) == "stopped" {
		return true, nil
	}

	return false, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields in the Node object on registration.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceMetadata(_ context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.V(4).Info("instances.InstanceMetadata() called, node: ", node.Name)

	if providedIP, ok := node.ObjectMeta.Annotations[cloudproviderapi.AnnotationAlphaProvidedIPAddr]; ok {
		var (
			vmRef  *pxapi.VmRef
			region string
			err    error
		)

		providerID := node.Spec.ProviderID
		if providerID == "" {
			klog.V(4).Infof("instances.InstanceMetadata() - trying to find providerID for node %s", node.Name)

			vmRef, region, err = i.c.FindVMByName(node.Name)
			if err != nil {
				return nil, fmt.Errorf("instances.InstanceMetadata() - failed to find instance by name %s: %v, skipped", node.Name, err)
			}
		} else if !strings.HasPrefix(node.Spec.ProviderID, ProviderName) {
			klog.V(4).Infof("instances.InstanceMetadata() node %s has foreign providerID: %s, skipped", node.Name, node.Spec.ProviderID)

			return &cloudprovider.InstanceMetadata{}, nil
		}

		if vmRef == nil {
			vmRef, region, err = i.getInstance(node)
			if err != nil {
				return nil, err
			}
		}

		addresses := []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: providedIP}}
		addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: node.Name})

		instanceType, err := i.getInstanceType(vmRef, region)
		if err != nil {
			instanceType = vmRef.GetVmType()
		}

		return &cloudprovider.InstanceMetadata{
			ProviderID:    i.getProviderID(region, vmRef),
			NodeAddresses: addresses,
			InstanceType:  instanceType,
			Zone:          vmRef.Node(),
			Region:        region,
		}, nil
	}

	klog.Infof("instances.InstanceMetadata() is kubelet has --cloud-provider=external on the node %s?", node.Name)

	return &cloudprovider.InstanceMetadata{}, nil
}

func (i *instances) getInstance(node *v1.Node) (*pxapi.VmRef, string, error) {
	vm, region, err := i.parseProviderID(node.Spec.ProviderID)
	if err != nil {
		return nil, "", fmt.Errorf("instances.getInstance() error: %v", err)
	}

	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		return nil, "", fmt.Errorf("instances.getInstance() error: %v", err)
	}

	vmInfo, err := px.GetVmInfo(vm)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, "", cloudprovider.InstanceNotFound
		}

		return nil, "", err
	}

	if vmInfo["name"].(string) != node.Name {
		return nil, "", fmt.Errorf("instances.getInstance() vm.name(%s) != node.name(%s)", vmInfo["name"].(string), node.Name)
	}

	klog.V(5).Infof("instances.getInstance() vmInfo %+v", vmInfo)

	return vm, region, nil
}

func (i *instances) getInstanceType(vmRef *pxapi.VmRef, region string) (string, error) {
	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		return "", err
	}

	vmInfo, err := px.GetVmInfo(vmRef)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%.0fVCPU-%.0fGB",
		vmInfo["maxcpu"].(float64),
		vmInfo["maxmem"].(float64)/1024/1024/1024), nil
}

var providerIDRegexp = regexp.MustCompile(`^` + ProviderName + `://([^/]*)/([^/]+)$`)

func (i *instances) getProviderID(region string, vmr *pxapi.VmRef) string {
	return fmt.Sprintf("%s://%s/%d", ProviderName, region, vmr.VmId())
}

func (i *instances) parseProviderID(providerID string) (*pxapi.VmRef, string, error) {
	if !strings.HasPrefix(providerID, ProviderName) {
		return nil, "", fmt.Errorf("foreign providerID or empty \"%s\"", providerID)
	}

	matches := providerIDRegexp.FindStringSubmatch(providerID)
	if len(matches) != 3 {
		return nil, "", fmt.Errorf("providerID \"%s\" didn't match expected format \"%s://region/InstanceID\"", providerID, ProviderName)
	}

	vmID, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, "", fmt.Errorf("providerID \"%s\" didn't match expected format \"%s://region/InstanceID\"", providerID, ProviderName)
	}

	return pxapi.NewVmRef(vmID), matches[1], nil
}
