package fm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/unicode/norm"
)

// UploadPolicy, yükleme sınırları (FILE_MANAGER §7.1).
type UploadPolicy struct {
	MaxFileSize  int64 // bayt; 0 = sınırsız
	MaxChunkSize int64 // bayt; varsayılan 50 MiB
}

// DefaultUploadPolicy: 2 GB dosya, 50 MiB chunk.
func DefaultUploadPolicy() UploadPolicy {
	return UploadPolicy{MaxFileSize: 2 << 30, MaxChunkSize: 50 << 20}
}

// UploadService, chunked/resume yüklemeleri yönetir (FILE_MANAGER §7):
// chunk'lar panel staging alanına yazılır → Finalize birleştirir →
// chunk SHA-256'ları doğrulanmış şekilde hedefe taşır.
type UploadService struct {
	mu      sync.Mutex
	staging string
	fs      *FileService
	policy  UploadPolicy
}

// NewUploadService, UploadService oluşturur (staging: panel alanı).
func NewUploadService(fs *FileService, staging string) *UploadService {
	return &UploadService{staging: staging, fs: fs, policy: DefaultUploadPolicy()}
}

// SetPolicy, yükleme politikasını günceller.
func (u *UploadService) SetPolicy(p UploadPolicy) { u.policy = p }

type uploadManifest struct {
	UploadID  string            `json:"upload_id"`
	SiteID    string            `json:"site_id"`
	Target    string            `json:"target"` // kök-göreli hedef dosya
	FileName  string            `json:"file_name"`
	TotalSize int64             `json:"total_size"`
	ChunkSize int64             `json:"chunk_size"`
	Chunks    map[int]chunkMeta `json:"chunks"`
}

type chunkMeta struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// validFileName, dosya adını normalleştirir ve doğrular (FILE_MANAGER §7.1):
// NFC, ayraç yok, NUL/kontrol yok, uzunluk sınırı.
func validFileName(name string) (string, error) {
	if name == "" || len(name) > 255 {
		return "", ErrInvalidPath
	}
	for _, r := range name {
		if r == 0 || r < 0x20 {
			return "", ErrInvalidPath
		}
	}
	normName := norm.NFC.String(name)
	if normName == "." || normName == ".." || strings.ContainsAny(normName, `/\`) {
		return "", ErrInvalidPath
	}
	return normName, nil
}

// Init, yükleme oturumu başlatır; uploadID döndürür
// (biçim: <siteID>-<32 hex>).
func (u *UploadService) Init(siteID, targetDir, fileName string, totalSize int64) (string, error) {
	if _, err := u.fs.siteHome(siteID); err != nil {
		return "", err
	}
	name, err := validFileName(fileName)
	if err != nil {
		return "", err
	}
	if u.policy.MaxFileSize > 0 && totalSize > u.policy.MaxFileSize {
		return "", fmt.Errorf("dosya boyutu sınırı aşıldı (%d bayt)", u.policy.MaxFileSize)
	}
	targetDir = path.Clean(targetDir)
	if targetDir == ".." || strings.HasPrefix(targetDir, "../") {
		return "", ErrInvalidPath
	}

	id := siteID + "-" + randHex(32)
	dir := u.uploadDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	chunkSize := u.policy.MaxChunkSize
	if chunkSize <= 0 {
		chunkSize = 50 << 20
	}
	m := uploadManifest{
		UploadID: id, SiteID: siteID,
		Target:    path.Join(targetDir, name),
		FileName:  name, TotalSize: totalSize,
		ChunkSize: chunkSize, Chunks: map[int]chunkMeta{},
	}
	return id, u.saveManifest(&m)
}

// Chunk, tek parçayı kaydeder (idempotent — resume). chunkSHA256 verilmişse
// doğrulanır; aynı hash'li parça zaten varsa tekrar yazılmaz.
func (u *UploadService) Chunk(uploadID string, index int, data []byte, chunkSHA256 string) (int64, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	m, dir, err := u.loadManifest(uploadID)
	if err != nil {
		return 0, err
	}
	if index < 0 {
		return 0, errors.New("chunk index geçersiz")
	}
	if int64(len(data)) > m.ChunkSize {
		return 0, fmt.Errorf("chunk boyutu sınırı aşıldı (%d)", m.ChunkSize)
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if chunkSHA256 != "" && chunkSHA256 != hash {
		return 0, errors.New("chunk SHA-256 eşleşmiyor")
	}
	if meta, ok := m.Chunks[index]; ok && meta.SHA256 == hash {
		return meta.Size, nil // resume: parça zaten güvenli
	}

	chunkPath := path.Join(dir, fmt.Sprintf("chunk-%06d", index))
	if err := atomicWrite(chunkPath, data, 0o600); err != nil {
		return 0, err
	}
	m.Chunks[index] = chunkMeta{Size: int64(len(data)), SHA256: hash}
	return int64(len(data)), u.saveManifest(m)
}

// Finalize, parçaları birleştirir (boyut + manifest tutarlılığı doğrulanır;
// chunk hash'leri zaten Chunk aşamasında doğrulanmıştır), executable
// politikasını işletir ve hedefe ATOMİK yazar.
func (u *UploadService) Finalize(ctx context.Context, uploadID string) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	m, dir, err := u.loadManifest(uploadID)
	if err != nil {
		return "", err
	}
	if len(m.Chunks) == 0 {
		return "", errors.New("yüklenecek parça yok")
	}

	finalPath := path.Join(dir, "final")
	out, err := os.OpenFile(finalPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	var total int64
	for i := 0; i < len(m.Chunks); i++ {
		meta, ok := m.Chunks[i]
		if !ok {
			out.Close()
			return "", fmt.Errorf("eksik parça: %d", i)
		}
		b, err := os.ReadFile(path.Join(dir, fmt.Sprintf("chunk-%06d", i)))
		if err != nil {
			out.Close()
			return "", err
		}
		if int64(len(b)) != meta.Size {
			out.Close()
			return "", fmt.Errorf("parça %d boyut uyuşmazlığı", i)
		}
		if _, err := out.Write(b); err != nil {
			out.Close()
			return "", err
		}
		total += int64(len(b))
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	if total != m.TotalSize {
		return "", fmt.Errorf("toplam boyut uyuşmuyor: %d != %d", total, m.TotalSize)
	}

	// Executable politikası (FILE_MANAGER §7.2): uyarı + audit.
	if exec, kind := detectExecutable(finalPath); exec {
		u.fs.auditEvent(ctx, m.SiteID, "upload", m.Target, "warning",
			map[string]any{"reason": "executable", "kind": kind})
	}

	// Hedef: FileService pipeline'ından geçer (resolve + rate limit).
	if err := u.fs.allow("write"); err != nil {
		return "", err
	}
	if err := u.fs.copyAbsFile(ctx, m.SiteID, finalPath, m.Target); err != nil {
		return "", err
	}
	u.fs.auditEvent(ctx, m.SiteID, "upload", m.Target, "success", map[string]any{"size": total})
	os.RemoveAll(dir)
	return m.Target, nil
}

// Abort, yükleme oturumunu iptal eder (staging temizliği).
func (u *UploadService) Abort(uploadID string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	dir := u.uploadDir(uploadID)
	if _, err := os.Stat(dir); err != nil {
		return errors.New("yükleme oturumu yok")
	}
	return os.RemoveAll(dir)
}

// detectExecutable, ELF magic ve shebang tespiti.
func detectExecutable(p string) (bool, string) {
	f, err := os.Open(p)
	if err != nil {
		return false, ""
	}
	defer f.Close()
	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil {
		return false, ""
	}
	if head[0] == 0x7f && head[1] == 'E' && head[2] == 'L' && head[3] == 'F' {
		return true, "elf"
	}
	if head[0] == '#' && head[1] == '!' {
		return true, "shebang"
	}
	return false, ""
}

// copyAbsFile, staging dosyasını FileService pipeline'ı üzerinden hedefe
// ATOMİK finalizasyonla yazar: tmp dosyaya stream kopyala → rename.
func (s *FileService) copyAbsFile(ctx context.Context, siteID, src, relTarget string) error {
	abs, err := s.resolveAbs(ctx, siteID, relTarget)
	if err != nil {
		return err
	}
	rf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer rf.Close()

	tmp := abs + ".aurapanel-upload-tmp"
	w, err := s.backend.CreateFile(ctx, tmp, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, rf); err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return s.backend.Rename(ctx, tmp, abs)
}

// --- manifest yardımcıları ---

// uploadDir, uploadID'nin staging dizinini türetir
// (biçim: staging/<siteID>/<id>/).
func (u *UploadService) uploadDir(id string) string {
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		return ""
	}
	return path.Join(u.staging, parts[0], id)
}

func (u *UploadService) loadManifest(id string) (*uploadManifest, string, error) {
	dir := u.uploadDir(id)
	if dir == "" {
		return nil, "", errors.New("yükleme kimliği geçersiz")
	}
	b, err := os.ReadFile(path.Join(dir, "manifest.json"))
	if err != nil {
		return nil, "", errors.New("yükleme oturumu yok")
	}
	var m uploadManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, "", err
	}
	return &m, dir, nil
}

func (u *UploadService) saveManifest(m *uploadManifest) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return atomicWrite(path.Join(u.uploadDir(m.UploadID), "manifest.json"), b, 0o600)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
