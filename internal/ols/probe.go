package ols

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// HTTPProber, loopback'e yapılan gerçek HTTP sağlık kontrollerini yürütür.
type HTTPProber struct {
	Timeout time.Duration
}

// Probe, spec'teki adrese istek atar. 5xx dışındaki her yanıt "vhost
// hizmet veriyor" kabul edilir (404 dahil — vhost canlı demektir).
//
// TLS doğrulaması YALNIZCA loopback hedeflerde atlanır; sertifikalar
// domaine bağlıdır, 127.0.0.1'e değil. Loopback dışı hedeflerde tam
// doğrulama yapılır — bu bir güvenlik değişmezidir.
func (h *HTTPProber) Probe(ctx context.Context, spec ProbeSpec) error {
	if spec.Path == "" {
		spec.Path = "/"
	}
	host, _, err := net.SplitHostPort(spec.Addr)
	if err != nil {
		return fmt.Errorf("probe adresi geçersiz %q: %w", spec.Addr, err)
	}
	ip := net.ParseIP(host)
	loopback := ip != nil && ip.IsLoopback()

	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	client := &http.Client{Timeout: timeout}
	tr := &http.Transport{TLSClientConfig: &tls.Config{}}
	if spec.TLS && loopback {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	client.Transport = tr

	scheme := "http"
	if spec.TLS {
		scheme = "https"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, scheme+"://"+spec.Addr+spec.Path, nil)
	if err != nil {
		return fmt.Errorf("probe isteği: %w", err)
	}
	req.Host = spec.Host

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s (%s): %w", spec.Host, spec.Addr, err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 500 {
		return fmt.Errorf("probe %s: HTTP %d", spec.Host, resp.StatusCode)
	}
	return nil
}
