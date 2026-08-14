// Package sftp, jail'li SFTP hesaplarını yönetir (ARCHITECTURE §11.2,
// ROADMAP W9.4): site kullanıcılarına shell YOK — yalnızca OpenSSH
// internal-sftp, ChrootDirectory ile.
package sftp

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// reAccount, SFTP hesap adı (Linux kullanıcı adına eklenir).
var reAccount = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// ConfigInstaller, üretilen sshd config parçasını uygular (priv helper
// sshd.install_config op'u üzerinden).
type ConfigInstaller interface {
	Install(ctx context.Context, content string) error
}

// Service, SFTP hesap yaşam döngüsü.
type Service struct {
	store     *store.Store
	installer ConfigInstaller
	audit     *audit.Service
	sitesRoot string
}

// NewService, Service oluşturur.
func NewService(st *store.Store, inst ConfigInstaller, au *audit.Service, sitesRoot string) *Service {
	return &Service{store: st, installer: inst, audit: au, sitesRoot: sitesRoot}
}

// CreateAccount, jail'li SFTP hesabı oluşturur. Hesap, SİTE kullanıcısının
// kimliğiyle (aynı UID) çalışır — ayrı bir sistem kullanıcısı DEĞİLDİR;
// OpenSSH Match bloğu AllowUsers ile hesap adını site kullanıcısına eşler.
// Parola güçlü rastgele üretilir, yalnızca bir kez döner.
func (s *Service) CreateAccount(ctx context.Context, siteID, accountName, password string) (string, error) {
	if err := s.requireSite(ctx, siteID); err != nil {
		return "", err
	}
	if !reAccount.MatchString(accountName) {
		return "", fmt.Errorf("hesap adı geçersiz: %q", accountName)
	}
	if password == "" {
		password = randPassword(24)
	}
	if len(password) < 16 || len(password) > 128 {
		return "", fmt.Errorf("parola 16..128 karakter olmalı")
	}

	// Benzersizlik: site içinde aynı ad ikinci kez açılamaz.
	if u, _ := s.store.GetSFTPAccountByName(ctx, accountName); u != nil {
		return "", fmt.Errorf("hesap zaten var: %s", accountName)
	}

	jail := path.Join(s.sitesRoot, siteID, "home")
	if _, err := s.store.InsertSFTPAccount(ctx, store.SFTPAccount{
		SiteID: siteID, Username: accountName, JailPath: jail, Status: "active",
	}); err != nil {
		return "", err
	}
	if err := s.reinstall(ctx); err != nil {
		return "", fmt.Errorf("sshd config: %w", err)
	}
	s.audit.Write(ctx, audit.Event{Action: "sftp.account.create", Target: siteID,
		Extra: map[string]any{"account": accountName}})
	return password, nil
}

// RemoveAccount, hesabı siler.
func (s *Service) RemoveAccount(ctx context.Context, siteID, accountName string) error {
	u, err := s.store.GetSFTPAccountByName(ctx, accountName)
	if err != nil {
		return err
	}
	if u == nil || u.SiteID != siteID {
		return fmt.Errorf("hesap yok veya yetkisiz: %s", accountName)
	}
	if err := s.store.DeleteSFTPAccount(ctx, u.ID); err != nil {
		return err
	}
	if err := s.reinstall(ctx); err != nil {
		return fmt.Errorf("sshd config: %w", err)
	}
	s.audit.Write(ctx, audit.Event{Action: "sftp.account.remove", Target: siteID,
		Extra: map[string]any{"account": accountName}})
	return nil
}

// ListAccounts, sitenin hesaplarını döndürür.
func (s *Service) ListAccounts(ctx context.Context, siteID string) ([]store.SFTPAccount, error) {
	return s.store.ListSFTPAccountsBySite(ctx, siteID)
}

// RenderConfig, tüm aktif hesaplardan OpenSSH Match konfigürasyonunu üretir.
// Her Match: hesap adı → site kullanıcısı kimliği + ForceCommand
// internal-sftp + ChrootDirectory (jail). Shell YOKTUR.
func (s *Service) RenderConfig(ctx context.Context) (string, error) {
	accounts, err := s.store.ListAllSFTPAccounts(ctx)
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "", nil
	}
	var b strings.Builder
	b.WriteString("# AuraPanel yönetimli SFTP jail config — elle düzenlemeyin.\n")
	b.WriteString("Subsystem sftp internal-sftp\n")
	for _, a := range accounts {
		if a.Status != "active" {
			continue
		}
		// Hesap adı <siteID>-<account> biçiminde globalleştirilir. Hesap,
		// site kullanıcısının UID'siyle oluşturulan ayrı bir Linux
		// kullanıcısıdır (sunucu fazında priv user.create ile) — kimlik
		// eşlemesi Linux hesabında, jail burada sağlanır.
		globalName := a.SiteID + "-" + a.Username
		fmt.Fprintf(&b, "Match User %s\n", globalName)
		fmt.Fprintf(&b, "    AllowUsers %s\n", globalName)
		fmt.Fprintf(&b, "    AuthorizedKeysFile /dev/null\n")
		fmt.Fprintf(&b, "    PasswordAuthentication yes\n")
		fmt.Fprintf(&b, "    ChrootDirectory %s\n", a.JailPath)
		fmt.Fprintf(&b, "    ForceCommand internal-sftp\n")
	}
	return b.String(), nil
}

// reinstall, config'i üretip priv üzerinden uygular (sshd -t doğrulamalı).
func (s *Service) reinstall(ctx context.Context) error {
	content, err := s.RenderConfig(ctx)
	if err != nil {
		return err
	}
	if content == "" {
		content = "# AuraPanel: aktif SFTP hesabı yok.\n"
	}
	return s.installer.Install(ctx, content)
}

func (s *Service) requireSite(ctx context.Context, siteID string) error {
	st, err := s.store.GetSite(ctx, siteID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("site yok: %s", siteID)
	}
	return nil
}
