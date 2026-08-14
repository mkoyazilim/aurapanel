package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// WAFRule, site başına özel WAF kuralı.
type WAFRule struct {
	ID          int64  `json:"id"`
	SiteID      string `json:"site_id"`
	RuleID      string `json:"rule_id"`
	Phase       int    `json:"phase"`
	Action      string `json:"action"`
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
}

// WAFRequestLog, WAF kural eşleşme kaydı.
type WAFRequestLog struct {
	ID        int64  `json:"id"`
	SiteID    string `json:"site_id"`
	RuleID    string `json:"rule_id"`
	Action    string `json:"action"`
	ClientIP  string `json:"client_ip"`
	URI       string `json:"uri"`
	Method    string `json:"method"`
	DryRun    bool   `json:"dry_run"`
	CreatedAt string `json:"created_at"`
}

// WAFCRSConfig, global OWASP CRS yapılandırması.
type WAFCRSConfig struct {
	CRSVersion string `json:"crs_version"`
	Paranoia   int    `json:"paranoia"`
	DryRun     bool   `json:"dry_run"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// ─── WAF Rules ───────────────────────────────────────────────────────────────

func (s *Store) ListWAFRules(ctx context.Context, siteID string) ([]WAFRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, rule_id, phase, action, pattern, description, enabled, created_at
		 FROM waf_rules WHERE site_id = ? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list waf_rules: %w", err)
	}
	defer rows.Close()
	var out []WAFRule
	for rows.Next() {
		var r WAFRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.SiteID, &r.RuleID, &r.Phase, &r.Action,
			&r.Pattern, &r.Description, &enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled == 1
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) InsertWAFRule(ctx context.Context, r WAFRule) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO waf_rules (site_id, rule_id, phase, action, pattern, description, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.SiteID, r.RuleID, r.Phase, r.Action, r.Pattern, r.Description, boolInt(r.Enabled))
	if err != nil {
		return 0, fmt.Errorf("insert waf_rule: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) UpdateWAFRule(ctx context.Context, id int64, r WAFRule) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE waf_rules SET phase=?, action=?, pattern=?, description=?, enabled=?
		 WHERE id=? AND site_id=?`,
		r.Phase, r.Action, r.Pattern, r.Description, boolInt(r.Enabled), id, r.SiteID)
	return err
}

func (s *Store) DeleteWAFRule(ctx context.Context, id int64, siteID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM waf_rules WHERE id=? AND site_id=?`, id, siteID)
	return err
}

func (s *Store) GetWAFRule(ctx context.Context, id int64, siteID string) (*WAFRule, error) {
	var r WAFRule
	var enabled int
	err := s.db.QueryRowContext(ctx,
		`SELECT id, site_id, rule_id, phase, action, pattern, description, enabled, created_at
		 FROM waf_rules WHERE id=? AND site_id=?`, id, siteID).
		Scan(&r.ID, &r.SiteID, &r.RuleID, &r.Phase, &r.Action, &r.Pattern, &r.Description, &enabled, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.Enabled = enabled == 1
	return &r, nil
}

// ─── WAF Request Log ─────────────────────────────────────────────────────────

func (s *Store) InsertWAFRequestLog(ctx context.Context, l WAFRequestLog) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO waf_request_log (site_id, rule_id, action, client_ip, uri, method, dry_run)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		l.SiteID, l.RuleID, l.Action, l.ClientIP, l.URI, l.Method, boolInt(l.DryRun))
	// Ring buffer: son 1000 kaydı tut
	s.db.ExecContext(ctx,
		`DELETE FROM waf_request_log WHERE site_id=? AND id NOT IN
		 (SELECT id FROM waf_request_log WHERE site_id=? ORDER BY id DESC LIMIT 1000)`,
		l.SiteID, l.SiteID)
	return err
}

func (s *Store) ListWAFRequestLog(ctx context.Context, siteID string, limit int) ([]WAFRequestLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, rule_id, action, client_ip, uri, method, dry_run, created_at
		 FROM waf_request_log WHERE site_id=? ORDER BY id DESC LIMIT ?`, siteID, limit)
	if err != nil {
		return nil, fmt.Errorf("list waf_request_log: %w", err)
	}
	defer rows.Close()
	var out []WAFRequestLog
	for rows.Next() {
		var l WAFRequestLog
		var dryRun int
		if err := rows.Scan(&l.ID, &l.SiteID, &l.RuleID, &l.Action, &l.ClientIP, &l.URI, &l.Method, &dryRun, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.DryRun = dryRun == 1
		out = append(out, l)
	}
	return out, nil
}

// ─── CRS Config ──────────────────────────────────────────────────────────────

func (s *Store) GetWAFCRSConfig(ctx context.Context) (*WAFCRSConfig, error) {
	var c WAFCRSConfig
	var dryRun int
	var updatedAt sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT crs_version, paranoia, dry_run, updated_at FROM waf_crs_config WHERE id=1`).
		Scan(&c.CRSVersion, &c.Paranoia, &dryRun, &updatedAt)
	if err != nil {
		return nil, err
	}
	c.DryRun = dryRun == 1
	c.UpdatedAt = updatedAt.String
	return &c, nil
}

func (s *Store) UpdateWAFCRSConfig(ctx context.Context, c WAFCRSConfig) error {
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	_, err := s.db.ExecContext(ctx,
		`UPDATE waf_crs_config SET crs_version=?, paranoia=?, dry_run=?, updated_at=? WHERE id=1`,
		c.CRSVersion, c.Paranoia, boolInt(c.DryRun), now)
	return err
}

// boolInt, bool'u SQLite integer'a çevirir.
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
