package proxmox

import (
	"context"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
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

	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceShutdown(_ context.Context, node *v1.Node) (bool, error) {
	klog.V(4).Info("instances.InstanceShutdown() called, node: ", node.Name)

	return true, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields in the Node object on registration.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instances) InstanceMetadata(_ context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.V(4).Info("instances.InstanceMetadata() called, node: ", node.Name)

	return &cloudprovider.InstanceMetadata{}, nil
}
