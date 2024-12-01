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
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"
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
				if err := client.Login(cfg.Username, cfg.Password, ""); err != nil {
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
func (c *Cluster) CheckClusters() error {
	for region, client := range c.proxmox {
		if _, err := client.GetVersion(); err != nil {
			return fmt.Errorf("failed to initialized proxmox client in region %s, error: %v", region, err)
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

// FindVMByName find a VM by name in all Proxmox clusters.
func (c *Cluster) FindVMByName(name string) (*pxapi.VmRef, string, error) {
	for region, px := range c.proxmox {
		vmr, err := px.GetVmRefByName(name)
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
func (c *Cluster) FindVMByUUID(uuid string) (*pxapi.VmRef, string, error) {
	for region, px := range c.proxmox {
		vms, err := px.GetResourceList("vm")
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

			config, err := px.GetVmConfig(vmr)
			if err != nil {
				return nil, "", err
			}

			if config["smbios1"] != nil {
				if c.getUUID(config["smbios1"].(string)) == uuid { //nolint:errcheck
					return vmr, region, nil
				}
			}
		}
	}

	return nil, "", fmt.Errorf("vm with uuid '%s' not found", uuid)
}

func (c *Cluster) getUUID(smbios string) string {
	for _, l := range strings.Split(smbios, ",") {
		if l == "" || l == "base64=1" {
			continue
		}

		parsedParameter, err := url.ParseQuery(l)
		if err != nil {
			return ""
		}

		for k, v := range parsedParameter {
			if k == "uuid" {
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
