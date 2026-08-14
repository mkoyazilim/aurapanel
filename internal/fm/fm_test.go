package fm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

const testHome = "/srv/aurapanel/sites/site001/home"

// fakeFS, kanonik çözümlemeyi sahte haritayla simüle eder (symlink
// politikasının platformdan bağımsız kanıtı için).
type fakeFS struct {
	files map[string]string // yol → kanonik sonuç (symlink'ler dahil)
}

func (f *fakeFS) canon(p string) (string, error) {
	if r, ok := f.files[p]; ok {
		return r, nil
	}
	return "", fmt.Errorf("yok: %s", p)
}

// --- resolve pipeline testleri (FILE_MANAGER §2) ---

func TestResolveTraversalMatrix(t *testing.T) {
	fs := &fakeFS{files: map[string]string{
		testHome:              testHome,
		testHome + "/index.php": testHome + "/index.php",
	}}

	valid := []string{"", ".", "index.php", "wp-content/uploads/x.jpg", "./index.php", "a//b"}
	for _, rel := range valid {
		if _, err := resolve(fs.canon, testHome, rel); err != nil {
			t.Errorf("geçerli yol reddedildi %q: %v", rel, err)
		}
	}

	invalid := []string{
		"../", "../..", "../../etc/passwd", "a/../../etc/passwd",
		"/etc/passwd", "/root/.ssh", "..",
		string([]byte{'a', 0, 'b'}),  // NUL
		string([]byte{'a', 0x01, 'b'}), // kontrol karakteri
	}
	for _, rel := range invalid {
		if _, err := resolve(fs.canon, testHome, rel); err == nil {
			t.Errorf("kaçış girişimi KABUL EDİLDİ: %q", rel)
		}
	}
}

func TestResolveSymlinkPolicy(t *testing.T) {
	// public/link → /etc (DIŞARI): erişim engelli.
	fs := &fakeFS{files: map[string]string{
		testHome:                    testHome,
		testHome + "/public":        testHome + "/public",
		testHome + "/public/link":   "/etc",
		testHome + "/real":          testHome + "/real",
		testHome + "/real/x.txt":    testHome + "/real/x.txt",
	}}
	if _, err := resolve(fs.canon, testHome, "public/link/passwd"); err != ErrOutsideRoot {
		t.Fatalf("dışarı kaçan symlink reddedilmedi: %v", err)
	}
	// Root İÇİNE işaret eden symlink'e izin var.
	fs.files[testHome+"/ok"] = testHome + "/real"
	if _, err := resolve(fs.canon, testHome, "ok/x.txt"); err != nil {
		t.Fatalf("iç symlink reddedildi: %v", err)
	}
}

// Henüz var olmayan hedef (oluşturma senaryosu): en yakın atadan çözülür.
func TestResolveMissingTarget(t *testing.T) {
	fs := &fakeFS{files: map[string]string{
		testHome:             testHome,
		testHome + "/uploads": testHome + "/uploads",
	}}
	got, err := resolve(fs.canon, testHome, "uploads/2026/08/x.jpg")
	if err != nil {
		t.Fatalf("eksik hedef çözümlenemedi: %v", err)
	}
	if got != testHome+"/uploads/2026/08/x.jpg" {
		t.Fatalf("çözüm hatalı: %s", got)
	}
}

// --- Service testleri (LocalBackend) ---

type denyLimiter struct{ deny map[string]bool }

func (d denyLimiter) Allow(action string) bool { return !d.deny[action] }

func testService(t *testing.T) (*FileService, *store.Store, string) {
	t.Helper()
	sitesRoot := filepath.Join(t.TempDir(), "sites")
	home := filepath.Join(sitesRoot, "site001", "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "fm.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	svc := New(NewLocalBackend(), audit.New(st), filepath.ToSlash(sitesRoot))
	return svc, st, filepath.ToSlash(home)
}

func TestServiceCRUD(t *testing.T) {
	svc, _, home := testService(t)
	ctx := context.Background()
	const siteID = "site001"

	// Write → Read roundtrip + hash/mtime dönüşü.
	content := []byte("<?php echo 'merhaba'; ?>")
	if err := svc.Write(ctx, siteID, "index.php", content, "", ""); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, hash, mtime, err := svc.Read(ctx, siteID, "index.php")
	if err != nil || string(got) != string(content) {
		t.Fatalf("Read: %q err=%v", got, err)
	}
	if hash != contentHash(content) || mtime == "" {
		t.Fatalf("doğrulama alanları eksik: hash=%s mtime=%s", hash, mtime)
	}

	// Mkdir + dizin içinde dosya.
	if err := svc.Mkdir(ctx, siteID, "uploads"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Write(ctx, siteID, "uploads/x.txt", []byte("x"), "", ""); err != nil {
		t.Fatal(err)
	}

	// Rename (root içinde).
	if err := svc.Rename(ctx, siteID, "uploads/x.txt", "uploads/y.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(home), "uploads", "y.txt")); err != nil {
		t.Fatal("rename gerçekleşmedi")
	}

	// Copy (dizin, özyinelemeli).
	if err := svc.Copy(ctx, siteID, "uploads", "uploads-copy"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(home), "uploads-copy", "y.txt")); err != nil {
		t.Fatal("dizin kopyalanmadı")
	}

	// List: dizinler önce, sıralı.
	entries, err := svc.List(ctx, siteID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 { // index.php, uploads, uploads-copy
		t.Fatalf("liste: %d giriş", len(entries))
	}
	if !entries[0].IsDir || entries[0].Name != "uploads" {
		t.Fatalf("sıralama: %+v", entries[0])
	}

	// Remove (özyinelemeli).
	if err := svc.Remove(ctx, siteID, "uploads-copy"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(home), "uploads-copy")); !os.IsNotExist(err) {
		t.Fatal("silinmedi")
	}

	// Stat.
	e, err := svc.Stat(ctx, siteID, "index.php")
	if err != nil || e.IsDir || e.Size != int64(len(content)) {
		t.Fatalf("Stat: %+v err=%v", e, err)
	}
}

// Yazma atomik: tmp dosya kalıntısı OLMAMALI (FILE_MANAGER §5.3).
func TestWriteAtomicNoTempLeft(t *testing.T) {
	svc, _, home := testService(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := svc.Write(ctx, "site001", "config.php", []byte("v"), "", ""); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.FromSlash(home))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".aurapanel-tmp-") {
			t.Fatalf("atomik yazım kalıntısı: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Fatalf("beklenmeyen girişler: %d", len(entries))
	}
}

// Optimistic locking: bayat hash/mtime → ErrConflict (FILE_MANAGER §13).
func TestOptimisticLockConflict(t *testing.T) {
	svc, _, _ := testService(t)
	ctx := context.Background()
	const siteID = "site001"

	if err := svc.Write(ctx, siteID, "f.txt", []byte("v1"), "", ""); err != nil {
		t.Fatal(err)
	}
	_, hash, mtime, err := svc.Read(ctx, siteID, "f.txt")
	if err != nil {
		t.Fatal(err)
	}

	// Güncel hash/mtime ile yazma BAŞARILI.
	if err := svc.Write(ctx, siteID, "f.txt", []byte("v2"), hash, mtime); err != nil {
		t.Fatalf("güncel kilit yazılmadı: %v", err)
	}
	// Bayat kilit ile yazma ÇAKIŞMA.
	err = svc.Write(ctx, siteID, "f.txt", []byte("v3"), hash, mtime)
	if err != ErrConflict {
		t.Fatalf("bayat kilit çakışma üretmedi: %v", err)
	}
}

// Rate limit: reddedilen işlem backend'e ULAŞMAMALI.
func TestRateLimitDeny(t *testing.T) {
	svc, _, home := testService(t)
	svc.SetRateLimiter(denyLimiter{deny: map[string]bool{"write": true}})
	ctx := context.Background()

	err := svc.Write(ctx, "site001", "blocked.txt", []byte("x"), "", "")
	if err != ErrRateLimited {
		t.Fatalf("rate limit hatası bekleniyordu: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(home), "blocked.txt")); !os.IsNotExist(err) {
		t.Fatal("reddedilen yazma diske ulaştı")
	}
}

// Site root'un kendisi asla silinemez.
func TestRemoveRootRejected(t *testing.T) {
	svc, _, _ := testService(t)
	for _, rel := range []string{".", ""} {
		if err := svc.Remove(context.Background(), "site001", rel); err == nil {
			t.Fatalf("site root silinebildi: %q", rel)
		}
	}
}

// Symlink hedefi dışarı kaçamaz — çözümleme aşamasında reddedilir
// (gerçek symlink oluşturulmadan, platform bağımsız).
func TestSymlinkCreateExternalTargetRejected(t *testing.T) {
	svc, _, _ := testService(t)
	err := svc.Symlink(context.Background(), "site001", "../../etc", "kacis")
	if err == nil {
		t.Fatal("dış hedefli symlink kabul edildi")
	}
}

// Audit: içerik ASLA kayıtlara yazılmaz (FILE_MANAGER §18).
func TestAuditNeverContainsContent(t *testing.T) {
	svc, st, _ := testService(t)
	ctx := context.Background()
	secret := "GIZLI-ICERIK-9f8e7d6c"
	if err := svc.Write(ctx, "site001", "secret.txt", []byte(secret), "", ""); err != nil {
		t.Fatal(err)
	}
	events, err := st.ListAuditLogs(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if strings.Contains(e.Extra, secret) {
			t.Fatal("AUDIT LOG DOSYA İÇERİĞİ İÇERİYOR")
		}
	}
	// Yazma olayı doğru alanlarla kaydedilmiş olmalı.
	found := false
	for _, e := range events {
		if e.Action == "file.write" && e.Target == "site001" && e.Result == "success" {
			found = true
			if !strings.Contains(e.Extra, "secret.txt") {
				t.Fatalf("audit'te yol yok: %s", e.Extra)
			}
		}
	}
	if !found {
		t.Fatal("file.write audit olayı yok")
	}
}

// DoD (FILE_MANAGER §27): FileService doğrudan os paketi çağırmamalı —
// tüm dosya erişimi Backend üzerinden.
func TestServiceUsesNoDirectOS(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{
		"os.Open(", "os.Create(", "os.Remove(", "os.RemoveAll(",
		"os.WriteFile(", "os.ReadFile(", "os.Mkdir", "os.Rename(", "os.Stat(",
	} {
		if strings.Contains(string(src), banned) {
			t.Errorf("FileService doğrudan %s kullanıyor — Backend üzerinden gitmeli", banned)
		}
	}
}
