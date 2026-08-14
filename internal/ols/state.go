// Package ols, OLS desired-state modelini, vhost renderer'ını ve güvenli
// apply pipeline'ını uygular (ARCHITECTURE §5.1).
//
// İlke: Source of truth SQLite desired state'tir; OLS config dosyaları
// yalnızca bu paketin ürettiği çıktıdır. Hiçbir değişiklik doğrulanmadan
// uygulanmaz ve her başarısızlık otomatik rollback ile geri alınır.
package ols

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Vhost, bir sitenin OLS vhost istek hâli (desired state).
type Vhost struct {
	SiteID      string
	Domain      string
	Aliases     []string
	PHPVersion  string
	IndexFiles  []string
	TLSRedirect bool
	EnableGzip  bool
	WAF         bool
	SSL         *SSLConfig
}

// SSLConfig, vhost için TLS yapılandırması.
type SSLConfig struct {
	CertPath string
	KeyPath  string
}

// Desteklenen PHP sürüm kümesi (kurulum manifestiyle eşleşir).
var SupportedPHP = map[string]bool{"8.2": true, "8.3": true, "8.4": true}

var (
	reSiteID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)
	// RE2 lookahead desteklemez; toplam uzunluk (253) Validate içinde ayrıca
	// denetlenir. Kurallar: en az bir nokta (FQDN zorunlu), ara etiketler
	// 1..63, son etiket (TLD) 2..63 karakter.
	reDomain   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*\.[a-z0-9]{2}([a-z0-9-]{0,60}[a-z0-9])?$`)
	reIndexFmt = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)
)

// maxDomainLen, DNS'e uygun toplam domain uzunluğu (noktalar dahil).
const maxDomainLen = 253

const (
	maxAliases    = 50
	maxIndexFiles = 20
)

// Validate, desired state'i sıkı kurallarla denetler.
// Docroot/log dizinleri kullanıcıdan ALINMAZ — renderer bunları
// siteID'den türetir; böylece docroot'a müdahale imkânsızdır.
func (v *Vhost) Validate(sitesRoot, certsRoot string) error {
	if !reSiteID.MatchString(v.SiteID) {
		return fmt.Errorf("site kimliği geçersiz: %q", v.SiteID)
	}
	if len(v.Domain) > maxDomainLen || !reDomain.MatchString(v.Domain) {
		return fmt.Errorf("domain geçersiz: %q", v.Domain)
	}
	if len(v.Aliases) > maxAliases {
		return fmt.Errorf("alias sayısı %d'yi aşamaz", maxAliases)
	}
	seen := map[string]bool{v.Domain: true}
	for _, a := range v.Aliases {
		if len(a) > maxDomainLen || !reDomain.MatchString(a) {
			return fmt.Errorf("alias geçersiz: %q", a)
		}
		if seen[a] {
			return fmt.Errorf("yinelenen domain/alias: %q", a)
		}
		seen[a] = true
	}
	if !SupportedPHP[v.PHPVersion] {
		return fmt.Errorf("desteklenmeyen PHP sürümü: %q", v.PHPVersion)
	}
	if len(v.IndexFiles) == 0 || len(v.IndexFiles) > maxIndexFiles {
		return fmt.Errorf("index dosyası sayısı 1..%d olmalı", maxIndexFiles)
	}
	for _, f := range v.IndexFiles {
		if !reIndexFmt.MatchString(f) {
			return fmt.Errorf("index dosyası geçersiz: %q", f)
		}
	}
	if v.SSL != nil {
		for _, p := range []string{v.SSL.CertPath, v.SSL.KeyPath} {
			if err := validateCertPath(certsRoot, p); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateCertPath, sertifika yollarının yönetilen cert deposunda kaldığını
// ve beklenen dosya adlarına sahip olduğunu doğrular.
func validateCertPath(certsRoot, p string) error {
	if !path.IsAbs(p) {
		return errors.New("sertifika yolu mutlak olmalı")
	}
	clean := path.Clean(p)
	rootClean := path.Clean(certsRoot)
	if clean == rootClean || !strings.HasPrefix(clean, rootClean+"/") {
		return fmt.Errorf("sertifika yolu %s altında olmalı", certsRoot)
	}
	switch path.Base(clean) {
	case "fullchain.pem", "privkey.pem", "cert.pem":
	default:
		return fmt.Errorf("sertifika dosyası beklenmeyen ad: %q", path.Base(clean))
	}
	return nil
}
