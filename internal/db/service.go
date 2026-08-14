package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// Service, site veritabanlarını ve kullanıcılarını yönetir.
// Tüm adlar site kimliğiyle ön eklenir (site001_wp) — çakışma imkânsız,
// ownership denetimi store kayıtlarıyla yapılır.
type Service struct {
	store  *store.Store
	engine Engine
	cipher *crypto.Cipher
	audit  *audit.Service
}

// NewService, Service oluşturur.
func NewService(st *store.Store, eng Engine, cipher *crypto.Cipher, au *audit.Service) *Service {
	return &Service{store: st, engine: eng, cipher: cipher, audit: au}
}

// Ad doğrulama kalıpları: ön ek (siteXXX_ = 8 karakter) ile birlikte
// MariaDB'nin 64 karakterlik identifier sınırının altında kalınır.
var (
	reDBName   = regexp.MustCompile(`^[a-z0-9_]{1,48}$`)
	reUserName = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
)

const (
	defaultHost   = "localhost"
	minPassLength = 16
	maxPassLength = 128
	gateTTL       = 15 * time.Minute
)

// fullName, site ön ekli ad üretir.
func fullName(siteID, name string) string { return siteID + "_" + name }

// --- Veritabanları ---

// CreateDatabase, site için yeni DB oluşturur.
func (s *Service) CreateDatabase(ctx context.Context, siteID, name string) error {
	if err := s.requireSite(ctx, siteID); err != nil {
		return err
	}
	if !reDBName.MatchString(name) {
		return fmt.Errorf("db adı geçersiz: %q (küçük harf/rakam/_, max 48)", name)
	}
	full := fullName(siteID, name)
	if err := s.engine.CreateDatabase(ctx, full); err != nil {
		return fmt.Errorf("motor: %w", err)
	}
	if _, err := s.store.InsertDatabase(ctx, store.Database{SiteID: siteID, Name: full, Charset: "utf8mb4"}); err != nil {
		return err
	}
	s.audit.Write(ctx, audit.Event{Action: "db.create", Target: siteID, Extra: map[string]any{"database": full}})
	return nil
}

// DropDatabase, siteye ait DB'yi siler.
func (s *Service) DropDatabase(ctx context.Context, siteID, name string) error {
	d, err := s.store.GetDatabaseByName(ctx, fullName(siteID, name))
	if err != nil {
		return err
	}
	if d == nil || d.SiteID != siteID {
		return fmt.Errorf("db yok veya yetkisiz: %s", name)
	}
	if err := s.engine.DropDatabase(ctx, d.Name); err != nil {
		return fmt.Errorf("motor: %w", err)
	}
	if err := s.store.DeleteDatabase(ctx, d.ID); err != nil {
		return err
	}
	s.audit.Write(ctx, audit.Event{Action: "db.drop", Target: siteID, Extra: map[string]any{"database": d.Name}})
	return nil
}

// ListDatabases, sitenin DB'lerini döndürür.
func (s *Service) ListDatabases(ctx context.Context, siteID string) ([]store.Database, error) {
	return s.store.ListDatabasesBySite(ctx, siteID)
}

// --- Kullanıcılar ---

// CreateUser, site için DB kullanıcısı oluşturur. Parola boşsa güçlü
// rastgele üretilir; parola yalnızca BİR KEZ döndürülür ve audit'e
// ASLA yazılmaz (encrypted-at-rest saklanır).
func (s *Service) CreateUser(ctx context.Context, siteID, username, password string) (string, error) {
	if err := s.requireSite(ctx, siteID); err != nil {
		return "", err
	}
	if !reUserName.MatchString(username) {
		return "", fmt.Errorf("kullanıcı adı geçersiz: %q (küçük harf/rakam/_, max 32)", username)
	}
	if password == "" {
		password = randPassword(24)
	}
	if len(password) < minPassLength || len(password) > maxPassLength {
		return "", fmt.Errorf("parola %d..%d karakter olmalı", minPassLength, maxPassLength)
	}
	full := fullName(siteID, username)
	if err := s.engine.CreateUser(ctx, full, defaultHost, password); err != nil {
		return "", fmt.Errorf("motor: %w", err)
	}
	enc, err := s.cipher.Encrypt([]byte(password))
	if err != nil {
		return "", err
	}
	if _, err := s.store.InsertDatabaseUser(ctx, store.DatabaseUser{
		SiteID: siteID, Username: full, PasswordEnc: enc, Host: defaultHost,
	}); err != nil {
		return "", err
	}
	s.audit.Write(ctx, audit.Event{Action: "db.user.create", Target: siteID, Extra: map[string]any{"username": full}})
	return password, nil
}

// DropUser, siteye ait DB kullanıcısını siler.
func (s *Service) DropUser(ctx context.Context, siteID, username string) error {
	u, err := s.store.GetDatabaseUserByName(ctx, fullName(siteID, username))
	if err != nil {
		return err
	}
	if u == nil || u.SiteID != siteID {
		return fmt.Errorf("kullanıcı yok veya yetkisiz: %s", username)
	}
	if err := s.engine.DropUser(ctx, u.Username, u.Host); err != nil {
		return fmt.Errorf("motor: %w", err)
	}
	if err := s.store.DeleteDatabaseUser(ctx, u.ID); err != nil {
		return err
	}
	s.audit.Write(ctx, audit.Event{Action: "db.user.drop", Target: siteID, Extra: map[string]any{"username": u.Username}})
	return nil
}

// ResetPassword, kullanıcının parolasını değiştirir (yeni değer döner).
func (s *Service) ResetPassword(ctx context.Context, siteID, username, newPassword string) (string, error) {
	u, err := s.store.GetDatabaseUserByName(ctx, fullName(siteID, username))
	if err != nil {
		return "", err
	}
	if u == nil || u.SiteID != siteID {
		return "", fmt.Errorf("kullanıcı yok veya yetkisiz: %s", username)
	}
	if newPassword == "" {
		newPassword = randPassword(24)
	}
	if len(newPassword) < minPassLength || len(newPassword) > maxPassLength {
		return "", fmt.Errorf("parola %d..%d karakter olmalı", minPassLength, maxPassLength)
	}
	if err := s.engine.SetUserPassword(ctx, u.Username, u.Host, newPassword); err != nil {
		return "", fmt.Errorf("motor: %w", err)
	}
	enc, err := s.cipher.Encrypt([]byte(newPassword))
	if err != nil {
		return "", err
	}
	if err := s.store.UpdateDatabaseUserPassword(ctx, u.ID, enc); err != nil {
		return "", err
	}
	s.audit.Write(ctx, audit.Event{Action: "db.user.password", Target: siteID, Extra: map[string]any{"username": u.Username}})
	return newPassword, nil
}

// RevealPassword, saklı parolayı çözer (UI'da göstermek için; audit'li).
func (s *Service) RevealPassword(ctx context.Context, siteID, username string) (string, error) {
	u, err := s.store.GetDatabaseUserByName(ctx, fullName(siteID, username))
	if err != nil {
		return "", err
	}
	if u == nil || u.SiteID != siteID {
		return "", fmt.Errorf("kullanıcı yok veya yetkisiz: %s", username)
	}
	plain, err := s.cipher.Decrypt(u.PasswordEnc)
	if err != nil {
		return "", err
	}
	s.audit.Write(ctx, audit.Event{Action: "db.user.reveal", Target: siteID, Extra: map[string]any{"username": u.Username}})
	return string(plain), nil
}

// ListUsers, sitenin DB kullanıcılarını döndürür (parolasız).
func (s *Service) ListUsers(ctx context.Context, siteID string) ([]store.DatabaseUser, error) {
	users, err := s.store.ListDatabaseUsersBySite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	for i := range users {
		users[i].PasswordEnc = "" // parola liste yanıtında ASLA
	}
	return users, nil
}

// Grant, site kullanıcısına site DB'si üzerinde tam yetki verir.
// İki kayıt da AYNI siteye ait olmak zorundadır.
func (s *Service) Grant(ctx context.Context, siteID, username, database string) error {
	u, err := s.store.GetDatabaseUserByName(ctx, fullName(siteID, username))
	if err != nil {
		return err
	}
	if u == nil || u.SiteID != siteID {
		return fmt.Errorf("kullanıcı yok veya yetkisiz: %s", username)
	}
	d, err := s.store.GetDatabaseByName(ctx, fullName(siteID, database))
	if err != nil {
		return err
	}
	if d == nil || d.SiteID != siteID {
		return fmt.Errorf("db yok veya yetkisiz: %s", database)
	}
	if err := s.engine.GrantAll(ctx, u.Username, u.Host, d.Name); err != nil {
		return fmt.Errorf("motor: %w", err)
	}
	s.audit.Write(ctx, audit.Event{Action: "db.grant", Target: siteID,
		Extra: map[string]any{"username": u.Username, "database": d.Name}})
	return nil
}

// --- Adminer gate ---

// OpenAdminer, scope kısıtlı geçici Adminer oturumu açar: yalnızca
// verilen DB'nin kullanıcı adı + parolasıyla, 15 dakika geçerli token.
func (s *Service) OpenAdminer(ctx context.Context, siteID string, databaseID int64) (string, error) {
	d, err := s.store.GetDatabaseByID(ctx, databaseID)
	if err != nil {
		return "", err
	}
	if d == nil || d.SiteID != siteID {
		return "", fmt.Errorf("db yok veya yetkisiz: id=%d", databaseID)
	}
	token := randToken()
	sum := sha256.Sum256([]byte(token))
	if _, err := s.store.InsertAdminerGate(ctx, store.AdminerGate{
		SiteID:     siteID,
		DatabaseID: nullInt64(databaseID),
		TokenHash:  hex.EncodeToString(sum[:]),
		ExpiresAt:  time.Now().UTC().Add(gateTTL).Format(time.RFC3339),
	}); err != nil {
		return "", err
	}
	// Tembel temizlik: süresi dolan gate'ler açılışta silinir.
	s.store.DeleteExpiredAdminerGates(ctx, time.Now().UTC().Format(time.RFC3339))

	s.audit.Write(ctx, audit.Event{Action: "adminer.open", Target: siteID,
		Extra: map[string]any{"database": d.Name}})
	return token, nil
}

// ValidateAdminer, gate token'ını doğrular; geçerliyse scope bilgilerini
// (db adı) döndürür. Süresi dolmuşsa silinir ve hata döner.
func (s *Service) ValidateAdminer(ctx context.Context, token string) (string, error) {
	sum := sha256.Sum256([]byte(token))
	g, err := s.store.GetAdminerGateByHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return "", err
	}
	if g == nil {
		return "", fmt.Errorf("gate geçersiz")
	}
	if g.ExpiresAt < time.Now().UTC().Format(time.RFC3339) {
		s.store.DeleteAdminerGate(ctx, g.ID)
		return "", fmt.Errorf("gate süresi doldu")
	}
	if !g.DatabaseID.Valid {
		return "", fmt.Errorf("gate kapsamı bozuk")
	}
	d, err := s.store.GetDatabaseByID(ctx, g.DatabaseID.Int64)
	if err != nil {
		return "", err
	}
	if d == nil {
		return "", fmt.Errorf("gate kapsamındaki db yok")
	}
	return d.Name, nil
}

// CloseAdminer, gate'i kapatır.
func (s *Service) CloseAdminer(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	g, err := s.store.GetAdminerGateByHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		return err
	}
	if g == nil {
		return nil // zaten kapalı — idempotent
	}
	return s.store.DeleteAdminerGate(ctx, g.ID)
}

// --- Yardımcılar ---

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

// randPassword, güvenli rastgele parola üretir (küçük/büyük/rakam).
func randPassword(n int) string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Kripto kaynağı arızası: zaman bazlı yedek (nadir ve kabul edilebilir).
		return fmt.Sprintf("p%016x", time.Now().UnixNano())
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// randToken, gate token'ı (32 byte hex).
func randToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("t%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func nullInt64(v int64) sql.NullInt64 { return sql.NullInt64{Int64: v, Valid: true} }
