package proxmox

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"
	"k8s.io/klog/v2"
)

type instances struct {
	c *client
}

func newInstances(client *client) *instances {
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

	vmRef, region, err := i.getInstance(node)
	if err != nil {
		return false, err
	}

	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		return false, err
	}

	vmState, err := px.client.GetVmState(vmRef)
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
		)

		providerID := node.Spec.ProviderID
		if providerID == "" {
			klog.V(4).Infof("instances.InstanceMetadata() - trying to find providerID for node %s", node.Name)

			for _, px := range i.c.proxmox {
				vm, err := px.client.GetVmRefByName(node.Name)
				if err != nil {
					continue
				}

				vmRef = vm
				region = px.region

				break
			}
		} else if !strings.HasPrefix(node.Spec.ProviderID, ProviderName) {
			klog.V(4).Infof("instances.InstanceMetadata() node %s has foreign providerID: %s, skipped", node.Name, node.Spec.ProviderID)

			return &cloudprovider.InstanceMetadata{}, nil
		}

		if vmRef == nil {
			var err error

			vmRef, region, err = i.getInstance(node)
			if err != nil {
				return nil, err
			}
		}

		addresses := []v1.NodeAddress{{Type: v1.NodeInternalIP, Address: providedIP}}
		addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: node.Name})

		providerID = fmt.Sprintf("%s://%s/%d", ProviderName, region, vmRef.VmId())

		instanceType, err := i.getInstanceType(vmRef, region)
		if err != nil {
			instanceType = vmRef.GetVmType()
		}

		return &cloudprovider.InstanceMetadata{
			ProviderID:    providerID,
			NodeAddresses: addresses,
			InstanceType:  instanceType,
			Zone:          vmRef.Node(),
			Region:        region,
		}, nil
	}

	return &cloudprovider.InstanceMetadata{}, nil
}

func (i *instances) getInstance(node *v1.Node) (*pxapi.VmRef, string, error) {
	if !strings.HasPrefix(node.Spec.ProviderID, ProviderName) {
		klog.V(4).Infof("instances.getInstance() node %s has foreign providerID: %s, skipped", node.Name, node.Spec.ProviderID)

		return nil, "", fmt.Errorf("node %s has foreign providerID: %s", node.Name, node.Spec.ProviderID)
	}

	vmid, region, err := i.parseProviderID(node.Spec.ProviderID)
	if err != nil {
		return nil, "", err
	}

	vmRef := pxapi.NewVmRef(vmid)

	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		return nil, "", err
	}

	vmInfo, err := px.client.GetVmInfo(vmRef)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, "", cloudprovider.InstanceNotFound
		}

		return nil, "", err
	}

	klog.V(5).Infof("instances.getInstance() vmInfo %+v", vmInfo)

	return vmRef, region, nil
}

func (i *instances) getInstanceType(vmRef *pxapi.VmRef, region string) (string, error) {
	px, err := i.c.GetProxmoxCluster(region)
	if err != nil {
		return "", err
	}

	vmInfo, err := px.client.GetVmInfo(vmRef)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%.0fVCPU-%.0fGB",
		vmInfo["maxcpu"].(float64),
		vmInfo["maxmem"].(float64)/1024/1024/1024), nil
}

var providerIDRegexp = regexp.MustCompile(`^` + ProviderName + `://([^/]*)/([^/]+)$`)

func (i *instances) parseProviderID(providerID string) (int, string, error) {
	matches := providerIDRegexp.FindStringSubmatch(providerID)
	if len(matches) != 3 {
		return 0, "", fmt.Errorf("ProviderID \"%s\" didn't match expected format \"%s://region/InstanceID\"", providerID, ProviderName)
	}

	vmID, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, "", err
	}

	return vmID, matches[1], nil
}
