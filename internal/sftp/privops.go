package sftp

import (
	"context"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
)

// PrivOps implements ConfigInstaller using the privileged helper.
type PrivOps struct {
	client *privclient.Client
}

// NewPrivOps creates a new PrivOps instance.
func NewPrivOps(client *privclient.Client) *PrivOps {
	return &PrivOps{client: client}
}

// Install sends the sshd config fragment to the priv helper to be applied.
func (p *PrivOps) Install(ctx context.Context, content string) error {
	_, err := p.client.Call(ctx, "sshd.install_config", map[string]interface{}{
		"content": content,
	})
	return err
}
