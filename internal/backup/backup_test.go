package backup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// fakeFiles, tar.gz streamini taklit eder (deterministik içerik).
type fakeFiles struct{ content string }

func (f *fakeFiles) StreamTarGz(ctx context.Context, siteID string, rels, abses []string, w io.Writer) error {
	_, err := io.WriteString(w, "TARGZ-"+f.content+"-"+siteID)
	return err
}

// fakeDumps, db dökümlerini kaydeder.
type fakeDumps struct{ dumped []string }

func (f *fakeDumps) DumpDatabase(ctx context.Context, dbName string, w io.Writer) error {
	f.dumped = append(f.dumped, dbName)
	_, err := io.WriteString(w, "SQLDUMP-"+dbName+"\n")
	return err
}

func testBackupService(t *testing.T) (*Service, *store.Store, *fakeDumps, string) {
	return newTestService(t, &fakeFiles{content: "VERI"})
}

func newTestService(t *testing.T, files FilesSource) (*Service, *store.Store, *fakeDumps, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "backup.db"))
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
	st.InsertDatabase(ctx, store.Database{SiteID: "site001", Name: "site001_wp", Charset: "utf8mb4"})

	key := make([]byte, 32)
	rand.Read(key)
	dir := filepath.Join(t.TempDir(), "storage")
	dumps := &fakeDumps{}
	svc, err := NewService(st, NewLocalStorage(dir), key, files, dumps, audit.New(st), 2)
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, dumps, dir
}

func TestBackupFullRoundtrip(t *testing.T) {
	svc, st, dumps, dir := testBackupService(t)
	ctx := context.Background()

	name, err := svc.Run(ctx, "site001", "full")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(dumps.dumped) != 1 || dumps.dumped[0] != "site001_wp" {
		t.Fatalf("db dökümü alınmadı: %v", dumps.dumped)
	}

	// Depoda şifreli dosya var; düz metin İÇERMEMELİ (encrypt-then-upload).
	rc, err := NewLocalStorage(dir).Open(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(rc)
	rc.Close()
	if bytes.Contains(raw, []byte("VERI")) {
		t.Fatal("depodaki dosya DÜZ METİN içeriyor (şifreleme yok)")
	}
	if string(raw[:5]) != string(magic) {
		t.Fatal("yedek imzası yok")
	}

	// Restore: çözülmüş içerik doğrulanır.
	rc2, _ := NewLocalStorage(dir).Open(ctx, name)
	dr, err := DecryptReader(svc.key, rc2)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := io.ReadAll(dr)
	rc2.Close()
	if !bytes.Contains(plain, []byte("VERI")) && !bytes.Contains(plain, []byte("SQLDUMP")) {
		t.Fatalf("çözülmüş içerik bozuk: %q", plain)
	}

	// DB kaydı success.
	backups, _ := st.ListBackupsBySite(ctx, "site001")
	if len(backups) != 1 || backups[0].Status != "success" || backups[0].Encrypted != 1 {
		t.Fatalf("kayıt hatalı: %+v", backups)
	}
}

func TestBackupRetention(t *testing.T) {
	svc, st, _, dir := testBackupService(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if _, err := svc.Run(ctx, "site001", "files"); err != nil {
			t.Fatal(err)
		}
	}
	backups, _ := st.ListBackupsBySite(ctx, "site001")
	if len(backups) != 2 {
		t.Fatalf("retention=2 bekleniyordu, %d kaldı", len(backups))
	}
	// Depodaki dosya sayısı da 2 olmalı.
	list, _ := NewLocalStorage(dir).List(ctx)
	if len(list) != 2 {
		t.Fatalf("depo %d dosya içeriyor", len(list))
	}
}

func TestDecryptRejectsTampering(t *testing.T) {
	svc, _, _, dir := testBackupService(t)
	ctx := context.Background()
	name, err := svc.Run(ctx, "site001", "files")
	if err != nil {
		t.Fatal(err)
	}

	// Dosyanın ortasındaki bir baytı boz → çözme BAŞARISIZ.
	path := filepath.Join(dir, name)
	b, _ := os.ReadFile(path)
	b[len(b)/2] ^= 0xFF
	os.WriteFile(path, b, 0o600)

	rc, _ := NewLocalStorage(dir).Open(ctx, name)
	dr, err := DecryptReader(svc.key, rc)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(dr)
	rc.Close()
	if err == nil {
		t.Fatal("kurcalanmış yedek çözüldü")
	}
}

func TestBackupBadKindRejected(t *testing.T) {
	svc, _, _, _ := testBackupService(t)
	if _, err := svc.Run(context.Background(), "site001", "sistem"); err == nil {
		t.Fatal("geçersiz tür kabul edildi")
	}
}

// blockingFiles, StreamTarGz'ı release kapatılana kadar bloklar — eşzamanlılık
// testleri için deterministik asma sağlar.
type blockingFiles struct {
	started chan struct{}
	release chan struct{}
}

func (f *blockingFiles) StreamTarGz(ctx context.Context, siteID string, rels, abses []string, w io.Writer) error {
	close(f.started)
	<-f.release
	_, err := io.WriteString(w, "BLOCKED-"+siteID)
	return err
}

// waitForBackup, sitenin son yedek kaydı running dışında bir duruma geçene
// kadar bekler (5 sn tavan).
func waitForBackup(t *testing.T, st *store.Store, siteID string) store.Backup {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		list, err := st.ListBackupsBySite(ctx, siteID)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) > 0 && list[len(list)-1].Status != "running" {
			return list[len(list)-1]
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("yedek zamanında tamamlanmadı")
	return store.Backup{}
}

// TestBackupRunAsyncCompletes, arka plan yedeğinin kayda gerçek boyutla
// success olarak işlendiğini doğrular.
func TestBackupRunAsyncCompletes(t *testing.T) {
	svc, st, dumps, _ := testBackupService(t)
	ctx := context.Background()

	id, name, err := svc.RunAsync(ctx, "site001", "full", "local")
	if err != nil {
		t.Fatalf("RunAsync: %v", err)
	}
	if id <= 0 || name == "" {
		t.Fatalf("RunAsync geçersiz kayıt döndü: %d %q", id, name)
	}
	b := waitForBackup(t, st, "site001")
	if b.Status != "success" {
		t.Fatalf("durum success bekleniyordu: %s", b.Status)
	}
	if b.SizeBytes <= 0 {
		t.Fatalf("gerçek depo boyutu kaydedilmedi: %d", b.SizeBytes)
	}
	if len(dumps.dumped) != 1 || dumps.dumped[0] != "site001_wp" {
		t.Fatalf("db dökümü alınmadı: %v", dumps.dumped)
	}
}

// TestBackupRejectsConcurrent, aynı site için ikinci yedeğin reddedildiğini
// (hem senkron hem async yoldan) doğrular.
func TestBackupRejectsConcurrent(t *testing.T) {
	bf := &blockingFiles{started: make(chan struct{}), release: make(chan struct{})}
	svc, st, _, _ := newTestService(t, bf)
	ctx := context.Background()

	if _, _, err := svc.RunAsync(ctx, "site001", "files", "local"); err != nil {
		t.Fatalf("RunAsync: %v", err)
	}
	<-bf.started // pipeline gerçekten çalışıyor (kayıt running)

	if _, err := svc.Run(ctx, "site001", "files"); !errors.Is(err, ErrBackupRunning) {
		t.Fatalf("senkron eşzamanlı yedek reddedilmedi: %v", err)
	}
	if _, _, err := svc.RunAsync(ctx, "site001", "files", "local"); !errors.Is(err, ErrBackupRunning) {
		t.Fatalf("async eşzamanlı yedek reddedilmedi: %v", err)
	}

	close(bf.release)
	if b := waitForBackup(t, st, "site001"); b.Status != "success" {
		t.Fatalf("ilk yedek başarıyla bitmedi: %s", b.Status)
	}
}
