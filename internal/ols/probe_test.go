package ols

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Loopback TLS sunucusuna probe: skip-verify istisnasıyla başarılı olmalı.
func TestProbeLoopbackTLSSuccess(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	p := &HTTPProber{Timeout: 3 * time.Second}
	err := p.Probe(context.Background(), ProbeSpec{
		Addr: srv.Listener.Addr().String(),
		TLS:  true,
		Host: "example.com",
		Path: "/",
	})
	if err != nil {
		t.Fatalf("probe başarısız: %v", err)
	}
}

// Kapalı sunucuya probe hata dönmeli (rollback tetikleyicisi).
func TestProbeFailure(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := srv.Listener.Addr().String()
	srv.Close()

	p := &HTTPProber{Timeout: 1 * time.Second}
	if err := p.Probe(context.Background(), ProbeSpec{Addr: addr, TLS: true, Host: "x.com", Path: "/"}); err == nil {
		t.Fatal("kapalı sunucuya probe başarılı dönmemeli")
	}
}

// 5xx yanıt da başarısızlık sayılmalı.
func TestProbeServerError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	p := &HTTPProber{Timeout: 3 * time.Second}
	if err := p.Probe(context.Background(), ProbeSpec{Addr: srv.Listener.Addr().String(), TLS: true, Host: "x.com", Path: "/"}); err == nil {
		t.Fatal("5xx yanıt başarısızlık sayılmadı")
	}
}

// 404 yanıt vhost canlı demektir — başarılı sayılmalı.
func TestProbeNotFoundStillAlive(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	p := &HTTPProber{Timeout: 3 * time.Second}
	if err := p.Probe(context.Background(), ProbeSpec{Addr: srv.Listener.Addr().String(), TLS: true, Host: "x.com", Path: "/"}); err != nil {
		t.Fatalf("404 yanıt başarısız sayıldı: %v", err)
	}
}
