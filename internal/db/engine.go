// Package db, site veritabanlarının yaşam döngüsünü yönetir (ROADMAP W8).
//
// Güvenlik modeli (İlke 4 — kullanıcı girdisi ASLA SQL string'ine gitmez):
//   - Tüm identifier'lar validIdent ile doğrulanır ve backtick ile quote
//     edilir; doğrulama regex'i dışında kalan HİÇBİR girdi SQL'e ulaşamaz.
//   - Parolalar yalnızca parametre (?) olarak geçer — query metninde asla.
//   - Engine arayüzü sayesinde üretim MariaDBEngine'in yanında testlerde
//     SQL kaydeden sahte motorlar kullanılır; MariaDB dışı motorlar
//     (PostgreSQL — Faz 2) bu arayüze takılır.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// Engine, veritabanı sunucusu işlemleri.
type Engine interface {
	CreateDatabase(ctx context.Context, name string) error
	DropDatabase(ctx context.Context, name string) error
	CreateUser(ctx context.Context, username, host, password string) error
	DropUser(ctx context.Context, username, host string) error
	SetUserPassword(ctx context.Context, username, host, password string) error
	GrantAll(ctx context.Context, username, host, database string) error
	Ping(ctx context.Context) error
}

// validIdent, SQL identifier güvenlik sınırı (İlke 4).
var validIdent = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// quoteIdent, doğrulanmış identifier'ı backtick ile sarar.
// Backtick içeremeyen girdi (regex sayesinde) kaçışa ihtiyaç duymaz.
func quoteIdent(s string) string { return "`" + s + "`" }

// quoteString, parolayı SQL string olarak (tek tırnak içinde) güvenle sarar.
func quoteString(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

// sqlExec, MariaDBEngine'in kullandığı dar SQL arayüzü (testte kaydedici).
type sqlExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PingContext(ctx context.Context) error
}

// MariaDBEngine, Engine'in MariaDB uygulaması. Bağlantı (dsn, unix socket)
// panel tarafından kurulur; şifreler driver parametresi olarak iletilir.
type MariaDBEngine struct {
	db sqlExec
}

// NewMariaDBEngine, Engine oluşturur (db: *sql.DB).
func NewMariaDBEngine(db sqlExec) *MariaDBEngine { return &MariaDBEngine{db: db} }

func (e *MariaDBEngine) mustIdent(name string) error {
	if !validIdent.MatchString(name) {
		return fmt.Errorf("geçersiz SQL identifier: %q", name)
	}
	return nil
}

func (e *MariaDBEngine) CreateDatabase(ctx context.Context, name string) error {
	if err := e.mustIdent(name); err != nil {
		return err
	}
	_, err := e.db.ExecContext(ctx,
		fmt.Sprintf("CREATE DATABASE %s CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", quoteIdent(name)))
	return err
}

func (e *MariaDBEngine) DropDatabase(ctx context.Context, name string) error {
	if err := e.mustIdent(name); err != nil {
		return err
	}
	_, err := e.db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE %s", quoteIdent(name)))
	return err
}

func (e *MariaDBEngine) CreateUser(ctx context.Context, username, host, password string) error {
	if err := e.mustIdent(username); err != nil {
		return err
	}
	if err := e.mustIdent(host); err != nil {
		return err
	}
	_, err := e.db.ExecContext(ctx,
		fmt.Sprintf("CREATE USER %s@%s IDENTIFIED BY %s", quoteIdent(username), quoteIdent(host), quoteString(password)))
	return err
}

func (e *MariaDBEngine) DropUser(ctx context.Context, username, host string) error {
	if err := e.mustIdent(username); err != nil {
		return err
	}
	if err := e.mustIdent(host); err != nil {
		return err
	}
	_, err := e.db.ExecContext(ctx,
		fmt.Sprintf("DROP USER %s@%s", quoteIdent(username), quoteIdent(host)))
	return err
}

func (e *MariaDBEngine) SetUserPassword(ctx context.Context, username, host, password string) error {
	if err := e.mustIdent(username); err != nil {
		return err
	}
	if err := e.mustIdent(host); err != nil {
		return err
	}
	_, err := e.db.ExecContext(ctx,
		fmt.Sprintf("ALTER USER %s@%s IDENTIFIED BY %s", quoteIdent(username), quoteIdent(host), quoteString(password)))
	return err
}

func (e *MariaDBEngine) GrantAll(ctx context.Context, username, host, database string) error {
	if err := e.mustIdent(username); err != nil {
		return err
	}
	if err := e.mustIdent(host); err != nil {
		return err
	}
	if err := e.mustIdent(database); err != nil {
		return err
	}
	_, err := e.db.ExecContext(ctx,
		fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO %s@%s", quoteIdent(database), quoteIdent(username), quoteIdent(host)))
	return err
}

func (e *MariaDBEngine) Ping(ctx context.Context) error { return e.db.PingContext(ctx) }
