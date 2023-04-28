package cluster

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

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

	return cfg, nil
}

// ReadCloudConfigFromFile reads cloud config from a file.
func ReadCloudConfigFromFile(file string) (ClustersConfig, error) {
	f, err := os.Open(filepath.Clean(file))
	if err != nil {
		return ClustersConfig{}, fmt.Errorf("error reading %s: %v", file, err)
	}
	defer f.Close() // nolint: errcheck

	cfg := ClustersConfig{}

	if f != nil {
		if err := yaml.NewDecoder(f).Decode(&cfg); err != nil {
			return ClustersConfig{}, err
		}
	}

	return cfg, nil
}
