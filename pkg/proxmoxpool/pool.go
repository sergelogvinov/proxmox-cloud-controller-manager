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

// Package proxmoxpool provides a pool of Telmate/proxmox-api-go/proxmox clients
package proxmoxpool

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"go.uber.org/multierr"

	goproxmox "github.com/sergelogvinov/go-proxmox-rest"
	"github.com/sergelogvinov/go-proxmox-rest/cluster"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// ProxmoxCluster defines a Proxmox cluster configuration.
type ProxmoxCluster struct {
	URL             string `yaml:"url"`
	Insecure        bool   `yaml:"insecure,omitempty"`
	TokenID         string `yaml:"token_id,omitempty"`
	TokenIDFile     string `yaml:"token_id_file,omitempty"`
	TokenSecret     string `yaml:"token_secret,omitempty"`
	TokenSecretFile string `yaml:"token_secret_file,omitempty"`
	Username        string `yaml:"username,omitempty"`
	Password        string `yaml:"password,omitempty"`
	Region          string `yaml:"region,omitempty"`
}

// ProxmoxPool is a Proxmox client pool of proxmox clusters.
type ProxmoxPool struct {
	clients map[string]*goproxmox.Client
}

// NewProxmoxPool creates a new Proxmox cluster client.
func NewProxmoxPool(config []*ProxmoxCluster) (*ProxmoxPool, error) {
	clusters := len(config)
	if clusters > 0 {
		clients := make(map[string]*goproxmox.Client, clusters)

		for _, cfg := range config {
			opts := []goproxmox.Option{
				goproxmox.WithURL(cfg.URL),
				goproxmox.WithInsecure(cfg.Insecure),
				goproxmox.WithUserAgent("ProxmoxCCM/1.0"),
			}

			if cfg.TokenID == "" && cfg.TokenIDFile != "" {
				var err error

				cfg.TokenID, err = readValueFromFile(cfg.TokenIDFile)
				if err != nil {
					return nil, err
				}
			}

			if cfg.TokenSecret == "" && cfg.TokenSecretFile != "" {
				var err error

				cfg.TokenSecret, err = readValueFromFile(cfg.TokenSecretFile)
				if err != nil {
					return nil, err
				}
			}

			if cfg.Username != "" && cfg.Password != "" {
				opts = append(opts, goproxmox.WithPasswordAuth(cfg.Username, cfg.Password))
			} else if cfg.TokenID != "" && cfg.TokenSecret != "" {
				opts = append(opts, goproxmox.WithTokenAuth(cfg.TokenID, cfg.TokenSecret))
			}

			pxClient, err := goproxmox.New(goproxmox.ClientConfig{}, opts...)
			if err != nil {
				return nil, err
			}

			clients[cfg.Region] = pxClient
		}

		return &ProxmoxPool{
			clients: clients,
		}, nil
	}

	return nil, ErrClustersNotFound
}

// GetRegions returns supported regions.
func (c *ProxmoxPool) GetRegions() []string {
	regions := make([]string, 0, len(c.clients))

	for region := range c.clients {
		regions = append(regions, region)
	}

	return regions
}

// CheckClusters checks if the Proxmox connection is working.
func (c *ProxmoxPool) CheckClusters(ctx context.Context) error {
	for region, pxClient := range c.clients {
		info, err := pxClient.Version(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialized proxmox client in region %s, error: %v", region, err)
		}

		// Check if we can have permission to list VMs
		vms, err := pxClient.Cluster().Resources().List(ctx, cluster.ListFilter{Type: cluster.ResourceTypeVM})
		if err != nil {
			return fmt.Errorf("failed to get list of VMs in region %s, error: %v", region, err)
		}

		if len(vms) > 0 {
			klog.V(4).InfoS("Proxmox cluster information", "region", region, "version", info.Version, "vms", len(vms))
		} else {
			klog.InfoS("Proxmox cluster has no VMs, or check the account permission", "region", region)
		}
	}

	return nil
}

// GetProxmoxCluster returns a Proxmox cluster client in a given region.
func (c *ProxmoxPool) GetProxmoxCluster(region string) (*goproxmox.Client, error) {
	if c.clients[region] != nil {
		return c.clients[region], nil
	}

	return nil, ErrRegionNotFound
}

// GetNodeHAGroups returns a Proxmox node ha-group in a given region for the node.
//
// +proxmox:rbac:feature=hagroup
func (c *ProxmoxPool) GetNodeHAGroups(ctx context.Context, region string, node string) ([]string, error) {
	groups := []string{}

	px, err := c.GetProxmoxCluster(region)
	if err != nil {
		return nil, err
	}

	haGroups, err := px.Cluster().HA().Groups().List(ctx)
	if err != nil {
		return nil, fmt.Errorf("error get ha-groups %v", err)
	}

	for _, g := range haGroups {
		if g.Type != "group" {
			continue
		}

		for n := range strings.SplitSeq(g.Nodes, ",") {
			if node == strings.Split(n, ":")[0] {
				groups = append(groups, g.Group)
			}
		}
	}

	if len(groups) > 0 {
		slices.Sort(groups)

		return groups, nil
	}

	return nil, ErrHAGroupNotFound
}

// GetVMResourceByID returns a summary of a VM by its ID in a given region.
//
// +proxmox:rbac:feature=base
func (c *ProxmoxPool) GetVMResourceByID(ctx context.Context, region string, vmID int) (*VMResource, error) {
	px, err := c.GetProxmoxCluster(region)
	if err != nil {
		return nil, err
	}

	resources, err := px.Cluster().Resources().List(ctx, cluster.ListFilter{
		Type:          cluster.ResourceTypeVM,
		GuestType:     "qemu",
		SkipTemplates: true,
		VMID:          vmID,
	})
	if err != nil {
		return nil, err
	}

	if len(resources) == 0 {
		return nil, ErrInstanceNotFound
	}

	rs := &resources[0]

	return &VMResource{
		VMID:   rs.VMID,
		Node:   rs.Node,
		Name:   rs.Name,
		Status: rs.Status,
	}, nil
}

// findVMResource scans the region's VM resources (skipping templates and
// non-qemu entries) and returns the first one for which match returns true.
// Returns ErrInstanceNotFound if nothing matches.
//
// +proxmox:rbac:feature=base
func findVMResource(ctx context.Context, px *goproxmox.Client, match func(*cluster.Resource) (bool, error)) (*cluster.Resource, error) {
	resources, err := px.Cluster().Resources().List(ctx, cluster.ListFilter{
		Type:          cluster.ResourceTypeVM,
		GuestType:     "qemu",
		SkipTemplates: true,
		Match:         match,
	})
	if err != nil {
		return nil, err
	}

	if len(resources) == 0 {
		return nil, ErrInstanceNotFound
	}

	return &resources[0], nil
}

// getVMUUID returns a VM's SMBIOS UUID, given the node it runs on.
//
// +proxmox:rbac:feature=base
func getVMUUID(ctx context.Context, px *goproxmox.Client, node string, vmid int) (string, error) {
	cfg, err := px.Nodes(node).Qemu().Config(ctx, vmid, nil)
	if err != nil {
		return "", err
	}

	if cfg.SMBios1 == nil {
		return "", nil
	}

	return cfg.SMBios1.UUID, nil
}

// VMResource is a minimal, client-agnostic summary of a VM found via a
// region's cluster-wide resource list (GET /cluster/resources).
type VMResource struct {
	VMID int
	Node string
	Name string
	// Status is the resource's cluster-wide status, e.g. "running",
	// "stopped", or "unknown" when the VM's node is unreachable.
	Status string
}

// VMDetails holds the VM config/status fields the CCM needs to build
// instance metadata: its node, live CPU/memory, and SMBIOS-derived UUID and
// instance type.
type VMDetails struct {
	VMID   int
	Node   string
	Name   string
	CPUs   float64
	MaxMem int64
	UUID   string
	Type   string
}

// GetVMConfig returns a VM's config and live status by its ID in a given
// region. Returns ErrNodeInaccessible if the VM's node is unreachable
// (reported as Status "unknown" in the cluster resource list).
//
// +proxmox:rbac:feature=base
func (c *ProxmoxPool) GetVMConfig(ctx context.Context, region string, vmID int) (*VMDetails, error) {
	rs, err := c.GetVMResourceByID(ctx, region, vmID)
	if err != nil {
		return nil, err
	}

	if rs.Status == "unknown" {
		return nil, ErrNodeInaccessible
	}

	px, err := c.GetProxmoxCluster(region)
	if err != nil {
		return nil, err
	}

	cfg, err := px.Nodes(rs.Node).Qemu().Config(ctx, vmID, nil)
	if err != nil {
		return nil, err
	}

	status, err := px.Nodes(rs.Node).Qemu().Status(ctx, vmID)
	if err != nil {
		return nil, err
	}

	details := &VMDetails{
		VMID:   rs.VMID,
		Node:   rs.Node,
		Name:   status.Name,
		CPUs:   status.CPUs,
		MaxMem: status.MaxMem,
	}

	if cfg.SMBios1 != nil {
		details.UUID = cfg.SMBios1.UUID

		if sku, err := base64.StdEncoding.DecodeString(cfg.SMBios1.SKU); err == nil {
			details.Type = string(sku)
		}
	}

	return details, nil
}

// FindVMByNode find a VM by kubernetes node resource in all Proxmox clusters.
func (c *ProxmoxPool) FindVMByNode(ctx context.Context, node *v1.Node) (vmID int, region string, err error) {
	var errs error

	for region, px := range c.clients {
		vm, err := findVMResource(ctx, px, func(rs *cluster.Resource) (bool, error) {
			if !strings.HasPrefix(rs.Name, node.Name) {
				return false, nil
			}

			if rs.Status == "unknown" {
				errs = multierr.Append(errs, fmt.Errorf("region %s node %s: %w", region, rs.Node, ErrNodeInaccessible))

				return false, nil //nolint: nilerr
			}

			uuid, err := getVMUUID(ctx, px, rs.Node, rs.VMID)
			if err != nil {
				return false, err
			}

			if uuid == "" {
				return false, nil
			}

			return strings.EqualFold(uuid, node.Status.NodeInfo.SystemUUID), nil
		})
		if err != nil {
			if errors.Is(err, ErrInstanceNotFound) {
				continue
			}

			return 0, "", err
		}

		return vm.VMID, region, nil
	}

	if errs != nil {
		return 0, "", errs
	}

	return 0, "", ErrInstanceNotFound
}

// FindVMByUUID find a VM by uuid in all Proxmox clusters.
func (c *ProxmoxPool) FindVMByUUID(ctx context.Context, uuid string) (vmID int, region string, err error) {
	var errs error

	for region, px := range c.clients {
		vm, err := findVMResource(ctx, px, func(rs *cluster.Resource) (bool, error) {
			if rs.Status == "unknown" {
				errs = multierr.Append(errs, fmt.Errorf("region %s node %s: %w", region, rs.Node, ErrNodeInaccessible))

				return false, nil //nolint: nilerr
			}

			vmUUID, err := getVMUUID(ctx, px, rs.Node, rs.VMID)
			if err != nil {
				return false, err
			}

			if vmUUID == "" {
				return false, nil
			}

			return strings.EqualFold(vmUUID, uuid), nil
		})
		if err != nil {
			if errors.Is(err, ErrInstanceNotFound) {
				continue
			}

			return 0, "", err
		}

		return vm.VMID, region, nil
	}

	if errs != nil {
		return 0, "", errs
	}

	return 0, "", ErrInstanceNotFound
}

func readValueFromFile(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file '%s': %w", path, err)
	}

	return strings.TrimSpace(string(content)), nil
}
