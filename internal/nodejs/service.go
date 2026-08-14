package nodejs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"text/template"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

type Dependencies struct {
	Store *store.Store
	Priv  *privclient.Client
}

type Service struct {
	deps Dependencies
}

func NewService(deps Dependencies) *Service {
	return &Service{deps: deps}
}

const systemdTmpl = `[Unit]
Description=AuraPanel Node App {{ .AppName }} (Site: {{ .SiteID }})
After=network.target

[Service]
Type=simple
User=www-{{ .SiteID }}
Group=www-{{ .SiteID }}
WorkingDirectory=/srv/aurapanel/sites/{{ .SiteID }}/home{{ .AppPath }}
ExecStart=/usr/bin/bash -c "source ~/.nvm/nvm.sh && nvm use {{ .NodeVersion }} && {{ .StartupScript }}"
{{- range $key, $val := .EnvVars }}
Environment={{ $key }}={{ $val }}
{{- end }}
Environment=PORT={{ .Port }}
Restart=on-failure
Slice=system-aurapanel.slice

[Install]
WantedBy=multi-user.target
`

func (s *Service) DeployService(ctx context.Context, app *store.NodeApp) error {
	var env map[string]string
	if app.EnvVars != "" {
		if err := json.Unmarshal([]byte(app.EnvVars), &env); err != nil {
			return fmt.Errorf("env_vars parse hatası: %w", err)
		}
	}

	tmpl, err := template.New("systemd").Parse(systemdTmpl)
	if err != nil {
		return err
	}

	data := map[string]any{
		"SiteID":        app.SiteID,
		"AppName":       app.AppName,
		"AppPath":       path.Clean("/" + app.AppPath),
		"StartupScript": app.StartupScript,
		"NodeVersion":   app.NodeVersion,
		"Port":          app.Port,
		"EnvVars":       env,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}

	req := map[string]any{
		"site":    app.SiteID,
		"app_id":  app.ID,
		"content": buf.String(),
	}

	_, err = s.deps.Priv.Call(ctx, "node.apply", req)
	return err
}

func (s *Service) RemoveService(ctx context.Context, siteID, appID string) error {
	req := map[string]any{
		"site":   siteID,
		"app_id": appID,
	}
	_, err := s.deps.Priv.Call(ctx, "node.remove", req)
	return err
}
