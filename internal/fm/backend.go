package fm

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// Backend, dosya işlemlerinin yürütme soyutlaması (FILE_MANAGER §14):
// yalnızca DOĞRULANMIŞ mutlak yollar alır. Üretimde Tier-1 backend'i
// (priv helper → setuid(site UID) + site cgroup) bu arayüzü uygular;
// LocalBackend geliştirme/test için süreç içi çalışır.
type Backend interface {
	ListDir(ctx context.Context, absDir string) ([]os.DirEntry, error)
	ReadFile(ctx context.Context, absPath string, limit int64) ([]byte, error)
	OpenFile(ctx context.Context, absPath string) (io.ReadCloser, error)
	CreateFile(ctx context.Context, absPath string, mode os.FileMode) (io.WriteCloser, error)
	WriteFileAtomic(ctx context.Context, absPath string, data []byte, mode os.FileMode) error
	Stat(ctx context.Context, absPath string) (os.FileInfo, error)
	MkdirAll(ctx context.Context, absPath string, mode os.FileMode) error
	Rename(ctx context.Context, from, to string) error
	RemoveAll(ctx context.Context, absPath string) error
	Symlink(ctx context.Context, target, link string) error
	EvalSymlinks(ctx context.Context, absPath string) (string, error)
}

// LocalBackend, süreç içi (geliştirme/test) Backend uygulaması.
// Windows'ta çalışabilmesi için POSIX yollar OS sınırında dönüştürülür;
// üretim (Linux) davranışı değişmez.
type LocalBackend struct{}

// NewLocalBackend, LocalBackend döndürür.
func NewLocalBackend() *LocalBackend { return &LocalBackend{} }

// osPath, POSIX mutlak yolu işletim sistemi biçimine çevirir.
func osPath(abs string) string { return filepath.FromSlash(abs) }

func (LocalBackend) ListDir(ctx context.Context, absDir string) ([]os.DirEntry, error) {
	return os.ReadDir(osPath(absDir))
}

func (LocalBackend) ReadFile(ctx context.Context, absPath string, limit int64) ([]byte, error) {
	f, err := os.Open(osPath(absPath))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

func (LocalBackend) OpenFile(ctx context.Context, absPath string) (io.ReadCloser, error) {
	return os.Open(osPath(absPath))
}

func (LocalBackend) CreateFile(ctx context.Context, absPath string, mode os.FileMode) (io.WriteCloser, error) {
	return os.OpenFile(osPath(absPath), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
}

func (LocalBackend) WriteFileAtomic(ctx context.Context, absPath string, data []byte, mode os.FileMode) error {
	return atomicWrite(osPath(absPath), data, mode)
}

func (LocalBackend) Stat(ctx context.Context, absPath string) (os.FileInfo, error) {
	return os.Stat(osPath(absPath))
}

func (LocalBackend) MkdirAll(ctx context.Context, absPath string, mode os.FileMode) error {
	return os.MkdirAll(osPath(absPath), mode)
}

func (LocalBackend) Rename(ctx context.Context, from, to string) error {
	return os.Rename(osPath(from), osPath(to))
}

func (LocalBackend) RemoveAll(ctx context.Context, absPath string) error {
	return os.RemoveAll(osPath(absPath))
}

func (LocalBackend) Symlink(ctx context.Context, target, link string) error {
	return os.Symlink(osPath(target), osPath(link))
}

func (LocalBackend) EvalSymlinks(ctx context.Context, absPath string) (string, error) {
	resolved, err := filepath.EvalSymlinks(osPath(absPath))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(resolved), nil
}

// atomicWrite, aynı dizinde tmp + rename ile atomik yazar (FILE_MANAGER §5.3).
func atomicWrite(target string, content []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".aurapanel-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, target)
}
