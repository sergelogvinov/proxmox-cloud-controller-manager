package proxmox

import (
	"crypto/tls"
	"os"

	pxapi "github.com/Telmate/proxmox-api-go/proxmox"

	clientkubernetes "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type client struct {
	config  *cloudConfig
	proxmox []pxCluster
	kclient clientkubernetes.Interface
}

type pxCluster struct {
	client *pxapi.Client
	region string
}

func newClient(config *cloudConfig) (*client, error) {
	clusters := len(config.Clusters)
	if clusters > 0 {
		proxmox := make([]pxCluster, clusters)

		for idx, cfg := range config.Clusters {
			tlsconf := &tls.Config{InsecureSkipVerify: true}
			if !cfg.Insecure {
				tlsconf = nil
			}

			client, err := pxapi.NewClient(cfg.URL, nil, os.Getenv("PM_HTTP_HEADERS"), tlsconf, "", 600)
			if err != nil {
				return nil, err
			}

			client.SetAPIToken(cfg.TokenID, cfg.TokenSecret)

			if _, err := client.GetVersion(); err != nil {
				klog.Errorf("failed to initialized proxmox client in cluster %s: %v", cfg.Region, err)

				return nil, err
			}

			proxmox[idx] = pxCluster{client: client, region: cfg.Region}
		}

		return &client{
			config:  config,
			proxmox: proxmox,
		}, nil
	}

	return nil, nil
}

func (c *client) GetProxmoxCluster(region string) (*pxCluster, error) {
	for _, px := range c.proxmox {
		if px.region == region {
			return &px, nil
		}
	}

	return nil, nil
}
