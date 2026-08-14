package fm

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Upload testleri ---

func TestUploadChunkedFlow(t *testing.T) {
	svc, _, _ := testService(t)
	up := NewUploadService(svc, filepath.Join(t.TempDir(), "staging"))
	ctx := context.Background()

	content := bytes.Repeat([]byte("x"), 1024*1024+123) // 1 MiB'den fazla
	id, err := up.Init("site001", "", "big.bin", int64(len(content)))
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	// 64 KiB'lik parçalara böl (chunk boyutu sınırını aşmamak için
	// MaxChunkSize'ı küçült).
	up.SetPolicy(UploadPolicy{MaxFileSize: 10 << 20, MaxChunkSize: 64 << 10})

	// Parçaları yükle (son parça tekrar denenerek resume kanıtlanır).
	chunk := 64 << 10
	for i := 0; i < len(content); i += chunk {
		end := i + chunk
		if end > len(content) {
			end = len(content)
		}
		if _, err := up.Chunk(id, i/chunk, content[i:end], ""); err != nil {
			t.Fatalf("Chunk %d: %v", i/chunk, err)
		}
	}
	// Resume: aynı parçayı tekrar gönder — hata OLMAMALI.
	if _, err := up.Chunk(id, 0, content[:chunk], ""); err != nil {
		t.Fatalf("resume chunk: %v", err)
	}

	target, err := up.Finalize(ctx, id)
	if err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if target != "big.bin" {
		t.Fatalf("hedef: %s", target)
	}
	got, _, _, err := svc.Read(ctx, "site001", "big.bin")
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("finalize içeriği bozuk: %d bayt, err=%v", len(got), err)
	}
}

func TestUploadRejects(t *testing.T) {
	svc, _, _ := testService(t)
	up := NewUploadService(svc, filepath.Join(t.TempDir(), "staging"))
	ctx := context.Background()

	// Boyut sınırı.
	if _, err := up.Init("site001", "", "big.bin", 3<<30); err == nil {
		t.Fatal("2 GB sınırı aşıldı")
	}
	// Kötü dosya adı (ayraç/traversal).
	for _, name := range []string{"../evil", "a/b", "..", "a\x00b"} {
		if _, err := up.Init("site001", "", name, 10); err == nil {
			t.Errorf("kötü dosya adı kabul edildi: %q", name)
		}
	}
	// Bozuk chunk hash'i reddedilir.
	id, err := up.Init("site001", "", "ok.bin", 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := up.Chunk(id, 0, []byte("1234567890"), "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("yanlış chunk hash'i kabul edildi")
	}
	// Eksik parça ile finalize reddedilir.
	if _, err := up.Finalize(ctx, id); err == nil {
		t.Fatal("eksik parçalı finalize kabul edildi")
	}
	// Abort sonrası oturum yok.
	if err := up.Abort(id); err != nil {
		t.Fatal(err)
	}
	if err := up.Abort(id); err == nil {
		t.Fatal("iptal edilen oturum ikinci kez iptal edildi")
	}
}

// --- Archive testleri ---

func TestArchiveCreateExtractRoundtrip(t *testing.T) {
	svc, _, _ := testService(t)
	as := NewArchiveService(svc)
	ctx := context.Background()

	// Kaynak dosyalar.
	svc.Write(ctx, "site001", "www/index.html", []byte("<h1>a</h1>"), "", "")
	svc.Write(ctx, "site001", "www/css/style.css", []byte("body{}"), "", "")

	for _, format := range []string{"zip", "targz"} {
		if err := as.Create(ctx, "site001", "backup."+format, format, []string{"www"}); err != nil {
			t.Fatalf("Create %s: %v", format, err)
		}
		// Çıkar: www-ext dizinine.
		if err := as.Extract(ctx, "site001", "backup."+format, "www-ext", format); err != nil {
			t.Fatalf("Extract %s: %v", format, err)
		}
		// Standart arşiv davranışı: kaynak dizin yapısı korunur.
		got, _, _, err := svc.Read(ctx, "site001", "www-ext/www/index.html")
		if err != nil || string(got) != "<h1>a</h1>" {
			t.Fatalf("%s roundtrip bozuk: %q err=%v", format, got, err)
		}
	}
}

// Zip-slip saldırısı: dışarı kaçan girdi REDDEDİLMELİ.
func TestExtractZipSlipRejected(t *testing.T) {
	svc, _, home := testService(t)
	as := NewArchiveService(svc)
	ctx := context.Background()

	// Kötü niyetli zip'i staging'de elle kurarız (site root dışında).
	staging := filepath.Join(t.TempDir(), "evil.zip")
	f, _ := os.Create(staging)
	zw := zip.NewWriter(f)
	for _, name := range []string{"../../etc/evil.txt", "/etc/passwd", "a/../../../root/.ssh/id_rsa"} {
		w, _ := zw.Create(name)
		w.Write([]byte("saldırı"))
	}
	zw.Close()
	f.Close()

	// Arşivi siteye koy (okuma pipeline'ından geçer).
	data, _ := os.ReadFile(staging)
	svc.Write(ctx, "site001", "evil.zip", data, "", "")

	if err := as.Extract(ctx, "site001", "evil.zip", "out", "zip"); err == nil {
		t.Fatal("zip-slip girdisi KABUL EDİLDİ")
	}
	// Dışarı dosya yazılmadığını doğrula.
	for _, p := range []string{"../../etc/evil.txt", "../../../root/.ssh/id_rsa"} {
		if _, err := os.Stat(filepath.Join(filepath.FromSlash(home), filepath.FromSlash(p))); err == nil {
			t.Fatalf("dışarı yazıldı: %s", p)
		}
	}
}

// Archive bomb: küçük arşivden dev çıktı oran sınırıyla durdurulmalı.
func TestExtractArchiveBombRatio(t *testing.T) {
	svc, _, _ := testService(t)
	as := NewArchiveService(svc)
	as.SetLimits(ArchiveLimits{MaxTotalSize: 1 << 30, MaxFiles: 100, MaxRatio: 2, MaxEntryPath: 4096})
	ctx := context.Background()

	// 1 KB sıkıştırılmış, ~100 KB açılan (yüksek oran) zip üret.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("big.txt")
	w.Write(bytes.Repeat([]byte("A"), 100*1024)) // zip'te iyi sıkışır
	zw.Close()

	svc.Write(ctx, "site001", "bomb.zip", buf.Bytes(), "", "")
	err := as.Extract(ctx, "site001", "bomb.zip", "out", "zip")
	if err == nil || !strings.Contains(err.Error(), "oran") {
		t.Fatalf("bomb yakalanmadı: %v", err)
	}
}

// --- Trash testleri ---

func TestTrashLifecycle(t *testing.T) {
	svc, _, home := testService(t)
	tr := NewTrashService(svc, filepath.Join(t.TempDir(), "trash"))
	ctx := context.Background()

	svc.Write(ctx, "site001", "eski.txt", []byte("silinecek"), "", "")
	dest, err := tr.MoveToTrash(ctx, "site001", "eski.txt")
	if err != nil {
		t.Fatalf("MoveToTrash: %v", err)
	}
	// Site root'tan silindi, trash'te duruyor.
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(home), "eski.txt")); !os.IsNotExist(err) {
		t.Fatal("site root'tan silinmedi")
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal("trash'te yok")
	}

	// Restore.
	name := filepath.Base(dest)
	if err := tr.Restore(ctx, "site001", name); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.FromSlash(home), "eski.txt")); err != nil {
		t.Fatal("geri yüklenmedi")
	}

	// Retention: eski girişler temizlenir, yeniler kalır.
	svc.Write(ctx, "site001", "yeni.txt", []byte("y"), "", "")
	tr.MoveToTrash(ctx, "site001", "yeni.txt")
	now := time.Now()
	// Trash girişinin zaman damgasını 30 gün geriye çek (simülasyon):
	entries, _ := os.ReadDir(filepath.Join(filepath.Dir(dest)))
	for _, e := range entries {
		old := filepath.Join(filepath.Dir(dest), e.Name())
		newName := filepath.Join(filepath.Dir(dest), "0000000000000000000-old-"+e.Name())
		os.Rename(old, newName)
	}
	removed, err := tr.Cleanup(ctx, now)
	if err != nil || removed == 0 {
		t.Fatalf("Cleanup: %d err=%v", removed, err)
	}
}
