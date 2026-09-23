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

// Package provider implements the providerID interface for Proxmox.
package provider

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// ProviderName is the name of the Proxmox provider.
	ProviderName = "proxmox"
)

// IDType identifies which providerID form was parsed.
type IDType string

const (
	// ProviderIDTypeDefault marks a providerID in the "proxmox://region/vmID" form.
	ProviderIDTypeDefault IDType = "default"
	// ProviderIDTypeCapmox marks a providerID in the "proxmox://uuid" form.
	ProviderIDTypeCapmox IDType = "capmox"
)

// ID is the structured representation of a parsed Proxmox providerID.
type ID struct {
	// Type is the form the providerID was parsed from.
	Type IDType
	// VMID is the VM ID, populated when Type is ProviderIDTypeDefault.
	VMID int
	// Region is the cluster/region name, populated when Type is ProviderIDTypeDefault.
	Region string
	// UUID is the VM SMBIOS UUID, populated when Type is ProviderIDTypeCapmox.
	UUID string
}

var (
	providerIDRegexp   = regexp.MustCompile(`^` + ProviderName + `://([^/]*)/([^/]+)$`)
	providerUUIDRegexp = regexp.MustCompile(`^` + ProviderName + `://([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$`)
)

// GetProviderIDFromID returns the magic providerID for kubernetes node.
func GetProviderIDFromID(region string, vmID int) string {
	return fmt.Sprintf("%s://%s/%d", ProviderName, region, vmID)
}

// GetProviderIDFromUUID returns the magic providerID for kubernetes node.
func GetProviderIDFromUUID(uuid string) string {
	return fmt.Sprintf("%s://%s", ProviderName, uuid)
}

// ParseProviderID parses a providerID in either the "proxmox://region/vmID"
// or the "proxmox://uuid" form and returns its structured representation.
func ParseProviderID(providerID string) (*ID, error) {
	if !strings.HasPrefix(providerID, ProviderName) {
		return nil, fmt.Errorf("foreign providerID or empty %q", providerID)
	}

	if matches := providerIDRegexp.FindStringSubmatch(providerID); len(matches) == 3 {
		vmID, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("InstanceID have to be a number, but got %q", matches[2])
		}

		return &ID{
			Type:   ProviderIDTypeDefault,
			VMID:   vmID,
			Region: matches[1],
		}, nil
	}

	if matches := providerUUIDRegexp.FindStringSubmatch(providerID); len(matches) == 2 {
		return &ID{
			Type: ProviderIDTypeCapmox,
			UUID: matches[1],
		}, nil
	}

	return nil, fmt.Errorf("providerID %q didn't match expected format %q or %q", providerID, ProviderName+"://region/InstanceID", ProviderName+"://UUID")
}
