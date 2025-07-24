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
	"fmt"
	"net"
	"strings"
	"unicode"

	v1 "k8s.io/api/core/v1"
)

// ErrorCIDRConflict is the error message formatting string for CIDR conflicts
const ErrorCIDRConflict = "CIDR %s intersects with ignored CIDR %s"

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
		isIgnore := false

		if strings.HasPrefix(item, "!") {
			item = strings.TrimPrefix(item, "!")
			isIgnore = true
		}

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

// HasTaintWithEffect checks if a node has a specific taint with the given key and effect.
// An empty effect string will match any effect for the specified key
func HasTaintWithEffect(node *v1.Node, key, effect string) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == key {
			if effect != "" {
				return string(taint.Effect) == effect
			}

			return true
		}
	}

	return false
}

func checkIPIntersects(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}
