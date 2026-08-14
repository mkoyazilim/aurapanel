package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type GitDeployment struct {
	ID             string `json:"id"`
	SiteID         string `json:"site_id"`
	RepoURL        string `json:"repo_url"`
	Branch         string `json:"branch"`
	DeployPath     string `json:"deploy_path"`
	WebhookSecret  string `json:"webhook_secret"`
	DeployScript   string `json:"deploy_script"`
	LastDeployedAt string `json:"last_deployed_at"`
	Status         string `json:"status"` // pending, success, failed
}

func (s *Store) GetGitDeployment(ctx context.Context, siteID string) (*GitDeployment, error) {
	var g GitDeployment
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, repo_url, branch, deploy_path, webhook_secret, deploy_script, COALESCE(last_deployed_at, ''), status 
		 FROM git_deployments WHERE site_id = ?`, siteID).
		Scan(&g.ID, &g.SiteID, &g.RepoURL, &g.Branch, &g.DeployPath, &g.WebhookSecret, &g.DeployScript, &g.LastDeployedAt, &g.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get git deployment: %w", err)
	}
	return &g, nil
}

func (s *Store) GetGitDeploymentBySecret(ctx context.Context, secret string) (*GitDeployment, error) {
	var g GitDeployment
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, repo_url, branch, deploy_path, webhook_secret, deploy_script, COALESCE(last_deployed_at, ''), status 
		 FROM git_deployments WHERE webhook_secret = ?`, secret).
		Scan(&g.ID, &g.SiteID, &g.RepoURL, &g.Branch, &g.DeployPath, &g.WebhookSecret, &g.DeployScript, &g.LastDeployedAt, &g.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get git deployment by secret: %w", err)
	}
	return &g, nil
}

func (s *Store) UpsertGitDeployment(ctx context.Context, g *GitDeployment) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO git_deployments (id, site_id, repo_url, branch, deploy_path, webhook_secret, deploy_script, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(site_id) DO UPDATE SET 
		 repo_url=excluded.repo_url, 
		 branch=excluded.branch, 
		 deploy_path=excluded.deploy_path, 
		 deploy_script=excluded.deploy_script`,
		g.ID, g.SiteID, g.RepoURL, g.Branch, g.DeployPath, g.WebhookSecret, g.DeployScript, g.Status)
	if err != nil {
		return fmt.Errorf("upsert git deployment: %w", err)
	}
	return nil
}

func (s *Store) UpdateGitStatus(ctx context.Context, siteID, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE git_deployments SET status = ?, last_deployed_at = datetime('now') WHERE site_id = ?`,
		status, siteID)
	return err
}

func (s *Store) DeleteGitDeployment(ctx context.Context, siteID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM git_deployments WHERE site_id = ?`, siteID)
	return err
}
