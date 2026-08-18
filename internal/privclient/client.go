// Package privclient, panelin aurapanel-priv helper'ıyla konuşan
// istemcisidir (ARCHITECTURE §3.1: Unix socket + JSON satır protokolü).
package privclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/mkoyazilim/aurapanel/internal/priv"
)

// maxResponseSize, yanıt okuma sınırı: en büyük file.op read yanıtını
// (16 MiB içerik, base64 ≈ 22.4 MB) karşılayacak kadar geniş.
const maxResponseSize = 24 << 20

// Client, helper'a tek op'luk çağrılar yapan istemci.
type Client struct {
	sockPath string
	timeout  time.Duration
}

// New, Client oluşturur. sockPath: /run/aurapanel/priv.sock.
func New(sockPath string, timeout time.Duration) *Client {
	return &Client{sockPath: sockPath, timeout: timeout}
}

// Call, tek bir op çağırır; ok=false yanıtları error olarak döner.
func (c *Client) Call(ctx context.Context, op string, args map[string]any) (map[string]any, error) {
	req := priv.Request{Op: op, RequestID: newRequestID()}
	if args != nil {
		b, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("args kodlama: %w", err)
		}
		req.Args = b
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')

	d := net.Dialer{Timeout: c.timeout}
	conn, err := d.DialContext(ctx, "unix", c.sockPath)
	if err != nil {
		return nil, fmt.Errorf("priv helper'a bağlanılamadı: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, err
	}

	if _, err := conn.Write(payload); err != nil {
		return nil, fmt.Errorf("istek yazılamadı: %w", err)
	}
	b, err := io.ReadAll(io.LimitReader(conn, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("yanıt okunamadı: %w", err)
	}

	var resp priv.Response
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("yanıt çözümlenemedi: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("priv op %q: %s", op, resp.Error)
	}
	data, _ := resp.Data.(map[string]any)
	return data, nil
}

func newRequestID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
