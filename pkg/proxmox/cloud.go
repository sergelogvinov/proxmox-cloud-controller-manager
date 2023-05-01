// Package proxmox is main CCM defenition.
package proxmox

import (
	"context"
	"io"

	"github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/cluster"

	clientkubernetes "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName is the name of the Proxmox provider.
	ProviderName = "proxmox"
	// ServiceAccountName is the service account name used in kube-system namespace.
	ServiceAccountName = "proxmox-cloud-controller-manager"
)

type cloud struct {
	client      *cluster.Cluster
	kclient     clientkubernetes.Interface
	instancesV2 cloudprovider.InstancesV2

	ctx  context.Context //nolint:containedctx
	stop func()
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := cluster.ReadCloudConfig(config)
		if err != nil {
			klog.Errorf("failed to read config: %v", err)

			return nil, err
		}

		return newCloud(&cfg)
	})
}

func newCloud(config *cluster.ClustersConfig) (cloudprovider.Interface, error) {
	client, err := cluster.NewCluster(config, nil)
	if err != nil {
		return nil, err
	}

	instancesInterface := newInstances(client)

	return &cloud{
		client:      client,
		instancesV2: instancesInterface,
	}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	c.kclient = clientBuilder.ClientOrDie(ServiceAccountName)

	klog.Infof("clientset initialized")

	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.stop = cancel

	err := c.client.CheckClusters()
	if err != nil {
		klog.Errorf("failed to check proxmox cluster: %v", err)
	}

	// Broadcast the upstream stop signal to all provider-level goroutines
	// watching the provider's context for cancellation.
	go func(provider *cloud) {
		<-stop
		klog.V(3).Infof("received cloud provider termination signal")
		provider.stop()
	}(c)

	klog.Infof("proxmox initialized")
}

// LoadBalancer returns a balancer interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

// Instances returns an instances interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce
// API calls to the cloud provider when registering and syncing nodes.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return c.instancesV2, c.instancesV2 != nil
}

// Zones returns a zones interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// Clusters is not implemented.
func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes is not implemented.
func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloud) ProviderName() string {
	return ProviderName
}

// HasClusterID is not implemented.
func (c *cloud) HasClusterID() bool {
	return true
}
