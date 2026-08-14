// filehelpers: worker'ın ortak yardımcıları (çapraz platform).
package priv

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
)

func b64Encode(b []byte) string   { return base64.StdEncoding.EncodeToString(b) }
func b64Decode(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) }

// ioReadAllLimit, stdin'i sınırla okur; sınır aşılırsa hata.
func ioReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("içerik sınırı aşıldı (%d bayt)", limit)
	}
	return b, nil
}

// atomicWrite, aynı dizinde tmp + rename ile atomik yazar.
func atomicWrite(target string, content []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(path.Dir(target), ".aurapanel-tmp-*")
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
