package proxmox

import (
	"context"

	clientkubernetes "k8s.io/client-go/kubernetes"
)

type client struct {
	config  *cloudConfig
	kclient clientkubernetes.Interface
}

func newClient(ctx context.Context, config *cloudConfig) (*client, error) {
	return &client{
		config: config,
	}, nil
}
