package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mkoyazilim/aurapanel/internal/store"
)

func TestPasswordHashVerify(t *testing.T) {
	hash, err := HashPassword("güçlü-parola-123")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword("güçlü-parola-123", hash)
	if err != nil || !ok {
		t.Fatalf("doğru parola reddedildi: %v", err)
	}
	ok, _ = VerifyPassword("yanlış", hash)
	if ok {
		t.Fatal("yanlış parola kabul edildi")
	}
	if _, err := VerifyPassword("x", "çöp-hash"); err == nil {
		t.Fatal("bozuk hash kabul edildi")
	}
}

func TestSessionLifecycle(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	roleID, _ := st.GetRoleIDByName(ctx, "admin")
	hash, _ := HashPassword("p")
	uid, err := st.InsertUser(ctx, store.User{
		Username: "admin-abc", PasswordHash: hash, RoleID: roleID,
		MustChangePassword: true, Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}

	ss := NewSessionStore(st)
	sid, csrf, err := ss.Create(ctx, uid, "127.0.0.1", "test")
	if err != nil || sid == "" || csrf == "" {
		t.Fatalf("Create: %v %s %s", err, sid, csrf)
	}
	u, err := ss.Validate(ctx, sid)
	if err != nil || u == nil || u.ID != uid {
		t.Fatalf("Validate: %+v %v", u, err)
	}
	gotCSRF, _ := ss.CSRFToken(ctx, sid)
	if gotCSRF != csrf {
		t.Fatal("csrf eşleşmiyor")
	}
	if err := ss.Destroy(ctx, sid); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.Validate(ctx, sid); err == nil {
		t.Fatal("kapatılan oturum geçerli")
	}
}

func TestTOTPFlow(t *testing.T) {
	secret, url, err := GenerateTOTP("AuraPanel", "admin")
	if err != nil || secret == "" || url == "" {
		t.Fatalf("GenerateTOTP: %v %s %s", err, secret, url)
	}
	// Gerçek kod üretimi doğrulanabilir: pquerna üzerinden şimdiki kod.
	ok, err := VerifyTOTP(secret, "000000")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("rastgele kod kabul edildi")
	}
}

func TestPATFlow(t *testing.T) {
	tok := NewPAT()
	if len(tok) != 3+64 {
		t.Fatalf("PAT uzunluğu: %d", len(tok))
	}
	if !VerifyPAT(tok, HashPAT(tok)) {
		t.Fatal("PAT doğrulanamadı")
	}
	if VerifyPAT(tok, HashPAT("ap_başka")) {
		t.Fatal("başka token kabul edildi")
	}
}
