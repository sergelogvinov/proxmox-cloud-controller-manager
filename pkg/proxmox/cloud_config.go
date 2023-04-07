package proxmox

import (
	"io"

	yaml "gopkg.in/yaml.v3"

	"k8s.io/klog/v2"
)

type cloudConfig struct {
	Global struct {
	} `yaml:"global,omitempty"`
}

func readCloudConfig(config io.Reader) (cloudConfig, error) {
	cfg := cloudConfig{}

	if config != nil {
		if err := yaml.NewDecoder(config).Decode(&cfg); err != nil {
			return cloudConfig{}, err
		}
	}

	klog.V(4).Infof("cloudConfig: %+v", cfg)

	return cfg, nil
}
