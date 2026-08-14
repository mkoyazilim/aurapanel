// Package fm, File Manager'ın Filesystem Access Layer'ını uygular
// (FILE_MANAGER.md): tüm dosya erişimi FileService üzerinden geçer;
// UI/HTTP katmanı ASLA doğrudan os paketini kullanmaz.
package fm

import (
	"errors"
	"path"
	"strings"
)

// Erişim hatası türleri (FILE_MANAGER §2 → HTTP 403).
var (
	ErrInvalidPath = errors.New("geçersiz yol")
	ErrOutsideRoot = errors.New("site root dışına erişim engellendi")
)

// canonicalizer, mutlak yolu symlink'leri çözerek kanonik hâle getirir.
// Testlerde sahte uygulamayla symlink politikası platformdan bağımsız
// kanıtlanır (gerçek symlink oluşturma Windows'ta ayrıcalık ister).
type canonicalizer func(p string) (string, error)

// resolve, FILE_MANAGER §2.2 pipeline'ını uygular:
//
//	Normalize → Join → Canonicalize (symlink'ler) → Verify Site Root
//
// Safe Mode (varsayılan AÇIK): site root DIŞINA çıkan her çözüm reddedilir —
// ara bileşenlerdeki kaçış symlink'leri de EvalSymlinks sonucunu dışarıda
// bırakacağından aynı doğrulama ile yakalanır. Root İÇİNE işaret eden
// symlink'lere izin verilir.
func resolve(canon canonicalizer, siteHome, relPath string) (string, error) {
	// 1. Normalize: NUL ve kontrol karakterleri reddi.
	for _, r := range relPath {
		if r == 0 || r < 0x20 {
			return "", ErrInvalidPath
		}
	}
	// 2. POSIX temizliği: mutlak yol ve ".." kaçışı reddi.
	// NOT: ".." kontrolü Clean SONRASI ve leading slash EKLENMEDEN yapılır —
	// path.Clean("/"+rel) kök seviyesindeki ".."'ları sessizce yutar
	// (traversal matrisinin yakaladığı gerçek hata).
	if path.IsAbs(relPath) {
		return "", ErrInvalidPath
	}
	clean := path.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrInvalidPath
	}

	// 3. Join + Canonicalize.
	full := path.Join(siteHome, clean)
	resolved, err := canon(full)
	if err != nil {
		// Hedef henüz yok (dosya oluşturma senaryosu): en yakın VAR OLAN
		// atadan çöz, kalan bileşenleri temiz biçimde ekle.
		resolved, err = resolveDeepestExisting(canon, full)
		if err != nil {
			return "", err
		}
	}

	// 4. Verify: kanonik yol site root içinde kalmak ZORUNDA.
	root := path.Clean(siteHome)
	if resolved != root && !strings.HasPrefix(resolved, root+"/") {
		return "", ErrOutsideRoot
	}
	return resolved, nil
}

// resolveDeepestExisting, var olan en derin ataya kadar iner ve eksik
// bileşenleri geri ekler.
func resolveDeepestExisting(canon canonicalizer, p string) (string, error) {
	missing := ""
	cur := p
	for {
		if _, err := canon(cur); err == nil {
			break
		}
		parent := path.Dir(cur)
		if parent == cur {
			return "", errors.New("yol çözümlenemedi")
		}
		missing = path.Join(path.Base(cur), missing)
		cur = parent
	}
	resolved, err := canon(cur)
	if err != nil {
		return "", err
	}
	if missing == "" {
		return resolved, nil
	}
	return path.Join(resolved, path.Clean(missing)), nil
}
