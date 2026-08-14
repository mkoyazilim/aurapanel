package ols

import (
	"context"
	"fmt"
	"os"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
)

// PrivInstaller, Installer arayüzünü aurapanel-priv helper üzerinden
// uygular (ARCHITECTURE §3.2 ols.* op'ları).
type PrivInstaller struct {
	client *privclient.Client
}

// NewPrivInstaller, PrivInstaller oluşturur.
func NewPrivInstaller(c *privclient.Client) *PrivInstaller {
	return &PrivInstaller{client: c}
}

func (pi *PrivInstaller) ReadBundle(ctx context.Context, site string) ([]Artifact, error) {
	data, err := pi.client.Call(ctx, "ols.read_bundle", map[string]any{"site": site})
	if err != nil {
		return nil, err
	}
	raw, _ := data["files"].([]any)
	out := make([]Artifact, 0, len(raw))
	for _, f := range raw {
		m, _ := f.(map[string]any)
		name, _ := m["name"].(string)
		content, _ := m["content"].(string)
		mode, _ := m["mode"].(float64) // JSON sayıları float64 gelir
		out = append(out, Artifact{RelPath: name, Content: []byte(content), Mode: os.FileMode(int(mode))})
	}
	return out, nil
}

func (pi *PrivInstaller) InstallBundle(ctx context.Context, site string, files []Artifact) error {
	arr := make([]map[string]any, 0, len(files))
	for _, a := range files {
		arr = append(arr, map[string]any{
			"name":    a.RelPath,
			"content": string(a.Content),
			"mode":    int(a.Mode.Perm()),
		})
	}
	_, err := pi.client.Call(ctx, "ols.install_bundle", map[string]any{"site": site, "files": arr})
	return err
}

func (pi *PrivInstaller) RemoveBundle(ctx context.Context, site string, names []string) error {
	_, err := pi.client.Call(ctx, "ols.remove_bundle", map[string]any{"site": site, "names": names})
	return err
}

func (pi *PrivInstaller) TestConfig(ctx context.Context) error {
	if _, err := pi.client.Call(ctx, "ols.test", nil); err != nil {
		return fmt.Errorf("ols config testi: %w", err)
	}
	return nil
}

func (pi *PrivInstaller) Reload(ctx context.Context) error {
	if _, err := pi.client.Call(ctx, "ols.reload", nil); err != nil {
		return fmt.Errorf("ols reload: %w", err)
	}
	return nil
}
