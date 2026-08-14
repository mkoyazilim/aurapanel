package priv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// privLog, helper'ın append-only kaydı (ARCHITECTURE §3.1):
// root'a ait, 0640 root:panel-user — panel okuyabilir, YAZAMAZ.
// Dosya O_APPEND ile açılır; her kayıt tek JSON satırıdır ve sync edilir.
type privLog struct {
	mu sync.Mutex
	f  *os.File
}

func openPrivLog(path string, panelGID uint32) (*privLog, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("priv log dizini %s: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o640)
	if err != nil {
		return nil, fmt.Errorf("priv log açılamadı: %w", err)
	}
	if err := f.Chmod(0o640); err != nil {
		f.Close()
		return nil, err
	}
	if runtime.GOOS != "windows" {
		if err := f.Chown(0, int(panelGID)); err != nil {
			f.Close()
			return nil, err
		}
	}
	return &privLog{f: f}, nil
}

func (l *privLog) write(entry map[string]any) error {
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.f.Write(append(b, '\n')); err != nil {
		return err
	}
	return l.f.Sync()
}

func (l *privLog) close() error { return l.f.Close() }
