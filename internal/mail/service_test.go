package mail

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mkoyazilim/aurapanel/internal/store"
	_ "modernc.org/sqlite"
)

// fakePriv, provision çağrısını yakalar; gerçek priv helper'a gidilmez.
type fakePriv struct {
	op   string
	args map[string]any
}

func (f *fakePriv) Call(_ context.Context, op string, args map[string]any) (map[string]any, error) {
	f.op = op
	f.args = args
	return map[string]any{}, nil
}

// TestProvisionSendsDomainsAndAccountsWithHash: provision, DB'deki domain ve
// hesapları (bcrypt hash dahil) priv op'una göndermeli. Regresyon: önceden
// nil payload gönderiliyordu ve /etc/dovecot/users boş yazılıyordu —
// Dovecot "Temporary authentication failure" döndürüyordu.
func TestProvisionSendsDomainsAndAccountsWithHash(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}

	// Fixture: mail_domains.site_id -> sites.id FK'sı için ham satır.
	raw, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO sites (id, name, linux_user, home_dir) VALUES ('mkoyazilim','mkoyazilim.com','www-mkoyazilim','/tmp/test')`); err != nil {
		raw.Close()
		t.Fatalf("sites fixture: %v", err)
	}
	raw.Close()

	ctx := context.Background()
	if err := st.EnsureMailDomain(ctx, "mkoyazilim.com", "mkoyazilim"); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateMailAccount(ctx, store.MailAccount{
		Email:        "info@mkoyazilim.com",
		Domain:       "mkoyazilim.com",
		PasswordHash: "$2a$10$secret",
		QuotaMB:      512,
	}); err != nil {
		t.Fatal(err)
	}
	priv := &fakePriv{}
	svc := NewService(Dependencies{Store: st, Priv: priv})
	if err := svc.provision(ctx); err != nil {
		t.Fatalf("provision: %v", err)
	}
	if priv.op != "mail.provision" {
		t.Fatalf("op = %q, want mail.provision", priv.op)
	}

	b, err := json.Marshal(priv.args)
	if err != nil {
		t.Fatalf("payload marshal: %v", err)
	}
	var got struct {
		Domains  []string `json:"domains"`
		Accounts []struct {
			Email        string `json:"email"`
			PasswordHash string `json:"password_hash"`
			Domain       string `json:"domain"`
			QuotaMB      int    `json:"quota_mb"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}

	if len(got.Domains) != 1 || got.Domains[0] != "mkoyazilim.com" {
		t.Fatalf("domains = %v", got.Domains)
	}
	if len(got.Accounts) != 1 {
		t.Fatalf("accounts = %d, want 1", len(got.Accounts))
	}
	a := got.Accounts[0]
	if a.Email != "info@mkoyazilim.com" || a.Domain != "mkoyazilim.com" || a.QuotaMB != 512 {
		t.Fatalf("account = %+v", a)
	}
	if a.PasswordHash != "$2a$10$secret" {
		t.Fatalf("password_hash = %q; hash payload'a taşınmıyor (json:\"-\" sızıntısı): %s", a.PasswordHash, fmt.Sprintf("%v", got))
	}
}
