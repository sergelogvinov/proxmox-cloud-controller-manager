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

package cluster

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// ClustersConfig is proxmox multi-cluster cloud config.
type ClustersConfig struct {
	Clusters []struct {
		URL         string `yaml:"url"`
		Insecure    bool   `yaml:"insecure,omitempty"`
		TokenID     string `yaml:"token_id,omitempty"`
		TokenSecret string `yaml:"token_secret,omitempty"`
		Region      string `yaml:"region,omitempty"`
	} `yaml:"clusters,omitempty"`
}

// ReadCloudConfig reads cloud config from a reader.
func ReadCloudConfig(config io.Reader) (ClustersConfig, error) {
	cfg := ClustersConfig{}

	if config != nil {
		if err := yaml.NewDecoder(config).Decode(&cfg); err != nil {
			return ClustersConfig{}, err
		}
	}

	for idx, c := range cfg.Clusters {
		if c.TokenID == "" {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: token_id is required", idx+1)
		}

		if c.TokenSecret == "" {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: token_secret is required", idx+1)
		}

		if c.Region == "" {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: region is required", idx+1)
		}

		if c.URL == "" || !strings.HasPrefix(c.URL, "http") {
			return ClustersConfig{}, fmt.Errorf("cluster #%d: url is required", idx+1)
		}
	}

	return cfg, nil
}

// ReadCloudConfigFromFile reads cloud config from a file.
func ReadCloudConfigFromFile(file string) (ClustersConfig, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return ClustersConfig{}, fmt.Errorf("error reading %s: %v", file, err)
	}
	defer f.Close() // nolint: errcheck

	return ReadCloudConfig(f)
}
