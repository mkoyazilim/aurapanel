// Package health, sistem sağlık kontrollerini uygular.
// OLS canlılık probu, MariaDB ping, sertifika süre uyarıları.
package health

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Status, tek bir sağlık kontrolünün sonucu.
type Status struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// Report, tüm sağlık kontrollerinin özeti.
type Report struct {
	Overall   bool     `json:"overall"`
	Checks    []Status `json:"checks"`
	CheckedAt string   `json:"checked_at"`
}

// Checker, sağlık kontrol servisi.
type Checker struct {
	st      *store.Store
	mariaDB *sql.DB
	olsAddr string // OLS WebAdmin adresi (genellikle http://127.0.0.1:7080)
}

// NewChecker, Checker oluşturur.
func NewChecker(st *store.Store, mariaDB *sql.DB, olsAddr string) *Checker {
	return &Checker{st: st, mariaDB: mariaDB, olsAddr: olsAddr}
}

// Run, tüm sağlık kontrollerini çalıştırır ve rapor döndürür.
func (c *Checker) Run(ctx context.Context) Report {
	var checks []Status
	all := true

	// 1. SQLite DB bağlantısı
	dbCheck := c.checkDB(ctx)
	if !dbCheck.OK {
		all = false
	}
	checks = append(checks, dbCheck)

	// 2. MariaDB bağlantısı
	if c.mariaDB != nil {
		mariaCheck := c.checkMariaDB(ctx)
		if !mariaCheck.OK {
			all = false
		}
		checks = append(checks, mariaCheck)
	}

	// 3. OLS canlılık probu
	olsCheck := c.checkOLS(ctx)
	if !olsCheck.OK {
		all = false
	}
	checks = append(checks, olsCheck)

	// 4. Sertifika süre uyarıları (30 gün içinde dolacaklar)
	certChecks := c.checkCerts(ctx)
	for _, cc := range certChecks {
		if !cc.OK {
			all = false
		}
		checks = append(checks, cc)
	}

	return Report{
		Overall:   all,
		Checks:    checks,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func (c *Checker) checkDB(ctx context.Context) Status {
	if err := c.st.Ping(ctx); err != nil {
		return Status{Name: "sqlite", OK: false, Message: err.Error()}
	}
	return Status{Name: "sqlite", OK: true}
}

func (c *Checker) checkMariaDB(ctx context.Context) Status {
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := c.mariaDB.PingContext(ctx2); err != nil {
		return Status{Name: "mariadb", OK: false, Message: err.Error()}
	}
	return Status{Name: "mariadb", OK: true}
}

func (c *Checker) checkOLS(ctx context.Context) Status {
	if c.olsAddr == "" {
		return Status{Name: "ols", OK: true, Message: "kontrol devre dışı"}
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx2, http.MethodGet, c.olsAddr, nil)
	if err != nil {
		return Status{Name: "ols", OK: false, Message: err.Error()}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Status{Name: "ols", OK: false, Message: err.Error()}
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return Status{Name: "ols", OK: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return Status{Name: "ols", OK: true}
}

func (c *Checker) checkCerts(ctx context.Context) []Status {
	// 30 gün içinde sürecek sertifikaları bul.
	threshold := time.Now().UTC().Add(30 * 24 * time.Hour).Format("2006-01-02T15:04:05.000Z")
	certs, err := c.st.ListSSLCertsExpiringBefore(ctx, threshold)
	if err != nil {
		return []Status{{Name: "ssl_certs", OK: false, Message: err.Error()}}
	}
	if len(certs) == 0 {
		return []Status{{Name: "ssl_certs", OK: true}}
	}
	var out []Status
	for _, cert := range certs {
		expiry := ""
		if cert.NotAfter.Valid {
			expiry = cert.NotAfter.String
		}
		out = append(out, Status{
			Name:    "ssl_cert:" + cert.SiteID,
			OK:      false,
			Message: fmt.Sprintf("sertifika %s tarihinde sona eriyor", expiry),
		})
	}
	return out
}
