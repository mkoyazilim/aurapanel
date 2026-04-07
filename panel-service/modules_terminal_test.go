package main

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"golang.org/x/net/websocket"
)

func TestTerminalWebsocketHandshakeAllowsForwardedHost(t *testing.T) {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8081/api/v1/terminal/ws", nil)
	req.Host = "127.0.0.1:8081"
	req.Header.Set("X-Forwarded-Host", "panel.example.com")

	cfg := &websocket.Config{
		Origin: mustParseURL(t, "https://panel.example.com"),
	}

	if err := terminalWebsocketHandshake(cfg, req); err != nil {
		t.Fatalf("expected handshake to allow forwarded host, got error: %v", err)
	}
}

func TestTerminalWebsocketHandshakeRejectsMismatchedOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8081/api/v1/terminal/ws", nil)
	req.Host = "127.0.0.1:8081"
	req.Header.Set("X-Forwarded-Host", "panel.example.com")

	cfg := &websocket.Config{
		Origin: mustParseURL(t, "https://evil.example.net"),
	}

	if err := terminalWebsocketHandshake(cfg, req); err == nil {
		t.Fatalf("expected handshake to reject mismatched origin")
	}
}

func TestTerminalWebsocketHandshakeAllowsMissingOrigin(t *testing.T) {
	req := httptest.NewRequest("GET", "http://127.0.0.1:8081/api/v1/terminal/ws", nil)
	req.Host = "127.0.0.1:8081"

	if err := terminalWebsocketHandshake(&websocket.Config{}, req); err != nil {
		t.Fatalf("expected handshake to allow missing origin, got error: %v", err)
	}
}

func TestParseHostPortForCompare(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantHost string
		wantPort string
	}{
		{name: "host only", input: "Panel.Example.Com", wantHost: "panel.example.com", wantPort: ""},
		{name: "host with port", input: "panel.example.com:443", wantHost: "panel.example.com", wantPort: "443"},
		{name: "scheme host", input: "https://panel.example.com:8443", wantHost: "panel.example.com", wantPort: "8443"},
		{name: "ipv6 with port", input: "[2001:db8::1]:443", wantHost: "2001:db8::1", wantPort: "443"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotHost, gotPort := parseHostPortForCompare(tc.input)
			if gotHost != tc.wantHost || gotPort != tc.wantPort {
				t.Fatalf("parseHostPortForCompare(%q) = (%q,%q), want (%q,%q)", tc.input, gotHost, gotPort, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse url %q: %v", raw, err)
	}
	return parsed
}
