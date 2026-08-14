package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

import (
	"github.com/mkoyazilim/aurapanel/internal/audit"
	"github.com/mkoyazilim/aurapanel/internal/auth"
	"github.com/mkoyazilim/aurapanel/internal/config"
	"github.com/mkoyazilim/aurapanel/internal/crypto"
	"github.com/mkoyazilim/aurapanel/internal/fm"
	"github.com/mkoyazilim/aurapanel/internal/priv"
	"github.com/mkoyazilim/aurapanel/internal/privclient"
	"github.com/mkoyazilim/aurapanel/internal/site"
	"github.com/mkoyazilim/aurapanel/internal/store"
)

// fakeSiteMgr, SiteManager arayüzünün test uygulaması.
type fakeSiteMgr struct {
	created []site.CreateRequest
}

func (f *fakeSiteMgr) Create(ctx context.Context, req site.CreateRequest) (string, error) {
	f.created = append(f.created, req)
	return "site001", nil
}
func (f *fakeSiteMgr) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeSiteMgr) UpdateLimits(ctx context.Context, id string, l site.Limits) error {
	return nil
}
func (f *fakeSiteMgr) SetFeatureFlags(ctx context.Context, id string, flags map[string]bool) error {
	return nil
}
func (f *fakeSiteMgr) ListSites(ctx context.Context) ([]store.Site, error) {
	return []store.Site{}, nil
}

// testServerWithPriv, gerçek store + auth + fm (LocalBackend) ile test
// sunucusu kurar; sock boş değilse Priv istemcisi o sokete bağlanır.
func testServerWithPriv(t *testing.T, sock string) (*httptest.Server, *store.Store, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	au := audit.New(st)
	key, _ := crypto.GenerateKey()
	cipher, _ := crypto.New(key)

	sitesRoot := filepath.ToSlash(filepath.Join(t.TempDir(), "sites"))
	files := fm.New(fm.NewLocalBackend(), au, sitesRoot)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	deps := Deps{
		Store: st, Audit: au, Sessions: auth.NewSessionStore(st), Cipher: cipher,
		Cfg: config.Default(), Log: log,
		Sites: &fakeSiteMgr{}, Files: files,
		Uploads: fm.NewUploadService(files, filepath.Join(t.TempDir(), "staging")),
		Archive: fm.NewArchiveService(files),
		Trash:   fm.NewTrashService(files, filepath.Join(t.TempDir(), "trash")),
	}
	if sock != "" {
		deps.Priv = privclient.New(sock, 5*time.Second)
	}
	srv := New(deps)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, st, sitesRoot
}

func testServer(t *testing.T) (*httptest.Server, *store.Store, string) {
	t.Helper()
	return testServerWithPriv(t, "")
}

// createAdmin, test için admin kullanıcısı üretir; (username, password) döner.
func createAdmin(t *testing.T, st *store.Store, username, password string) {
	t.Helper()
	ctx := context.Background()
	roleID, _ := st.GetRoleIDByName(ctx, "admin")
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertUser(ctx, store.User{
		Username: username, PasswordHash: hash, RoleID: roleID,
		MustChangePassword: false, Status: "active",
	}); err != nil {
		t.Fatal(err)
	}
}

// apiClient, cookie jar'lı test istemcisi (CSRF header desteği).
type apiClient struct {
	t      *testing.T
	base   string
	client *http.Client
	csrf   string
}

func newClient(t *testing.T, base string) *apiClient {
	jar, _ := cookiejar.New(nil)
	return &apiClient{t: t, base: base, client: &http.Client{Jar: jar}}
}

func (c *apiClient) post(path string, body any, withCSRF bool) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(http.MethodPost, c.base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if withCSRF && c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (c *apiClient) get(path string) (int, map[string]any) {
	resp, err := c.client.Get(c.base + path)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (c *apiClient) put(path string, body any, withCSRF bool) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(http.MethodPut, c.base+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if withCSRF && c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestLoginFlowAndForcedChange(t *testing.T) {
	ts, st, _ := testServer(t)
	createAdmin(t, st, "admin-test", "ilk-parola-123")
	c := newClient(t, ts.URL)

	// Yanlış parola → 401.
	if code, _ := c.post("/api/v1/auth/login", map[string]string{"username": "admin-test", "password": "yanlis"}, false); code != 401 {
		t.Fatalf("yanlış parola kodu: %d", code)
	}
	// Doğru giriş.
	code, out := c.post("/api/v1/auth/login", map[string]string{"username": "admin-test", "password": "ilk-parola-123"}, false)
	if code != 200 {
		t.Fatalf("giriş kodu: %d %v", code, out)
	}
	c.csrf, _ = out["csrf_token"].(string)
	if c.csrf == "" {
		t.Fatal("csrf token dönmedi")
	}

	// Oturumsuz istek → 401.
	if code, _ := newClient(t, ts.URL).get("/api/v1/status"); code != 401 {
		t.Fatalf("oturumsuz status kodu: %d", code)
	}
	// Oturumlu status → 200.
	if code, _ := c.get("/api/v1/status"); code != 200 {
		t.Fatalf("status kodu: %d", code)
	}

	// CSRF'siz durum değiştirme → 403.
	code, _ = c.post("/api/v1/sites", map[string]string{"domain": "x.com"}, false)
	if code != 403 {
		t.Fatalf("CSRF'siz istek kodu: %d", code)
	}
	// CSRF'li → 201 (fake manager).
	code, _ = c.post("/api/v1/sites", map[string]string{"domain": "x.com", "php_version": "8.3"}, true)
	if code != 201 {
		t.Fatalf("site oluşturma kodu: %d", code)
	}
}

func TestForcedPasswordChangeGate(t *testing.T) {
	ts, st, _ := testServer(t)
	ctx := context.Background()
	roleID, _ := st.GetRoleIDByName(ctx, "admin")
	hash, _ := auth.HashPassword("varsayilan-12345")
	st.InsertUser(ctx, store.User{
		Username: "admin-new", PasswordHash: hash, RoleID: roleID,
		MustChangePassword: true, Status: "active",
	})
	c := newClient(t, ts.URL)
	code, out := c.post("/api/v1/auth/login", map[string]string{"username": "admin-new", "password": "varsayilan-12345"}, false)
	if code != 200 || out["must_change_password"] != true {
		t.Fatalf("giriş: %d %v", code, out)
	}
	c.csrf, _ = out["csrf_token"].(string)

	// Şifre değiştirilmeden site işlemi → 403.
	if code, _ := c.post("/api/v1/sites", map[string]string{"domain": "x.com"}, true); code != 403 {
		t.Fatalf("zorunlu değişim kapısı çalışmadı: %d", code)
	}
	// Şifre değişimi → 200; ardından işlemler açılır.
	code, _ = c.post("/api/v1/auth/change-password",
		map[string]string{"old_password": "varsayilan-12345", "new_password": "yeni-guclu-12345"}, true)
	if code != 200 {
		t.Fatalf("şifre değişimi: %d", code)
	}
	if code, _ := c.post("/api/v1/sites", map[string]string{"domain": "x.com"}, true); code != 201 {
		t.Fatalf("değişim sonrası işlem: %d", code)
	}
}

func TestPATAuth(t *testing.T) {
	ts, st, _ := testServer(t)
	createAdmin(t, st, "admin-pat", "pat-parola-12345")
	c := newClient(t, ts.URL)
	code, out := c.post("/api/v1/auth/login", map[string]string{"username": "admin-pat", "password": "pat-parola-12345"}, false)
	if code != 200 {
		t.Fatal("giriş başarısız")
	}
	c.csrf, _ = out["csrf_token"].(string)

	// PAT üret.
	code, out = c.post("/api/v1/auth/pat", map[string]string{"name": "cli"}, true)
	if code != 201 {
		t.Fatalf("PAT üretimi: %d %v", code, out)
	}
	token, _ := out["token"].(string)
	if len(token) < 20 {
		t.Fatal("token dönmedi")
	}

	// PAT ile doğrudan istek (cookie'siz).
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("PAT isteği: %d", resp.StatusCode)
	}
}

func TestFileAPIWithOptimisticLock(t *testing.T) {
	ts, st, sitesRoot := testServer(t)
	createAdmin(t, st, "admin-f", "f-parola-123456")
	c := newClient(t, ts.URL)
	code, out := c.post("/api/v1/auth/login", map[string]string{"username": "admin-f", "password": "f-parola-123456"}, false)
	if code != 200 {
		t.Fatal("giriş başarısız")
	}
	c.csrf, _ = out["csrf_token"].(string)

	// Site root'un gerçek dizini olsun (fm resolve için).
	if err := os.MkdirAll(filepath.Join(filepath.FromSlash(sitesRoot), "site001", "home"), 0o755); err != nil {
		t.Fatal(err)
	}

	// İlk yazma (kilit yok).
	code, _ = c.put("/api/v1/sites/site001/files/content?path=index.html",
		map[string]string{"content_b64": "aGVsbG8="}, true)
	if code != 200 {
		t.Fatalf("yazma: %d", code)
	}
	// Oku: hash + mtime al.
	code, out = c.get("/api/v1/sites/site001/files/content?path=index.html")
	if code != 200 {
		t.Fatalf("okuma: %d %v", code, out)
	}
	hash, _ := out["sha256"].(string)
	mtime, _ := out["modified_at"].(string)

	// Bayat kilit ile yazma → 409.
	code, _ = c.put("/api/v1/sites/site001/files/content?path=index.html",
		map[string]string{"content_b64": "d29ybGQ=", "expected_hash": "0000", "expected_mtime": mtime}, true)
	if code != 409 {
		t.Fatalf("bayat kilit: %d", code)
	}
	// Güncel kilit ile yazma → 200.
	code, _ = c.put("/api/v1/sites/site001/files/content?path=index.html",
		map[string]string{"content_b64": "d29ybGQ=", "expected_hash": hash, "expected_mtime": mtime}, true)
	if code != 200 {
		t.Fatalf("güncel kilit: %d", code)
	}

	// Traversal denemesi → 403.
	code, _ = c.get("/api/v1/sites/site001/files?path=../../etc")
	if code != 403 {
		t.Fatalf("traversal: %d", code)
	}
}

// fakePrivReq, sahte priv sunucusunun yakaladığı tek istek.
type fakePrivReq struct {
	op   string
	args map[string]any
}

// startFakePriv, JSON satır protokolünü konuşan tek op'luk sahte priv
// sunucusu başlatır; gelen istekleri ch kanalına bırakır.
func startFakePriv(t *testing.T, ch chan<- fakePrivReq) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "priv.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var req priv.Request
				if err := json.NewDecoder(c).Decode(&req); err != nil {
					return
				}
				var args map[string]any
				_ = json.Unmarshal(req.Args, &args)
				select {
				case ch <- fakePrivReq{op: req.Op, args: args}:
				default:
				}
				c.Write([]byte(`{"ok":true,"data":{}}`))
			}(conn)
		}
	}()
	return sock
}

// TestPasswordChangeSyncsOlsWebAdmin: şifre değişiminde panel, OLS WebAdmin
// tek giriş çiftini helper'a senkronlar (ARCHITECTURE §9.10).
func TestPasswordChangeSyncsOlsWebAdmin(t *testing.T) {
	ch := make(chan fakePrivReq, 4)
	sock := startFakePriv(t, ch)
	ts, st, _ := testServerWithPriv(t, sock)
	createAdmin(t, st, "admin-sync", "eski-parola-12345")
	c := newClient(t, ts.URL)

	code, out := c.post("/api/v1/auth/login",
		map[string]string{"username": "admin-sync", "password": "eski-parola-12345"}, false)
	if code != 200 {
		t.Fatalf("giriş: %d %v", code, out)
	}
	c.csrf, _ = out["csrf_token"].(string)

	code, _ = c.post("/api/v1/auth/change-password",
		map[string]string{"old_password": "eski-parola-12345", "new_password": "yeni-guclu-12345"}, true)
	if code != 200 {
		t.Fatalf("şifre değişimi: %d", code)
	}

	select {
	case req := <-ch:
		if req.op != "ols.webadmin_credentials" {
			t.Fatalf("op: %q", req.op)
		}
		if req.args["username"] != "admin-sync" {
			t.Fatalf("username: %v", req.args["username"])
		}
		if req.args["password"] != "yeni-guclu-12345" {
			t.Fatalf("password: %v", req.args["password"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("helper'a senkron isteği gitmedi")
	}
}
