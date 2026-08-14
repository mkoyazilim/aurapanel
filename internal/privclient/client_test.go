package privclient

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/priv"
)

// unix test sunucusu: tek bağlantı kabul eder, handler'ı çalıştırır.
func serveUnix(t *testing.T, handler func(priv.Request) priv.Response) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "test.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req priv.Request
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			return
		}
		b, _ := json.Marshal(handler(req))
		conn.Write(append(b, '\n'))
	}()
	return sock
}

func TestCallRoundTrip(t *testing.T) {
	ctx := context.Background()
	sock := serveUnix(t, func(req priv.Request) priv.Response {
		if req.Op != "priv.ping" {
			t.Errorf("op hatalı: %s", req.Op)
		}
		if req.RequestID == "" {
			t.Error("request_id gönderilmeli")
		}
		return priv.Response{OK: true, Data: map[string]any{"pong": true}, RequestID: req.RequestID}
	})
	c := New(sock, 3*time.Second)
	data, err := c.Call(ctx, "priv.ping", map[string]any{})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if data["pong"] != true {
		t.Fatalf("veri hatalı: %v", data)
	}
}

func TestCallErrorResponse(t *testing.T) {
	ctx := context.Background()
	sock := serveUnix(t, func(req priv.Request) priv.Response {
		return priv.Response{OK: false, Error: "bilinmeyen op", RequestID: req.RequestID}
	})
	c := New(sock, 3*time.Second)
	_, err := c.Call(ctx, "shell.exec", nil)
	if err == nil {
		t.Fatal("hata bekleniyordu")
	}
}

func TestCallTimeout(t *testing.T) {
	ctx := context.Background()
	sock := serveUnix(t, func(req priv.Request) priv.Response {
		time.Sleep(2 * time.Second)
		return priv.Response{OK: true}
	})
	c := New(sock, 300*time.Millisecond)
	if _, err := c.Call(ctx, "priv.ping", nil); err == nil {
		t.Fatal("zaman aşımı bekleniyordu")
	}
}

func TestCallNoServer(t *testing.T) {
	c := New(filepath.Join(t.TempDir(), "yok.sock"), time.Second)
	if _, err := c.Call(context.Background(), "priv.ping", nil); err == nil {
		t.Fatal("bağlantı hatası bekleniyordu")
	}
}
