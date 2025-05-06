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
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// Cluster is a Proxmox client.
type Cluster struct {
	config  *ClustersConfig
	proxmox map[string]*pxapi.Client
}

// NewCluster creates a new Proxmox cluster client.
func NewCluster(config *ClustersConfig, hclient *http.Client) (*Cluster, error) {
	clusters := len(config.Clusters)
	if clusters > 0 {
		proxmox := make(map[string]*pxapi.Client, clusters)

		for _, cfg := range config.Clusters {
			tlsconf := &tls.Config{InsecureSkipVerify: true}
			if !cfg.Insecure {
				tlsconf = nil
			}

			client, err := pxapi.NewClient(cfg.URL, hclient, os.Getenv("PM_HTTP_HEADERS"), tlsconf, "", 600)
			if err != nil {
				return nil, err
			}

			if cfg.Username != "" && cfg.Password != "" {
				if err := client.Login(context.Background(), cfg.Username, cfg.Password, ""); err != nil {
					return nil, err
				}
			} else {
				client.SetAPIToken(cfg.TokenID, cfg.TokenSecret)
			}

			proxmox[cfg.Region] = client
		}

		return &Cluster{
			config:  config,
			proxmox: proxmox,
		}, nil
	}

	return nil, fmt.Errorf("no Proxmox clusters found")
}

// CheckClusters checks if the Proxmox connection is working.
func (c *Cluster) CheckClusters(ctx context.Context) error {
	for region, client := range c.proxmox {
		if _, err := client.GetVersion(ctx); err != nil {
			return fmt.Errorf("failed to initialized proxmox client in region %s, error: %v", region, err)
		}

		vmlist, err := client.GetVmList(ctx)
		if err != nil {
			return fmt.Errorf("failed to get list of VMs in region %s, error: %v", region, err)
		}

		vms, ok := vmlist["data"].([]interface{})
		if !ok {
			return fmt.Errorf("failed to cast response to list of VMs in region %s, error: %v", region, err)
		}

		if len(vms) > 0 {
			klog.V(4).InfoS("Proxmox cluster has VMs", "region", region, "count", len(vms))
		} else {
			klog.InfoS("Proxmox cluster has no VMs, or check the account permission", "region", region)
		}
	}

	return nil
}

// GetProxmoxCluster returns a Proxmox cluster client in a given region.
func (c *Cluster) GetProxmoxCluster(region string) (*pxapi.Client, error) {
	if c.proxmox[region] != nil {
		return c.proxmox[region], nil
	}

	return nil, fmt.Errorf("proxmox cluster %s not found", region)
}

// FindVMByNode find a VM by kubernetes node resource in all Proxmox clusters.
func (c *Cluster) FindVMByNode(ctx context.Context, node *v1.Node) (*pxapi.VmRef, string, error) {
	for region, px := range c.proxmox {
		vmrs, err := px.GetVmRefsByName(ctx, node.Name)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				continue
			}

			return nil, "", err
		}

		for _, vmr := range vmrs {
			config, err := px.GetVmConfig(ctx, vmr)
			if err != nil {
				return nil, "", err
			}

			if c.GetVMUUID(config) == node.Status.NodeInfo.SystemUUID {
				return vmr, region, nil
			}
		}
	}

	return nil, "", fmt.Errorf("vm '%s' not found", node.Name)
}

// FindVMByName find a VM by name in all Proxmox clusters.
func (c *Cluster) FindVMByName(ctx context.Context, name string) (*pxapi.VmRef, string, error) {
	for region, px := range c.proxmox {
		vmr, err := px.GetVmRefByName(ctx, name)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				continue
			}

			return nil, "", err
		}

		return vmr, region, nil
	}

	return nil, "", fmt.Errorf("vm '%s' not found", name)
}

// FindVMByUUID find a VM by uuid in all Proxmox clusters.
func (c *Cluster) FindVMByUUID(ctx context.Context, uuid string) (*pxapi.VmRef, string, error) {
	for region, px := range c.proxmox {
		vms, err := px.GetResourceList(ctx, "vm")
		if err != nil {
			return nil, "", fmt.Errorf("error get resources %v", err)
		}

		for vmii := range vms {
			vm, ok := vms[vmii].(map[string]interface{})
			if !ok {
				return nil, "", fmt.Errorf("failed to cast response to map, vm: %v", vm)
			}

			if vm["type"].(string) != "qemu" { //nolint:errcheck
				continue
			}

			vmr := pxapi.NewVmRef(int(vm["vmid"].(float64))) //nolint:errcheck
			vmr.SetNode(vm["node"].(string))                 //nolint:errcheck
			vmr.SetVmType("qemu")

			config, err := px.GetVmConfig(ctx, vmr)
			if err != nil {
				return nil, "", err
			}

			if config["smbios1"] != nil {
				if c.getSMBSetting(config, "uuid") == uuid {
					return vmr, region, nil
				}
			}
		}
	}

	return nil, "", fmt.Errorf("vm with uuid '%s' not found", uuid)
}

// GetVMName returns the VM name.
func (c *Cluster) GetVMName(vmInfo map[string]interface{}) string {
	if vmInfo["name"] != nil {
		return vmInfo["name"].(string) //nolint:errcheck
	}

	return ""
}

// GetVMUUID returns the VM UUID.
func (c *Cluster) GetVMUUID(vmInfo map[string]interface{}) string {
	if vmInfo["smbios1"] != nil {
		return c.getSMBSetting(vmInfo, "uuid")
	}

	return ""
}

// GetVMSKU returns the VM instance type name.
func (c *Cluster) GetVMSKU(vmInfo map[string]interface{}) string {
	if vmInfo["smbios1"] != nil {
		return c.getSMBSetting(vmInfo, "sku")
	}

	return ""
}

func (c *Cluster) getSMBSetting(vmInfo map[string]interface{}, name string) string {
	smbios, ok := vmInfo["smbios1"].(string)
	if !ok {
		return ""
	}

	for _, l := range strings.Split(smbios, ",") {
		if l == "" || l == "base64=1" {
			continue
		}

		parsedParameter, err := url.ParseQuery(l)
		if err != nil {
			return ""
		}

		for k, v := range parsedParameter {
			if k == name {
				decodedString, err := base64.StdEncoding.DecodeString(v[0])
				if err != nil {
					decodedString = []byte(v[0])
				}

				return string(decodedString)
			}
		}
	}

	return ""
}
