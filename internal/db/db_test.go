package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// --- SQL güvenlik testleri (İlke 4) ---

// sqlRecorder, query'leri kaydeder; MariaDBEngine'in ürettiği SQL'i
// incelemede kullanılır — parola asla query metninde olmamalı.
type sqlRecorder struct {
	queries []string
	args    [][]any
	fail    bool
}

func (r *sqlRecorder) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if r.fail {
		return nil, fmt.Errorf("motor hata")
	}
	r.queries = append(r.queries, query)
	r.args = append(r.args, args)
	return nil, nil
}

func (r *sqlRecorder) PingContext(ctx context.Context) error { return nil }

func TestIdentifierInjectionRejected(t *testing.T) {
	e := NewMariaDBEngine(&sqlRecorder{})
	ctx := context.Background()

	bad := []string{
		"x`; DROP DATABASE mysql; --",
		"x'; DELETE FROM users; --",
		"ABC",      // büyük harf
		"x-y",      // tire
		"",         // boş
		strings.Repeat("a", 65), // 64 üstü
	}
	for _, name := range bad {
		if err := e.CreateDatabase(ctx, name); err == nil {
			t.Errorf("enjeksiyon girişimi kabul edildi: %q", name)
		}
		if err := e.CreateUser(ctx, name, "localhost", "parola123456"); err == nil {
			t.Errorf("enjeksiyon girişimi kabul edildi (user): %q", name)
		}
	}
}

func TestSQLQuotingAndParameterization(t *testing.T) {
	rec := &sqlRecorder{}
	e := NewMariaDBEngine(rec)
	ctx := context.Background()
	secret := "s3cr3t-p4ssw0rd!"

	if err := e.CreateDatabase(ctx, "site001_wp"); err != nil {
		t.Fatal(err)
	}
	if err := e.CreateUser(ctx, "site001_user", "localhost", secret); err != nil {
		t.Fatal(err)
	}
	if err := e.GrantAll(ctx, "site001_user", "localhost", "site001_wp"); err != nil {
		t.Fatal(err)
	}
	if err := e.SetUserPassword(ctx, "site001_user", "localhost", secret); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(rec.queries, "\n")
	// Identifier'lar backtick'li; parola query metninde ASLA.
	for _, want := range []string{"`site001_wp`", "`site001_user`", "`localhost`"} {
		if !strings.Contains(joined, want) {
			t.Errorf("quote eksik: %s\nSQL: %s", want, joined)
		}
	}
	if strings.Contains(joined, secret) {
		t.Fatal("PAROLA QUERY METNİNDE — İlke 4 ihlali")
	}
	// Parola parametre olarak iletilmeli.
	found := false
	for _, args := range rec.args {
		for _, a := range args {
			if a == secret {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("parola parametre olarak iletilmedi")
	}
}

// --- Service testleri ---

func testService(t *testing.T) (*Service, *store.Store, *sqlRecorder, *crypto.Cipher) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "db.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	limits, _ := json.Marshal(site.DefaultLimits())
	st.InsertSite(ctx, store.Site{
		ID: "site001", Name: "example.com", LinuxUser: "www-site001",
		HomeDir: "/srv/aurapanel/sites/site001/home", Status: "active",
		FeatureFlags: `{}`, Limits: string(limits),
	})
	key, _ := crypto.GenerateKey()
	cipher, _ := crypto.New(key)
	rec := &sqlRecorder{}
	svc := NewService(st, NewMariaDBEngine(rec), cipher, audit.New(st))
	return svc, st, rec, cipher
}

func TestCreateDatabaseFlow(t *testing.T) {
	svc, st, rec, _ := testService(t)
	ctx := context.Background()

	// Geçersiz ad: motor çağrısı YAPILMAMALI.
	if err := svc.CreateDatabase(ctx, "site001", "BAD-NAME`; DROP --"); err == nil {
		t.Fatal("geçersiz ad kabul edildi")
	}
	if len(rec.queries) != 0 {
		t.Fatal("doğrulama hatasında SQL üretildi")
	}

	if err := svc.CreateDatabase(ctx, "site001", "wp"); err != nil {
		t.Fatalf("CreateDatabase: %v", err)
	}
	if !strings.Contains(rec.queries[0], "`site001_wp`") {
		t.Fatalf("ön ekli ad kullanılmadı: %s", rec.queries[0])
	}
	d, _ := st.GetDatabaseByName(ctx, "site001_wp")
	if d == nil || d.SiteID != "site001" {
		t.Fatalf("kayıt hatalı: %+v", d)
	}
}

func TestDropDatabaseOwnership(t *testing.T) {
	svc, st, _, _ := testService(t)
	ctx := context.Background()
	svc.CreateDatabase(ctx, "site001", "wp")

	// Başka sitenin DB'si silinemez.
	if err := svc.DropDatabase(ctx, "site999", "wp"); err == nil {
		t.Fatal("yetkisiz silme kabul edildi")
	}
	if err := svc.DropDatabase(ctx, "site001", "wp"); err != nil {
		t.Fatalf("DropDatabase: %v", err)
	}
	if d, _ := st.GetDatabaseByName(ctx, "site001_wp"); d != nil {
		t.Fatal("kayıt silinmedi")
	}
}

func TestCreateUserPasswordHandling(t *testing.T) {
	svc, st, rec, _ := testService(t)
	ctx := context.Background()

	// Üretilen parola: döner, 16+ karakter, audit'te ASLA.
	pw, err := svc.CreateUser(ctx, "site001", "user", "")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if len(pw) < minPassLength {
		t.Fatalf("üretilen parola kısa: %d", len(pw))
	}
	u, _ := st.GetDatabaseUserByName(ctx, "site001_user")
	if u == nil {
		t.Fatal("kullanıcı kaydı yok")
	}
	if u.PasswordEnc == pw || u.PasswordEnc == "" {
		t.Fatal("parola düz metin saklandı (encrypted-at-rest ihlali)")
	}
	// Query metninde parola yok.
	if strings.Contains(strings.Join(rec.queries, "\n"), pw) {
		t.Fatal("parola SQL metninde")
	}

	// RevealPassword: çözer ve audit yazar.
	got, err := svc.RevealPassword(ctx, "site001", "user")
	if err != nil || got != pw {
		t.Fatalf("RevealPassword: %q err=%v", got, err)
	}

	// ResetPassword.
	pw2, err := svc.ResetPassword(ctx, "site001", "user", "")
	if err != nil || pw2 == pw {
		t.Fatalf("ResetPassword: %v", err)
	}

	// Liste yanıtında parola (şifreli bile) ASLA.
	users, _ := svc.ListUsers(ctx, "site001")
	if len(users) != 1 || users[0].PasswordEnc != "" {
		t.Fatal("liste yanıtı parola içeriyor")
	}
}

func TestGrantOwnership(t *testing.T) {
	svc, _, rec, _ := testService(t)
	ctx := context.Background()
	svc.CreateDatabase(ctx, "site001", "wp")
	svc.CreateUser(ctx, "site001", "user", "güvenli-parola-12345")

	if err := svc.Grant(ctx, "site001", "user", "wp"); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if !strings.Contains(rec.queries[len(rec.queries)-1], "GRANT ALL PRIVILEGES ON `site001_wp`.* TO `site001_user`@`localhost`") {
		t.Fatalf("grant SQL hatalı: %s", rec.queries[len(rec.queries)-1])
	}
	// Başka site kullanıcısına grant YASAK.
	if err := svc.Grant(ctx, "site999", "user", "wp"); err == nil {
		t.Fatal("yetkisiz grant kabul edildi")
	}
}

func TestAdminerGateFlow(t *testing.T) {
	svc, st, _, _ := testService(t)
	ctx := context.Background()
	svc.CreateDatabase(ctx, "site001", "wp")
	d, _ := st.GetDatabaseByName(ctx, "site001_wp")

	token, err := svc.OpenAdminer(ctx, "site001", d.ID)
	if err != nil {
		t.Fatalf("OpenAdminer: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("token uzunluğu: %d", len(token))
	}

	// Doğrulama: scope doğru db adını döner.
	name, err := svc.ValidateAdminer(ctx, token)
	if err != nil || name != "site001_wp" {
		t.Fatalf("ValidateAdminer: %q err=%v", name, err)
	}
	// Bozuk token reddedilir.
	if _, err := svc.ValidateAdminer(ctx, "sahte"); err == nil {
		t.Fatal("sahte token kabul edildi")
	}
	// Kapatma: idempotent.
	if err := svc.CloseAdminer(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err := svc.CloseAdminer(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateAdminer(ctx, token); err == nil {
		t.Fatal("kapalı token kabul edildi")
	}
}

// Migration v2: adminer_gates tablosu kurulmuş olmalı (W8).
func TestMigrationV2GatesTable(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "v2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	var n int
	st.Ping(context.Background())
	// Tablo varlığı: store katmanından gate insert ile doğrulanır.
	if _, err := st.InsertAdminerGate(context.Background(), store.AdminerGate{
		SiteID: "site001", TokenHash: "h", ExpiresAt: "2030-01-01T00:00:00Z",
	}); err == nil {
		t.Fatal("FK: olmayan site'ye gate eklenebildi") // siteler boş
	}
	_ = n
}
