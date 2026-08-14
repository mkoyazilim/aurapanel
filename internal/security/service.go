// Package security, PHP güvenlik profili yönetimini sağlar.
// Minimal / Balanced / Hardened profilleri site başına php.ini üretir.
package security

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Profile sabitleri.
const (
	ProfileMinimal  = "minimal"
	ProfileBalanced = "balanced"
	ProfileHardened = "hardened"
)

// ProfileInfo, bir profilin kullanıcıya gösterilecek açıklaması.
type ProfileInfo struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Settings    []string `json:"settings"`
}

// Profiles, mevcut profil açıklamaları.
var Profiles = []ProfileInfo{
	{
		ID:          ProfileMinimal,
		Label:       "Minimal",
		Description: "Temel izolasyon. Geliştirme veya güvenilen uygulamalar için uygundur.",
		Settings:    []string{"expose_php=Off", "display_errors=Off"},
	},
	{
		ID:          ProfileBalanced,
		Label:       "Balanced",
		Description: "Dengeli güvenlik. Üretim ortamları için önerilen varsayılan.",
		Settings:    []string{"expose_php=Off", "display_errors=Off", "open_basedir", "disable_functions (kısmi)"},
	},
	{
		ID:          ProfileHardened,
		Label:       "Hardened",
		Description: "Maksimum güvenlik. Katı kısıtlamalar; bazı uygulamalar uyumsuz olabilir.",
		Settings:    []string{"expose_php=Off", "display_errors=Off", "open_basedir", "disable_functions (tam)", "session hardening", "WAF temel"},
	},
}

// Service, güvenlik profili yönetim servisi.
type Service struct {
	st        *store.Store
	priv      *privclient.Client
	sitesRoot string
}

// NewService, Service oluşturur.
func NewService(st *store.Store, priv *privclient.Client, sitesRoot string) *Service {
	return &Service{st: st, priv: priv, sitesRoot: sitesRoot}
}

// GetWAF returns the WAF status of a site.
func (s *Service) GetWAF(ctx context.Context, siteID string) (bool, error) {
	st, err := s.st.GetSite(ctx, siteID)
	if err != nil {
		return false, err
	}
	var flags map[string]any
	if err := json.Unmarshal([]byte(st.FeatureFlags), &flags); err != nil {
		return false, nil // Default false
	}
	val, _ := flags["waf_enabled"].(bool)
	return val, nil
}

// SetWAF enables or disables WAF for a site.
func (s *Service) SetWAF(ctx context.Context, siteID string, enabled bool) error {
	st, err := s.st.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	var flags map[string]any
	if err := json.Unmarshal([]byte(st.FeatureFlags), &flags); err != nil {
		flags = make(map[string]any)
	}
	flags["waf_enabled"] = enabled
	
	b, _ := json.Marshal(flags)
	if err := s.st.UpdateSiteFeatureFlags(ctx, siteID, string(b)); err != nil {
		return err
	}
	return nil
}

// GetProfile, sitenin mevcut güvenlik profilini döndürür.
func (s *Service) GetProfile(ctx context.Context, siteID string) (string, error) {
	return s.st.GetSecurityProfile(ctx, siteID)
}

// SetProfile, sitenin güvenlik profilini değiştirir ve php.ini'yi günceller.
func (s *Service) SetProfile(ctx context.Context, siteID, profile string) error {
	if !validProfile(profile) {
		return fmt.Errorf("geçersiz profil: %q (minimal|balanced|hardened)", profile)
	}

	if err := s.st.SetSecurityProfile(ctx, siteID, profile); err != nil {
		return err
	}

	// php.ini'yi priv üzerinden güncelle.
	iniContent := buildIni(siteID, profile, s.sitesRoot)
	iniPath := fmt.Sprintf("%s/%s/conf/php.ini", s.sitesRoot, siteID)
	_, err := s.priv.Call(ctx, "php.install_ini", map[string]any{
		"site":    siteID,
		"path":    iniPath,
		"content": iniContent,
	})
	return err
}

func validProfile(p string) bool {
	return p == ProfileMinimal || p == ProfileBalanced || p == ProfileHardened
}

// buildIni, profil adına göre php.ini içeriği üretir.
func buildIni(siteID, profile, sitesRoot string) string {
	base := `; AuraPanel güvenlik profili: ` + profile + `
; Elle düzenlemeyin.
expose_php = Off
display_errors = Off
log_errors = On
error_log = ` + sitesRoot + `/` + siteID + `/logs/php-errors.log
`

	switch profile {
	case ProfileBalanced:
		base += `
open_basedir = ` + sitesRoot + `/` + siteID + `/home:` + sitesRoot + `/` + siteID + `/tmp:/tmp
disable_functions = exec,passthru,shell_exec,system,proc_open,popen
`
	case ProfileHardened:
		base += `
open_basedir = ` + sitesRoot + `/` + siteID + `/home:` + sitesRoot + `/` + siteID + `/tmp
disable_functions = exec,passthru,shell_exec,system,proc_open,popen,curl_exec,curl_multi_exec,parse_ini_file,show_source,phpinfo
session.cookie_httponly = On
session.cookie_secure = On
session.use_strict_mode = On
session.cookie_samesite = Strict
allow_url_fopen = Off
allow_url_include = Off
`
	}
	return base
}
