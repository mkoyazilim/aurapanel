//go:build linux

package priv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			mode := a.mkdirMode
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(a.mkdir, mode); err != nil {
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
		case actRemove:
			if err := os.Remove(a.remove); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove %s: %w", a.remove, err)
			}
		case actRemoveAll:
			if err := os.RemoveAll(a.removeAll); err != nil {
				return fmt.Errorf("removeall %s: %w", a.removeAll, err)
			}
		case actExec:
			cmd := exec.CommandContext(ctx, a.exec.bin, a.exec.args...)
			cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C.UTF-8"}
			if a.exec.stdin != "" {
				cmd.Stdin = strings.NewReader(a.exec.stdin)
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				// lshttpd -t: yalnızca WARN içeren sağlıklı config'te 1 döner —
				// gerçek hata yalnızca "[ERROR]" satırıyla ayırt edilir.
				if a.exec.tolerateWarn && !bytes.Contains(out, []byte("[ERROR]")) {
					continue
				}
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
