package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// fakeFetcher, URL → içerik eşlemesi.
type fakeFetcher struct {
	files map[string][]byte
	fail  map[string]bool
}

func (f *fakeFetcher) Get(ctx context.Context, url string) ([]byte, error) {
	if f.fail[url] {
		return nil, fmt.Errorf("indirme hatası")
	}
	b, ok := f.files[url]
	if !ok {
		return nil, fmt.Errorf("404")
	}
	return b, nil
}

func testManifest(t *testing.T, panelVersion, binaryURL string, binary []byte) (string, *fakeFetcher) {
	t.Helper()
	sum := sha256.Sum256(binary)
	m := Manifest{
		Panel: panelVersion,
		Components: map[string]Component{
			"panel": {Version: panelVersion, SHA256: hex.EncodeToString(sum[:]), URL: binaryURL},
		},
		TestedAt: "2026-08-14",
	}
	b, _ := json.Marshal(m)
	f := &fakeFetcher{files: map[string][]byte{
		"https://example.com/versions.json": b,
		binaryURL:                           binary,
	}}
	return "https://example.com/versions.json", f
}

func TestCheck(t *testing.T) {
	manifestURL, f := testManifest(t, "0.2.0", "https://example.com/bin", []byte("binary"))
	svc := NewService(f, manifestURL, "0.1.0", "/tmp/aurapanel")

	out, err := svc.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if out["update"] != true || out["latest"] != "0.2.0" || out["current"] != "0.1.0" {
		t.Fatalf("durum hatalı: %+v", out)
	}
}

func TestSelfUpdateHappy(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "aurapanel")
	os.WriteFile(target, []byte("eski"), 0o755)

	newBinary := []byte("yeni-binary-icerik")
	manifestURL, f := testManifest(t, "0.2.0", "https://example.com/bin", newBinary)
	svc := NewService(f, manifestURL, "0.1.0", target)

	v, err := svc.SelfUpdate(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != "0.2.0" {
		t.Fatalf("sürüm: %s", v)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(newBinary) {
		t.Fatal("binary değişmedi")
	}
	// tmp kalıntısı olmamalı (atomik).
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "aurapanel" {
			t.Fatalf("kalıntı: %s", e.Name())
		}
	}
}

func TestSelfUpdateRejectsBadSHA(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "aurapanel")
	os.WriteFile(target, []byte("eski"), 0o755)

	manifestURL, f := testManifest(t, "0.2.0", "https://example.com/bin", []byte("gerçek"))
	// Manifestteki SHA'yı boz: içerik değiştirilmiş gibi.
	b, _ := f.Get(context.Background(), manifestURL)
	var m Manifest
	json.Unmarshal(b, &m)
	m.Components["panel"] = Component{Version: "0.2.0", SHA256: "00" + m.Components["panel"].SHA256[2:], URL: "https://example.com/bin"}
	nb, _ := json.Marshal(m)
	f.files[manifestURL] = nb

	svc := NewService(f, manifestURL, "0.1.0", target)
	if _, err := svc.SelfUpdate(context.Background()); err == nil {
		t.Fatal("bozuk SHA kabul edildi")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "eski" {
		t.Fatal("başarısız güncellemede eski binary korunmadı")
	}
}

func TestSelfUpdateNoop(t *testing.T) {
	manifestURL, f := testManifest(t, "0.1.0", "https://example.com/bin", []byte("x"))
	svc := NewService(f, manifestURL, "0.1.0", "/tmp/x")
	if _, err := svc.SelfUpdate(context.Background()); err == nil {
		t.Fatal("güncel sürümde güncelleme denenmemeli")
	}
}

func TestParseManifestRejectsEmpty(t *testing.T) {
	if _, err := parseManifest([]byte(`{"components":{}}`)); err == nil {
		t.Fatal("sürümsüz manifest kabul edildi")
	}
	if _, err := parseManifest([]byte("çöp")); err == nil {
		t.Fatal("bozuk manifest kabul edildi")
	}
}
