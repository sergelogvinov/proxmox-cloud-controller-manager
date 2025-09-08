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

// Package proxmox is main CCM definition.
package proxmox

import (
	"context"
	"io"

	ccmConfig "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/config"
	provider "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/provider"
	pxpool "github.com/sergelogvinov/proxmox-cloud-controller-manager/pkg/proxmoxpool"

	clientkubernetes "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

const (
	// ProviderName is the name of the Proxmox provider.
	ProviderName = provider.ProviderName

	// ServiceAccountName is the service account name used in kube-system namespace.
	ServiceAccountName = provider.ProviderName + "-cloud-controller-manager"

	// Group name
	Group = "proxmox.sinextra.dev"
)

type cloud struct {
	client *client

	instancesV2 cloudprovider.InstancesV2

	ctx  context.Context //nolint:containedctx
	stop func()
}

type client struct {
	pxpool  *pxpool.ProxmoxPool
	kclient clientkubernetes.Interface
}

func init() {
	cloudprovider.RegisterCloudProvider(provider.ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := ccmConfig.ReadCloudConfig(config)
		if err != nil {
			klog.ErrorS(err, "failed to read config")

			return nil, err
		}

		return newCloud(&cfg)
	})
}

func newCloud(config *ccmConfig.ClustersConfig) (cloudprovider.Interface, error) {
	client, err := newClient(config.Clusters)
	if err != nil {
		return nil, err
	}

	instancesInterface := newInstances(client, config.Features)

	return &cloud{
		client:      client,
		instancesV2: instancesInterface,
	}, nil
}

func newClient(clusters []*pxpool.ProxmoxCluster) (*client, error) {
	px, err := pxpool.NewProxmoxPool(clusters, nil)
	if err != nil {
		return nil, err
	}

	return &client{
		pxpool: px,
	}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	c.client.kclient = clientBuilder.ClientOrDie(ServiceAccountName)

	klog.InfoS("clientset initialized")

	ctx, cancel := context.WithCancel(context.Background())
	c.ctx = ctx
	c.stop = cancel

	err := c.client.pxpool.CheckClusters(ctx)
	if err != nil {
		klog.ErrorS(err, "failed to check proxmox cluster")
	}

	// Broadcast the upstream stop signal to all provider-level goroutines
	// watching the provider's context for cancellation.
	go func(provider *cloud) {
		<-stop
		klog.V(3).InfoS("received cloud provider termination signal")
		provider.stop()
	}(c)

	klog.InfoS("proxmox initialized")
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
	return provider.ProviderName
}

// HasClusterID is not implemented.
func (c *cloud) HasClusterID() bool {
	return true
}
