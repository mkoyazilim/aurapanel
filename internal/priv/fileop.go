package priv

import (
	"encoding/json"
	"errors"
	"fmt"
)

// fileOpVerbs, file.op allowlist'i (FILE_MANAGER §14 Tier-1).
var fileOpVerbs = map[string]bool{
	"list": true, "read": true, "write": true, "mkdir": true,
	"rename": true, "remove": true, "stat": true, "symlink": true, "eval": true,
}

// fileOpArgs, file.op istek yapısı.
type fileOpArgs struct {
	Site    string   `json:"site"`
	Verb    string   `json:"verb"`
	Paths   []string `json:"paths"`
	Content string   `json:"content_b64"` // base64 (yalnızca write)
	Offset  int64    `json:"offset"`      // yalnızca read: başlangıç baytı
	Limit   int64    `json:"limit"`       // yalnızca read: en çok bayt (0 = sınır)
}

// validateFileOp, isteği DOĞRULAR (yürütme Linux'ta — çapraz platform).
// Dönen content: base64 çözülmüş içerik (write için).
func validateFileOp(raw json.RawMessage) (fileOpArgs, []byte, error) {
	var a fileOpArgs
	if err := strictDecode(raw, &a); err != nil {
		return a, nil, fmt.Errorf("file.op: %w", err)
	}
	if !reSiteID.MatchString(a.Site) {
		return a, nil, errors.New("file.op: site kimliği geçersiz")
	}
	if !fileOpVerbs[a.Verb] {
		return a, nil, fmt.Errorf("file.op: bilinmeyen fiil: %q", a.Verb)
	}
	if len(a.Paths) == 0 || len(a.Paths) > 3 {
		return a, nil, errors.New("file.op: yol sayısı 1..3 olmalı")
	}
	for _, p := range a.Paths {
		if len(p) > 1024 {
			return a, nil, errors.New("file.op: yol çok uzun")
		}
		for _, r := range p {
			if r == 0 || r < 0x20 {
				return a, nil, errors.New("file.op: yol geçersiz karakter içeriyor")
			}
		}
	}
	var content []byte
	if a.Verb == "write" {
		// Boş içerik = boş dosya oluştur (touch); offset > 0 = ekleme.
		b, err := b64Decode(a.Content)
		if err != nil || len(b) > fileOpContentLimit {
			return a, nil, errors.New("file.op: içerik geçersiz veya sınır aşıldı (16 MiB)")
		}
		if a.Offset < 0 || a.Limit != 0 {
			return a, nil, errors.New("file.op: write için geçersiz offset/limit")
		}
		content = b
	} else if a.Content != "" {
		return a, nil, errors.New("file.op: content_b64 yalnızca write için geçerli")
	}
	if a.Verb == "read" {
		if a.Offset < 0 || a.Limit < 0 {
			return a, nil, errors.New("file.op: offset/limit negatif olamaz")
		}
		if a.Limit == 0 || a.Limit > fileOpContentLimit {
			a.Limit = fileOpContentLimit
		}
	} else if a.Verb != "write" {
		a.Offset, a.Limit = 0, 0
	}
	return a, content, nil
}

// fileOpContentLimit: JSON üzerinden taşınan içerik sınırı (16 MiB).
// Büyük yüklemeler için akış yolu (OpenFile/CreateFile) LocalBackend'te;
// Tier-1 akış desteği sunucu fazında eklenecek.
const fileOpContentLimit = 16 << 20
