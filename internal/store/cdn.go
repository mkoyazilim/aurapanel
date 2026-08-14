package store

import (
	"context"
	"fmt"
)

// CDNStat, tek bir hit/miss istatistik kaydı.
type CDNStat struct {
	ID         int64  `json:"id"`
	SiteID     string `json:"site_id"`
	Source     string `json:"source"`
	Hits       int64  `json:"hits"`
	Misses     int64  `json:"misses"`
	Purges     int64  `json:"purges"`
	RecordedAt string `json:"recorded_at"`
}

// CDNCacheRule, site başına CF cache kuralı.
type CDNCacheRule struct {
	ID         int64  `json:"id"`
	SiteID     string `json:"site_id"`
	Pattern    string `json:"pattern"`
	CacheLevel string `json:"cache_level"`
	TTL        int    `json:"ttl"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
}

// ─── CDN Stats ───────────────────────────────────────────────────────────────

func (s *Store) InsertCDNStat(ctx context.Context, st CDNStat) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cdn_stats (site_id, source, hits, misses, purges) VALUES (?, ?, ?, ?, ?)`,
		st.SiteID, st.Source, st.Hits, st.Misses, st.Purges)
	return err
}

func (s *Store) ListCDNStats(ctx context.Context, siteID string, limit int) ([]CDNStat, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, source, hits, misses, purges, recorded_at
		 FROM cdn_stats WHERE site_id=? ORDER BY id DESC LIMIT ?`, siteID, limit)
	if err != nil {
		return nil, fmt.Errorf("list cdn_stats: %w", err)
	}
	defer rows.Close()
	var out []CDNStat
	for rows.Next() {
		var st CDNStat
		if err := rows.Scan(&st.ID, &st.SiteID, &st.Source, &st.Hits, &st.Misses, &st.Purges, &st.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, nil
}

// CDNStatSummary, toplam hit/miss/purge özeti döndürür.
func (s *Store) CDNStatSummary(ctx context.Context, siteID string) (hits, misses, purges int64, err error) {
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(hits),0), COALESCE(SUM(misses),0), COALESCE(SUM(purges),0)
		 FROM cdn_stats WHERE site_id=?`, siteID).Scan(&hits, &misses, &purges)
	return
}

// ─── CDN Cache Rules ─────────────────────────────────────────────────────────

func (s *Store) ListCDNCacheRules(ctx context.Context, siteID string) ([]CDNCacheRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, site_id, pattern, cache_level, ttl, enabled, created_at
		 FROM cdn_cache_rules WHERE site_id=? ORDER BY id`, siteID)
	if err != nil {
		return nil, fmt.Errorf("list cdn_cache_rules: %w", err)
	}
	defer rows.Close()
	var out []CDNCacheRule
	for rows.Next() {
		var r CDNCacheRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.SiteID, &r.Pattern, &r.CacheLevel, &r.TTL, &enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Enabled = enabled == 1
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) InsertCDNCacheRule(ctx context.Context, r CDNCacheRule) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO cdn_cache_rules (site_id, pattern, cache_level, ttl, enabled) VALUES (?, ?, ?, ?, ?)`,
		r.SiteID, r.Pattern, r.CacheLevel, r.TTL, boolInt(r.Enabled))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateCDNCacheRule(ctx context.Context, id int64, r CDNCacheRule) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE cdn_cache_rules SET pattern=?, cache_level=?, ttl=?, enabled=? WHERE id=? AND site_id=?`,
		r.Pattern, r.CacheLevel, r.TTL, boolInt(r.Enabled), id, r.SiteID)
	return err
}

func (s *Store) DeleteCDNCacheRule(ctx context.Context, id int64, siteID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cdn_cache_rules WHERE id=? AND site_id=?`, id, siteID)
	return err
}
