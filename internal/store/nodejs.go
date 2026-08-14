package store

import (
	"context"
	"fmt"
)

type NodeApp struct {
	ID            string `json:"id"`
	SiteID        string `json:"site_id"`
	AppName       string `json:"app_name"`
	AppPath       string `json:"app_path"`
	StartupScript string `json:"startup_script"`
	Port          int    `json:"port"`
	NodeVersion   string `json:"node_version"`
	EnvVars       string `json:"env_vars"` // JSON encoded
	Status        string `json:"status"`
}

func (s *Store) ListNodeApps(ctx context.Context, siteID string) ([]NodeApp, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, app_name, app_path, startup_script, port, node_version, env_vars, status 
		 FROM node_apps WHERE site_id = ? ORDER BY app_name`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list node apps: %w", err)
	}
	defer rows.Close()

	var apps []NodeApp
	for rows.Next() {
		var a NodeApp
		if err := rows.Scan(&a.ID, &a.SiteID, &a.AppName, &a.AppPath, &a.StartupScript, &a.Port, &a.NodeVersion, &a.EnvVars, &a.Status); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func (s *Store) InsertNodeApp(ctx context.Context, a *NodeApp) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO node_apps (id, site_id, app_name, app_path, startup_script, port, node_version, env_vars, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.SiteID, a.AppName, a.AppPath, a.StartupScript, a.Port, a.NodeVersion, a.EnvVars, a.Status)
	if err != nil {
		return fmt.Errorf("insert node app: %w", err)
	}
	return nil
}

func (s *Store) UpdateNodeAppStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE node_apps SET status = ? WHERE id = ?`, status, id)
	return err
}

func (s *Store) DeleteNodeApp(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM node_apps WHERE id = ?`, id)
	return err
}
