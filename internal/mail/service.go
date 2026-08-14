package mail

import (
	"context"
	"fmt"
	"strings"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

type Dependencies struct {
	Store *store.Store
	Audit *audit.Service
}

type Service struct {
	deps Dependencies
}

func NewService(deps Dependencies) *Service {
	return &Service{deps: deps}
}

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

	return nil
}

func (s *Service) DeleteAccount(ctx context.Context, email string) error {
	if err := s.deps.Store.DeleteMailAccount(ctx, email); err != nil {
		return err
	}
	s.deps.Audit.Write(ctx, audit.Event{
		Action: "mail.delete_account",
		Target: email,
	})
	return nil
}
