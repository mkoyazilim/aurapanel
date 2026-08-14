// Package logger, site log dosyalarını canlı olarak akıtır.
// WebSocket üzerinden tail -f benzeri davranış sağlar.
package logger

import (
	"bufio"
	"context"
	"io"
	"os"
	"path"
	"time"
)

// TailOptions, log izleme seçenekleri.
type TailOptions struct {
	SiteID   string // site kimliği
	LogFile  string // "access" | "error"
	LogsRoot string // /srv/aurapanel/sites
}

// TailLine, akıtılan tek bir log satırı.
type TailLine struct {
	Line string `json:"line"`
	Ts   string `json:"ts"`
}

// Tail, log dosyasını ctx iptal edilene kadar akıtır.
// Her yeni satır lines kanalına gönderilir.
func Tail(ctx context.Context, opts TailOptions, lines chan<- TailLine) error {
	filename := "access.log"
	if opts.LogFile == "error" {
		filename = "error.log"
	}
	logPath := path.Join(opts.LogsRoot, opts.SiteID, "logs", filename)

	f, err := os.Open(logPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Dosyanın sonuna git (sadece yenileri akıt).
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	reader := bufio.NewReader(f)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for {
				line, err := reader.ReadString('\n')
				if len(line) > 0 {
					select {
					case lines <- TailLine{Line: line, Ts: time.Now().UTC().Format(time.RFC3339)}:
					case <-ctx.Done():
						return nil
					}
				}
				if err != nil {
					// Daha fazla veri yok, bir sonraki tick'te tekrar dene.
					break
				}
			}
		}
	}
}
