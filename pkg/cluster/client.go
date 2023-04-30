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

// Package cluster implements the multi-cloud provider interface for Proxmox.
package cluster

import (
	"crypto/tls"
	"fmt"
	"os"

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"
)

// Client is a Proxmox client.
type Client struct {
	config  *ClustersConfig
	proxmox map[string]*pxapi.Client
}

// NewClient creates a new Proxmox client.
func NewClient(config *ClustersConfig) (*Client, error) {
	clusters := len(config.Clusters)
	if clusters > 0 {
		proxmox := make(map[string]*pxapi.Client, clusters)

		for _, cfg := range config.Clusters {
			tlsconf := &tls.Config{InsecureSkipVerify: true}
			if !cfg.Insecure {
				tlsconf = nil
			}

			client, err := pxapi.NewClient(cfg.URL, nil, os.Getenv("PM_HTTP_HEADERS"), tlsconf, "", 600)
			if err != nil {
				return nil, err
			}

			client.SetAPIToken(cfg.TokenID, cfg.TokenSecret)

			proxmox[cfg.Region] = client
		}

		return &Client{
			config:  config,
			proxmox: proxmox,
		}, nil
	}

	return nil, fmt.Errorf("no Proxmox clusters found")
}

// CheckClusters checks if the Proxmox connection is working.
func (c *Client) CheckClusters() error {
	for region, client := range c.proxmox {
		if _, err := client.GetVersion(); err != nil {
			return fmt.Errorf("failed to initialized proxmox client in region %s, error: %v", region, err)
		}
	}

	return nil
}

// GetProxmoxCluster returns a Proxmox cluster client in a given region.
func (c *Client) GetProxmoxCluster(region string) (*pxapi.Client, error) {
	if c.proxmox[region] != nil {
		return c.proxmox[region], nil
	}

	return nil, fmt.Errorf("proxmox cluster %s not found", region)
}

// FindVMByName find a VM by name in all Proxmox clusters.
func (c *Client) FindVMByName(name string) (*pxapi.VmRef, string, error) {
	for region, px := range c.proxmox {
		vmr, err := px.GetVmRefByName(name)
		if err != nil {
			continue
		}

		return vmr, region, nil
	}

	return nil, "", fmt.Errorf("VM %s not found", name)
}
