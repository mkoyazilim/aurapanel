package store

import (
	"context"
)

type StagingEnvironment struct {
	ID               string `json:"id"`
	ProductionSiteID string `json:"production_site_id"`
	StagingSiteID    string `json:"staging_site_id"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
}

func (s *Store) ListStagingEnvironments(ctx context.Context, prodSiteID string) ([]StagingEnvironment, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, production_site_id, staging_site_id, status, created_at FROM staging_environments WHERE production_site_id = ?",
		prodSiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []StagingEnvironment
	for rows.Next() {
		var e StagingEnvironment
		if err := rows.Scan(&e.ID, &e.ProductionSiteID, &e.StagingSiteID, &e.Status, &e.CreatedAt); err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}
	if envs == nil {
		envs = []StagingEnvironment{}
	}
	return envs, nil
}

func (s *Store) InsertStagingEnvironment(ctx context.Context, env *StagingEnvironment) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO staging_environments (id, production_site_id, staging_site_id, status) VALUES (?, ?, ?, ?)",
		env.ID, env.ProductionSiteID, env.StagingSiteID, env.Status)
	return err
}

func (s *Store) GetStagingEnvironment(ctx context.Context, id string) (*StagingEnvironment, error) {
	var e StagingEnvironment
	err := s.db.QueryRowContext(ctx,
		"SELECT id, production_site_id, staging_site_id, status, created_at FROM staging_environments WHERE id = ?",
		id).Scan(&e.ID, &e.ProductionSiteID, &e.StagingSiteID, &e.Status, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *Store) GetActiveStagingEnvironment(ctx context.Context, prodSiteID string) (*StagingEnvironment, error) {
	var e StagingEnvironment
	err := s.db.QueryRowContext(ctx,
		"SELECT id, production_site_id, staging_site_id, status, created_at FROM staging_environments WHERE production_site_id = ? AND status = 'active' LIMIT 1",
		prodSiteID).Scan(&e.ID, &e.ProductionSiteID, &e.StagingSiteID, &e.Status, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
