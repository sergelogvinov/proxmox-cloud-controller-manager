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

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"strings"
	"unicode"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	clientkubernetes "k8s.io/client-go/kubernetes"
	cloudproviderapi "k8s.io/cloud-provider/api"
	cloudnodeutil "k8s.io/cloud-provider/node/helpers"
)

// ErrorCIDRConflict is the error message formatting string for CIDR conflicts
const ErrorCIDRConflict = "CIDR %s intersects with ignored CIDR %s"

var uninitializedTaint = &corev1.Taint{
	Key:    cloudproviderapi.TaintExternalCloudProvider,
	Effect: corev1.TaintEffectNoSchedule,
}

// SplitTrim splits a string of values separated by sep rune into a slice of
// strings with trimmed spaces.
func SplitTrim(s string, sep rune) []string {
	f := func(c rune) bool {
		return unicode.IsSpace(c) || c == sep
	}

	return strings.FieldsFunc(s, f)
}

// ParseCIDRRuleset parses a comma separated list of CIDRs and returns two slices of *net.IPNet, the first being the allow list, the second be the disallow list
func ParseCIDRRuleset(cidrList string) (allowList, ignoreList []*net.IPNet, err error) {
	cidrlist := SplitTrim(cidrList, ',')
	if len(cidrlist) == 0 {
		return []*net.IPNet{}, []*net.IPNet{}, nil
	}

	for _, item := range cidrlist {
		item, isIgnore := strings.CutPrefix(item, "!")

		_, cidr, err := net.ParseCIDR(item)
		if err != nil {
			continue
		}

		if isIgnore {
			ignoreList = append(ignoreList, cidr)

			continue
		}

		allowList = append(allowList, cidr)
	}

	// Check for no interactions
	for _, n1 := range allowList {
		for _, n2 := range ignoreList {
			if checkIPIntersects(n1, n2) {
				return nil, nil, fmt.Errorf(ErrorCIDRConflict, n1.String(), n2.String())
			}
		}
	}

	return ignoreList, allowList, nil
}

// ParseCIDRList parses a comma separated list of CIDRs and returns a slice of *net.IPNet ignoring errors
func ParseCIDRList(cidrList string) []*net.IPNet {
	cidrlist := SplitTrim(cidrList, ',')
	if len(cidrlist) == 0 {
		return []*net.IPNet{}
	}

	cidrs := make([]*net.IPNet, 0, len(cidrlist))

	for _, item := range cidrlist {
		_, cidr, err := net.ParseCIDR(item)
		if err != nil {
			continue
		}

		cidrs = append(cidrs, cidr)
	}

	return cidrs
}

func checkIPIntersects(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

func hasUninitializedTaint(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.MatchTaint(uninitializedTaint) {
			return true
		}
	}

	return false
}

func syncNodeAnnotations(ctx context.Context, kclient clientkubernetes.Interface, node *corev1.Node, nodeAnnotations map[string]string) error {
	nodeAnnotationsOrig := node.ObjectMeta.Annotations
	annotationsToUpdate := map[string]string{}

	for k, v := range nodeAnnotations {
		if r, ok := nodeAnnotationsOrig[k]; !ok || r != v {
			annotationsToUpdate[k] = v
		}
	}

	if len(annotationsToUpdate) > 0 {
		oldData, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("failed to marshal the existing node %#v: %w", node, err)
		}

		newNode := node.DeepCopy()
		if newNode.Annotations == nil {
			newNode.Annotations = make(map[string]string)
		}

		maps.Copy(newNode.Annotations, annotationsToUpdate)

		newData, err := json.Marshal(newNode)
		if err != nil {
			return fmt.Errorf("failed to marshal the new node %#v: %w", newNode, err)
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, &corev1.Node{})
		if err != nil {
			return fmt.Errorf("failed to create a two-way merge patch: %v", err)
		}

		if _, err := kclient.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
			return fmt.Errorf("failed to patch the node: %v", err)
		}
	}

	return nil
}

func syncNodeLabels(c *client, node *corev1.Node, nodeLabels map[string]string) error {
	nodeLabelsOrig := node.ObjectMeta.Labels
	labelsToUpdate := map[string]string{}

	for k, v := range nodeLabels {
		if r, ok := nodeLabelsOrig[k]; !ok || r != v {
			labelsToUpdate[k] = v
		}
	}

	if len(labelsToUpdate) > 0 {
		if !cloudnodeutil.AddOrUpdateLabelsOnNode(c.kclient, labelsToUpdate, node) {
			return fmt.Errorf("failed update labels for node %s", node.Name)
		}
	}

	return nil
}
