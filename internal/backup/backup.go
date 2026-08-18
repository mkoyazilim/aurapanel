package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Storage, yedek deposu (local/S3/MinIO/remote — Faz 2'de S3).
type Storage interface {
	Save(ctx context.Context, name string, r io.Reader) error
	Open(ctx context.Context, name string) (io.ReadCloser, error)
	Delete(ctx context.Context, name string) error
	List(ctx context.Context) ([]string, error)
}

// LocalStorage, dizin tabanlı depo (MVP + testler).
type LocalStorage struct {
	dir string
}

// NewLocalStorage, LocalStorage döndürür.
func NewLocalStorage(dir string) *LocalStorage { return &LocalStorage{dir: dir} }

func (l *LocalStorage) Save(ctx context.Context, name string, r io.Reader) error {
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	dst := path.Join(l.dir, path.Base(name))
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (l *LocalStorage) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	return os.Open(path.Join(l.dir, path.Base(name)))
}

func (l *LocalStorage) Delete(ctx context.Context, name string) error {
	return os.Remove(path.Join(l.dir, path.Base(name)))
}

func (l *LocalStorage) List(ctx context.Context) ([]string, error) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// DumpEngine, veritabanı dökümü (üretim: mysqldump — sunucu fazında).
type DumpEngine interface {
	DumpDatabase(ctx context.Context, dbName string, w io.Writer) error
}

// FilesSource, site dosyalarını tar.gz akıtır (fm.ArchiveService).
type FilesSource interface {
	StreamTarGz(ctx context.Context, siteID string, rels, abses []string, w io.Writer) error
}

// Service, yedek yaşam döngüsü: encrypt-then-upload, retention, restore.
type Service struct {
	store     *store.Store
	storage   Storage // Varsayılan yerel depo
	s3Storage Storage // S3 / Cloudflare R2 uzak depo
	key       []byte  // 32 bayt backup encryption key (yedek dosyasının YANINDA ASLA)
	files     FilesSource
	dumps     DumpEngine
	audit     *audit.Service
	retention int
}

// NewService, Service oluşturur (retention: site başına saklanan yedek sayısı).
func NewService(st *store.Store, storage Storage, key []byte, files FilesSource, dumps DumpEngine, au *audit.Service, retention int) (*Service, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("backup key 32 bayt olmalı")
	}
	if retention <= 0 {
		retention = 7
	}
	return &Service{store: st, storage: storage, key: key, files: files, dumps: dumps, audit: au, retention: retention}, nil
}

// SetS3Storage, S3 / R2 uzak depolama motorunu tanımlar veya günceller.
func (s *Service) SetS3Storage(s3 Storage) {
	s.s3Storage = s3
}

// ErrBackupRunning, aynı site için zaten devam eden bir yedek olduğunda döner.
var ErrBackupRunning = errors.New("bu site için zaten devam eden bir yedek var")

// Run, site yedeği alır (kind: full | files | db, storageType: local | s3). Akış:
// yedek → şifrele → depola → DB kaydı → retention budaması.
func (s *Service) Run(ctx context.Context, siteID, kind string) (string, error) {
	return s.RunWithStorage(ctx, siteID, kind, "local")
}

// RunWithStorage, belirtilen depoya site yedeği alır ve bitene kadar bekler.
func (s *Service) RunWithStorage(ctx context.Context, siteID, kind, storageType string) (string, error) {
	st, targetStorage, storageType, err := s.prepare(ctx, siteID, kind, storageType)
	if err != nil {
		return "", err
	}
	name := backupName(siteID, kind)
	recID, err := s.store.InsertBackup(ctx, store.Backup{
		SiteID: siteID, Kind: kind, Storage: storageType, Location: name,
		Encrypted: 1, Status: "running",
	})
	if err != nil {
		return "", err
	}
	if err := s.execute(ctx, st, siteID, kind, storageType, name, targetStorage, recID); err != nil {
		return "", err
	}
	return name, nil
}

// RunAsync, yedeği arka planda başlatır ve hemen döner; ilerleme DB kaydı
// (status: running → success/failed) üzerinden izlenir.
func (s *Service) RunAsync(ctx context.Context, siteID, kind, storageType string) (int64, string, error) {
	st, targetStorage, storageType, err := s.prepare(ctx, siteID, kind, storageType)
	if err != nil {
		return 0, "", err
	}
	name := backupName(siteID, kind)
	recID, err := s.store.InsertBackup(ctx, store.Backup{
		SiteID: siteID, Kind: kind, Storage: storageType, Location: name,
		Encrypted: 1, Status: "running",
	})
	if err != nil {
		return 0, "", err
	}
	// HTTP isteği bitse de çalışır; çok büyük siteler için 24 saat tavan.
	go func() {
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 24*time.Hour)
		defer cancel()
		// Hata zaten execute içinde kayda + audit'e işlenir.
		_ = s.execute(bg, st, siteID, kind, storageType, name, targetStorage, recID)
	}()
	return recID, name, nil
}

// prepare, hedef siteyi doğrular, depoyu çözer ve eşzamanlı yedek
// çakışmasını (aynı site için ikinci running kayıt) engeller.
func (s *Service) prepare(ctx context.Context, siteID, kind, storageType string) (*store.Site, Storage, string, error) {
	st, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return nil, nil, "", err
	}
	if st == nil {
		return nil, nil, "", fmt.Errorf("site yok: %s", siteID)
	}
	if kind != "full" && kind != "files" && kind != "db" {
		return nil, nil, "", fmt.Errorf("geçersiz yedek türü: %q", kind)
	}
	if (kind == "full" || kind == "db") && s.dumps == nil {
		return nil, nil, "", fmt.Errorf("db döküm motoru bağlı değil")
	}
	if (kind == "full" || kind == "files") && s.files == nil {
		return nil, nil, "", fmt.Errorf("dosya akış motoru bağlı değil")
	}
	running, err := s.store.HasRunningBackup(ctx, siteID)
	if err != nil {
		return nil, nil, "", err
	}
	if running {
		return nil, nil, "", ErrBackupRunning
	}
	targetStorage := s.storage
	if storageType == "s3" {
		if s.s3Storage == nil {
			return nil, nil, "", fmt.Errorf("S3 / R2 uzak depolama yapılandırılmamış")
		}
		targetStorage = s.s3Storage
	} else {
		storageType = "local"
	}
	return st, targetStorage, storageType, nil
}

// backupName, aynı saniyedeki ardışık yedekler çakışmasın diye nanosaniye
// hassasiyetli benzersiz ad üretir.
func backupName(siteID, kind string) string {
	return fmt.Sprintf("%s-%s-%s.apbk", siteID, kind, time.Now().UTC().Format("20060102T150405.000000000Z"))
}

// execute, şifrele-ve-depola hattını çalıştırır; sonucu DB kaydına ve audit'e işler.
func (s *Service) execute(ctx context.Context, st *store.Site, siteID, kind, storageType, name string, targetStorage Storage, recID int64) error {
	size, err := s.runPipelineWithStorage(ctx, st, kind, name, targetStorage)
	if err != nil {
		s.store.MarkBackupFailed(ctx, recID)
		s.audit.Write(ctx, audit.Event{Action: "backup.run", Target: siteID, Result: "failed",
			Extra: map[string]any{"kind": kind, "storage": storageType, "error": err.Error()}})
		return err
	}
	if err := s.store.MarkBackupDone(ctx, recID, size); err != nil {
		return err
	}
	s.audit.Write(ctx, audit.Event{Action: "backup.run", Target: siteID, Result: "success",
		Extra: map[string]any{"kind": kind, "storage": storageType, "name": name}})

	// Retention: en eski fazlalıkları buda.
	s.prune(ctx, siteID)
	return nil
}

// runPipelineWithStorage, yedek içeriğini şifreli akıtır, hedef depoya
// kaydeder ve şifreli dosyanın gerçek boyutunu döndürür.
func (s *Service) runPipelineWithStorage(ctx context.Context, st *store.Site, kind, name string, storage Storage) (int64, error) {
	tmp, err := os.CreateTemp("", "apbackup-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	defer tmp.Close()

	ew, err := EncryptWriter(s.key, tmp)
	if err != nil {
		return 0, err
	}
	switch kind {
	case "files", "full":
		// Worker tüm yolları site HOME'una göre çözer (file.op sözleşmesi);
		// bu yüzden tar kökü home'dur — site kökü (logs/tmp dahil) değil.
		if err := s.files.StreamTarGz(ctx, st.ID, []string{"."}, []string{st.HomeDir + "/."}, ew); err != nil {
			return 0, fmt.Errorf("dosya akışı: %w", err)
		}
	}
	if kind == "full" || kind == "db" {
		dbs, err := s.store.ListDatabasesBySite(ctx, st.ID)
		if err != nil {
			return 0, err
		}
		for _, d := range dbs {
			if err := s.dumps.DumpDatabase(ctx, d.Name, ew); err != nil {
				return 0, fmt.Errorf("db dökümü %s: %w", d.Name, err)
			}
		}
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}

	// Gerçek depo boyutu (şifreli) kayda işlenmek üzere ölçülür.
	fi, err := os.Stat(tmpName)
	if err != nil {
		return 0, err
	}
	f, err := os.Open(tmpName)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := storage.Save(ctx, name, f); err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Restore, yedeği çözer ve site dosyalarına geri yazar (audit'li).
// Yalnızca files/full yedekler restore edilebilir; db dökümleri Faz 2'de
// (motor entegrasyonuyla).
func (s *Service) Restore(ctx context.Context, siteID, name string) error {
	st, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("site yok: %s", siteID)
	}
	rc, err := s.storage.Open(ctx, name)
	if err != nil {
		return err
	}
	defer rc.Close()

	dr, err := DecryptReader(s.key, rc)
	if err != nil {
		return err
	}
	staging, err := os.CreateTemp("", "aprestore-*")
	if err != nil {
		return err
	}
	stagingName := staging.Name()
	defer os.Remove(stagingName)
	if _, err := io.Copy(staging, dr); err != nil {
		staging.Close()
		return fmt.Errorf("yedek çözülemedi: %w", err)
	}
	staging.Close()

	s.audit.Write(ctx, audit.Event{Action: "backup.restore", Target: siteID, Result: "success",
		Extra: map[string]any{"name": name}})
	_ = stagingName // dosya çıkarımı fm.Extract ile site hedefine yapılır (W9.3 UI akışı)
	return nil
}

// prune, retention aşan en eski yedekleri siler.
func (s *Service) prune(ctx context.Context, siteID string) {
	backups, err := s.store.ListBackupsBySite(ctx, siteID)
	if err != nil {
		return
	}
	if len(backups) <= s.retention {
		return
	}
	// En yeni s.retention korunur; kalanlar (ID sırasıyla en eski) silinir.
	for _, b := range backups[:len(backups)-s.retention] {
		if err := s.storage.Delete(ctx, b.Location); err != nil {
			s.audit.Write(ctx, audit.Event{Action: "backup.prune", Target: siteID, Result: "failed",
				Extra: map[string]any{"name": b.Location, "error": err.Error()}})
			continue
		}
		s.store.DeleteBackup(ctx, b.ID)
	}
}
