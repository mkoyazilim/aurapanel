package mail

import (
	"context"
	"fmt"
	"strings"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// PrivClient abstracts the privileged helper so the service is testable.
type PrivClient interface {
	Call(ctx context.Context, op string, args map[string]any) (map[string]any, error)
}

// MailStatus reports whether the mail stack services are running.
type MailStatus struct {
	Postfix  string `json:"postfix"`
	Dovecot  string `json:"dovecot"`
	OpenDKIM string `json:"opendkim"`
}

type Dependencies struct {
	Store *store.Store
	Audit *audit.Service
	Priv  PrivClient
}

type Service struct {
	deps Dependencies
}

func NewService(deps Dependencies) *Service {
	return &Service{deps: deps}
}

// CreateAccount creates a mail account in the DB and provisions it on the server.
func (s *Service) CreateAccount(ctx context.Context, siteID, domain, localPart, password string, quotaMB int) error {
	if quotaMB <= 0 {
		quotaMB = 512
	}

	email := fmt.Sprintf("%s@%s", localPart, domain)

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}

	if err := s.deps.Store.EnsureMailDomain(ctx, domain, siteID); err != nil {
		return err
	}

	acc := store.MailAccount{
		Email:        email,
		Domain:       domain,
		PasswordHash: hash,
		QuotaMB:      quotaMB,
	}

	if err := s.deps.Store.CreateMailAccount(ctx, acc); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("email address already exists")
		}
		return err
	}

	s.deps.Audit.Write(ctx, audit.Event{
		Action: "mail.create_account",
		Target: email,
	})

	if err := s.provision(ctx); err != nil {
		return fmt.Errorf("provisioning after create: %w", err)
	}

	return nil
}

// DeleteAccount removes a mail account from the DB and re-provisions the server.
func (s *Service) DeleteAccount(ctx context.Context, email string) error {
	if err := s.deps.Store.DeleteMailAccount(ctx, email); err != nil {
		return err
	}
	s.deps.Audit.Write(ctx, audit.Event{
		Action: "mail.delete_account",
		Target: email,
	})

	if err := s.provision(ctx); err != nil {
		return fmt.Errorf("provisioning after delete: %w", err)
	}
	return nil
}

// GenerateDKIM generates a DKIM keypair for the domain via the priv helper
// and stores it in the DB. Returns the public DNS TXT record value.
func (s *Service) GenerateDKIM(ctx context.Context, domain string) (string, error) {
	data, err := s.deps.Priv.Call(ctx, "mail.dkim_generate", map[string]any{
		"domain": domain,
	})
	if err != nil {
		return "", fmt.Errorf("DKIM generation: %w", err)
	}

	pubKey, _ := data["public_key"].(string)
	privKey, _ := data["private_key"].(string)

	if pubKey == "" {
		return "", fmt.Errorf("DKIM generation returned empty public key")
	}

	if err := s.deps.Store.SaveDKIMKey(ctx, domain, privKey, pubKey); err != nil {
		return "", fmt.Errorf("saving DKIM key: %w", err)
	}

	s.deps.Audit.Write(ctx, audit.Event{
		Action: "mail.dkim_generate",
		Target: domain,
	})

	return pubKey, nil
}

// GetDKIMRecord returns the stored DKIM public key for DNS configuration.
func (s *Service) GetDKIMRecord(ctx context.Context, domain string) (string, error) {
	_, pub, err := s.deps.Store.GetDKIMKey(ctx, domain)
	if err != nil {
		return "", err
	}
	return pub, nil
}

// GetMailStatus checks whether Postfix, Dovecot, and OpenDKIM are running.
func (s *Service) GetMailStatus(ctx context.Context) (MailStatus, error) {
	data, err := s.deps.Priv.Call(ctx, "server.services", nil)
	if err != nil {
		return MailStatus{}, fmt.Errorf("checking mail services: %w", err)
	}

	status := MailStatus{
		Postfix:  serviceState(data, "postfix"),
		Dovecot:  serviceState(data, "dovecot"),
		OpenDKIM: serviceState(data, "opendkim"),
	}
	return status, nil
}

// SetupMailServer performs the one-time initial configuration of
// Postfix, Dovecot, and OpenDKIM via the priv helper.
func (s *Service) SetupMailServer(ctx context.Context) error {
	_, err := s.deps.Priv.Call(ctx, "mail.setup", nil)
	if err != nil {
		return fmt.Errorf("mail server setup: %w", err)
	}

	s.deps.Audit.Write(ctx, audit.Event{
		Action: "mail.setup",
		Target: "server",
	})
	return nil
}

// ChangePassword updates the password hash in the DB and re-provisions
// the Dovecot users file.
func (s *Service) ChangePassword(ctx context.Context, email, newPassword string) error {
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.deps.Store.UpdateMailPassword(ctx, email, hash); err != nil {
		return err
	}

	s.deps.Audit.Write(ctx, audit.Event{
		Action: "mail.change_password",
		Target: email,
	})

	if err := s.provision(ctx); err != nil {
		return fmt.Errorf("provisioning after password change: %w", err)
	}
	return nil
}

// provision regenerates Postfix/Dovecot config files from the DB and reloads services.
func (s *Service) provision(ctx context.Context) error {
	_, err := s.deps.Priv.Call(ctx, "mail.provision", nil)
	return err
}

// serviceState safely extracts a service state string from the priv response map.
func serviceState(data map[string]any, name string) string {
	if v, ok := data[name].(string); ok {
		return v
	}
	return "unknown"
}
