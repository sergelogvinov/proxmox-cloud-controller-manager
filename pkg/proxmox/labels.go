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

const (
	// LabelTopologyRegion is the label used to store the Proxmox region name.
	LabelTopologyRegion = "topology." + Group + "/region"

	// LabelTopologyZone is the label used to store the Proxmox zone name.
	LabelTopologyZone = "topology." + Group + "/zone"

	// LabelTopologyNode is the label used to store the Proxmox node name.
	LabelTopologyNode = "topology." + Group + "/node"

	// LabelTopologyHAGroup is the label used to store the Proxmox HA group name.
	LabelTopologyHAGroup = "topology." + Group + "/ha-group"
)
