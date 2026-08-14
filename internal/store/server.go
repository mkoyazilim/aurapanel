package store

import (
	"context"
	"fmt"
)

// Server, cluster içindeki bir düğümü temsil eder.
type Server struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	APIKey    string `json:"api_key"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

var serverColumns = "id, name, ip_address, api_key, status, created_at, updated_at"

// ListServers, cluster'daki tüm sunucuları getirir.
func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+serverColumns+` FROM servers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("server list: %w", err)
	}
	defer rows.Close()

	var out []Server
	for rows.Next() {
		var srv Server
		if err := rows.Scan(&srv.ID, &srv.Name, &srv.IPAddress, &srv.APIKey, &srv.Status, &srv.CreatedAt, &srv.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, srv)
	}
	return out, nil
}

// GetServer, belirli bir sunucuyu ID ile getirir.
func (s *Store) GetServer(ctx context.Context, id string) (*Server, error) {
	var srv Server
	if err := s.db.QueryRowContext(ctx, `SELECT `+serverColumns+` FROM servers WHERE id = ?`, id).Scan(
		&srv.ID, &srv.Name, &srv.IPAddress, &srv.APIKey, &srv.Status, &srv.CreatedAt, &srv.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("server get: %w", err)
	}
	return &srv, nil
}

// InsertServer, yeni bir sunucu ekler.
func (s *Store) InsertServer(ctx context.Context, srv Server) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO servers (id, name, ip_address, api_key, status) VALUES (?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.IPAddress, srv.APIKey, srv.Status)
	return err
}

// DeleteServer, bir sunucuyu siler.
func (s *Store) DeleteServer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	return err
}
