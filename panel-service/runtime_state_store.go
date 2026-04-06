package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	mysqlcfg "github.com/go-sql-driver/mysql"
)

const (
	defaultRuntimeStatePath      = "/var/lib/aurapanel/panel-service-state.json"
	defaultRuntimeStateBackend   = "auto"
	defaultRuntimeStateMariaDB   = "aurapanel"
	runtimeStateMariaDBTableName = "panel_service_state"
)

type runtimeStateStore interface {
	Name() string
	Load() (persistedRuntimeState, bool, error)
	Save(persistedRuntimeState) error
}

type fileRuntimeStateStore struct {
	path string
}

type mariadbRuntimeStateStore struct {
	mu     sync.Mutex
	db     *sql.DB
	dbName string
}

var sharedMariaDBRuntimeStateStore = &mariadbRuntimeStateStore{}

func runtimeStatePath() string {
	return envOr("AURAPANEL_STATE_FILE", defaultRuntimeStatePath)
}

func runtimeStateBackend() string {
	raw := strings.ToLower(strings.TrimSpace(envOr("AURAPANEL_STATE_BACKEND", "")))
	switch raw {
	case "file", "json":
		return "file"
	case "mariadb", "mysql":
		return "mariadb"
	case "", "auto":
		if strings.TrimSpace(envOr("AURAPANEL_STATE_FILE", "")) != "" {
			return "file"
		}
		return defaultRuntimeStateBackend
	default:
		return defaultRuntimeStateBackend
	}
}

func runtimeStateStores() []runtimeStateStore {
	fileStore := &fileRuntimeStateStore{path: runtimeStatePath()}
	switch runtimeStateBackend() {
	case "file":
		return []runtimeStateStore{fileStore}
	case "mariadb":
		return []runtimeStateStore{sharedMariaDBRuntimeStateStore}
	default:
		return []runtimeStateStore{sharedMariaDBRuntimeStateStore, fileStore}
	}
}

func (s *fileRuntimeStateStore) Name() string {
	return "file"
}

func (s *fileRuntimeStateStore) Load() (persistedRuntimeState, bool, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return persistedRuntimeState{}, false, nil
		}
		return persistedRuntimeState{}, false, err
	}
	var persisted persistedRuntimeState
	if err := json.Unmarshal(raw, &persisted); err != nil {
		return persistedRuntimeState{}, false, fmt.Errorf("decode runtime state: %w", err)
	}
	return persisted, true, nil
}

func (s *fileRuntimeStateStore) Save(payload persistedRuntimeState) error {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tempPath := s.path + ".tmp"
	if err := os.WriteFile(tempPath, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tempPath, s.path)
}

func (s *mariadbRuntimeStateStore) Name() string {
	return "mariadb"
}

func (s *mariadbRuntimeStateStore) Load() (persistedRuntimeState, bool, error) {
	db, err := s.ensureDB()
	if err != nil {
		return persistedRuntimeState{}, false, err
	}
	var payload string
	row := db.QueryRow("SELECT payload FROM " + runtimeStateMariaDBTableName + " WHERE id = 1 LIMIT 1")
	if err := row.Scan(&payload); err != nil {
		if err == sql.ErrNoRows {
			return persistedRuntimeState{}, false, nil
		}
		return persistedRuntimeState{}, false, err
	}
	var persisted persistedRuntimeState
	if err := json.Unmarshal([]byte(payload), &persisted); err != nil {
		return persistedRuntimeState{}, false, fmt.Errorf("decode runtime state: %w", err)
	}
	return persisted, true, nil
}

func (s *mariadbRuntimeStateStore) Save(payload persistedRuntimeState) error {
	db, err := s.ensureDB()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime state: %w", err)
	}
	_, err = db.Exec(
		"INSERT INTO "+runtimeStateMariaDBTableName+" (id, payload) VALUES (1, ?) ON DUPLICATE KEY UPDATE payload = VALUES(payload), updated_at = CURRENT_TIMESTAMP",
		string(raw),
	)
	return err
}

func (s *mariadbRuntimeStateStore) ensureDB() (*sql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dbName := runtimeStateMariaDBName()
	if s.db != nil && s.dbName == dbName {
		if err := s.db.Ping(); err == nil {
			return s.db, nil
		}
		_ = s.db.Close()
		s.db = nil
	}

	if err := ensureMariaRuntimeStateDatabase(dbName); err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", runtimeStateMariaDSN(dbName))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureMariaRuntimeStateTable(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	s.db = db
	s.dbName = dbName
	return s.db, nil
}

func ensureMariaRuntimeStateDatabase(dbName string) error {
	serverDB, err := sql.Open("mysql", runtimeStateMariaDSN(""))
	if err != nil {
		return err
	}
	defer serverDB.Close()
	if err := serverDB.Ping(); err != nil {
		return err
	}
	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", escapeMariaIdentifier(dbName))
	_, err = serverDB.Exec(query)
	return err
}

func ensureMariaRuntimeStateTable(db *sql.DB) error {
	query := "CREATE TABLE IF NOT EXISTS " + runtimeStateMariaDBTableName + " (" +
		"id TINYINT UNSIGNED NOT NULL PRIMARY KEY," +
		"payload LONGTEXT NOT NULL," +
		"updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"
	_, err := db.Exec(query)
	return err
}

func runtimeStateMariaDBName() string {
	name := strings.TrimSpace(envOr("AURAPANEL_STATE_DB_NAME", defaultRuntimeStateMariaDB))
	if name == "" {
		return defaultRuntimeStateMariaDB
	}
	return name
}

func runtimeStateMariaDSN(dbName string) string {
	rawDSN := strings.TrimSpace(envOr("AURAPANEL_STATE_DB_DSN", ""))
	if rawDSN != "" {
		if dbName == "" {
			cfg, err := mysqlcfg.ParseDSN(rawDSN)
			if err != nil {
				return rawDSN
			}
			cfg.DBName = ""
			return cfg.FormatDSN()
		}
		cfg, err := mysqlcfg.ParseDSN(rawDSN)
		if err != nil {
			return rawDSN
		}
		cfg.DBName = dbName
		return cfg.FormatDSN()
	}

	cfg := mysqlcfg.NewConfig()
	cfg.User = firstNonEmpty(strings.TrimSpace(envOr("AURAPANEL_STATE_DB_USER", "")), "root")
	cfg.Passwd = strings.TrimSpace(envOr("AURAPANEL_STATE_DB_PASSWORD", ""))
	host := strings.TrimSpace(envOr("AURAPANEL_STATE_DB_HOST", ""))
	if host == "" {
		cfg.Net = "unix"
		cfg.Addr = runtimeStateMariaSocketPath()
	} else {
		cfg.Net = "tcp"
		port := firstNonEmpty(strings.TrimSpace(envOr("AURAPANEL_STATE_DB_PORT", "")), "3306")
		cfg.Addr = net.JoinHostPort(host, port)
	}
	cfg.DBName = dbName
	cfg.Params = map[string]string{
		"charset": "utf8mb4",
	}
	return cfg.FormatDSN()
}

func runtimeStateMariaSocketPath() string {
	candidates := []string{
		strings.TrimSpace(envOr("AURAPANEL_STATE_DB_SOCKET", "")),
		"/run/mysqld/mysqld.sock",
		"/var/run/mysqld/mysqld.sock",
		"/var/lib/mysql/mysql.sock",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if fileExists(candidate) {
			return candidate
		}
	}
	return "/run/mysqld/mysqld.sock"
}

func escapeMariaIdentifier(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "`", "``")
}
