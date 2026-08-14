package php

import (
	"context"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
)

// privImpl, privOps'un priv helper üzerinden üretim uygulaması.
type privImpl struct {
	c *privclient.Client
}

// NewPrivOps, privOps üretim uygulamasını döndürür.
func NewPrivOps(c *privclient.Client) privOps { return &privImpl{c: c} }

func (p *privImpl) DetectPHP(ctx context.Context) (map[string]bool, error) {
	data, err := p.c.Call(ctx, "php.detect", nil)
	if err != nil {
		return nil, err
	}
	raw, _ := data["versions"].(map[string]any)
	out := map[string]bool{}
	for k, v := range raw {
		if b, ok := v.(bool); ok {
			out[k] = b
		}
	}
	return out, nil
}

func (p *privImpl) InstallIni(ctx context.Context, siteID, content string) error {
	_, err := p.c.Call(ctx, "php.install_ini", map[string]any{"site": siteID, "content": content})
	return err
}

func (p *privImpl) ReadIni(ctx context.Context, siteID string) (string, error) {
	data, err := p.c.Call(ctx, "php.read_ini", map[string]any{"site": siteID})
	if err != nil {
		return "", err
	}
	content, _ := data["content"].(string)
	return content, nil
}
