//go:build linux

package priv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// requireRoot, helper'ın root olarak çalıştığını zorunlu kılar.
func requireRoot() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("aurapanel-priv root olarak çalışmalı (euid=%d)", os.Geteuid())
	}
	return nil
}

// ExecError, başarısız bir komutun çıktısını (kırpılmış) taşır.
type ExecError struct {
	Bin    string
	Args   []string
	Err    error
	Output string
}

func (e *ExecError) Error() string {
	return fmt.Sprintf("%s: %v: %s", e.Bin, e.Err, e.Output)
}

// executePlan, plan'daki eylemleri sırasıyla yürütür.
// Tüm komutlar sabit mutlak yollardan (binPaths), sabit PATH ortamıyla
// ve zaman aşımıyla çalışır; shell ASLA kullanılmaz.
func executePlan(ctx context.Context, p *plan) error {
	for _, a := range p.actions {
		switch a.kind {
		case actMkdir:
			if err := os.MkdirAll(a.mkdir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", a.mkdir, err)
			}
		case actWrite:
			if err := os.WriteFile(a.write.path, []byte(a.write.content), a.write.mode); err != nil {
				return fmt.Errorf("write %s: %w", a.write.path, err)
			}
		case actCopy:
			b, err := os.ReadFile(a.copy.src)
			if err != nil {
				return fmt.Errorf("copy src %s: %w", a.copy.src, err)
			}
			if err := os.MkdirAll(filepath.Dir(a.copy.dst), 0o755); err != nil {
				return fmt.Errorf("copy dst dizini %s: %w", a.copy.dst, err)
			}
			if err := os.WriteFile(a.copy.dst, b, a.copy.mode); err != nil {
				return fmt.Errorf("copy dst %s: %w", a.copy.dst, err)
			}
		case actExec:
			cmd := exec.CommandContext(ctx, a.exec.bin, a.exec.args...)
			cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C.UTF-8"}
			out, err := cmd.CombinedOutput()
			if err != nil {
				return &ExecError{Bin: a.exec.bin, Args: a.exec.args, Err: err, Output: truncateOutput(out)}
			}
		default:
			return errors.New("bilinmeyen eylem türü")
		}
	}
	return nil
}

// truncateOutput, komut çıktısını hata mesajı için sınırlar.
func truncateOutput(b []byte) string {
	s := string(b)
	if len(s) > 2048 {
		s = s[:2048] + "…(kırpıldı)"
	}
	return s
}
