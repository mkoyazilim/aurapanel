package main

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func liveCertificateDir(domain string) string {
	return filepath.Join("/etc/letsencrypt/live", normalizeDomain(domain))
}

func customCertificateDir(domain string) string {
	return filepath.Join("/etc/letsencrypt/live-custom", normalizeDomain(domain))
}

func findCertificatePair(domain string) (string, string) {
	dirs := []string{liveCertificateDir(domain), customCertificateDir(domain)}
	for _, dir := range dirs {
		cert := filepath.Join(dir, "fullchain.pem")
		key := filepath.Join(dir, "privkey.pem")
		if fileExists(cert) && fileExists(key) {
			return cert, key
		}
	}
	
	parts := strings.Split(domain, ".")
	if len(parts) > 2 {
		rootDomain := strings.Join(parts[len(parts)-2:], ".")
		dirs = []string{liveCertificateDir(rootDomain), customCertificateDir(rootDomain)}
		for _, dir := range dirs {
			cert := filepath.Join(dir, "fullchain.pem")
			key := filepath.Join(dir, "privkey.pem")
			if fileExists(cert) && fileExists(key) {
				raw, err := os.ReadFile(cert)
				if err == nil {
					block, _ := pem.Decode(raw)
					if block != nil {
						parsed, err := x509.ParseCertificate(block.Bytes)
						if err == nil {
							for _, dnsName := range parsed.DNSNames {
								if dnsName == "*."+rootDomain {
									return cert, key
								}
							}
						}
					}
				}
			}
		}
	}
	return "", ""
}

func inspectCertificate(domain string) SSLCertificateDetail {
	certPath, _ := findCertificatePair(domain)
	if certPath == "" {
		return SSLCertificateDetail{Domain: normalizeDomain(domain), Status: "missing", Issuer: "-", ExpiryDate: "-", DaysRemaining: 0}
	}
	raw, err := os.ReadFile(certPath)
	if err != nil {
		return SSLCertificateDetail{Domain: normalizeDomain(domain), Status: "error", Issuer: "-", ExpiryDate: "-", DaysRemaining: 0}
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return SSLCertificateDetail{Domain: normalizeDomain(domain), Status: "error", Issuer: "-", ExpiryDate: "-", DaysRemaining: 0}
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return SSLCertificateDetail{Domain: normalizeDomain(domain), Status: "error", Issuer: "-", ExpiryDate: "-", DaysRemaining: 0}
	}
	return SSLCertificateDetail{
		Domain:        normalizeDomain(domain),
		Status:        "issued",
		Issuer:        cert.Issuer.CommonName,
		ExpiryDate:    cert.NotAfter.UTC().Format("2006-01-02"),
		DaysRemaining: int(time.Until(cert.NotAfter).Hours() / 24),
		Wildcard:      strings.HasPrefix(domain, "*."),
	}
}

func storeCustomCertificate(domain, certPEM, keyPEM string) error {
	dir := customCertificateDir(domain)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "fullchain.pem"), []byte(certPEM), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "privkey.pem"), []byte(keyPEM), 0o600)
}

func panelAdminEmailValue() string {
	return firstNonEmpty(
		readEnvFileValue("/etc/aurapanel/aurapanel.env", "AURAPANEL_ADMIN_EMAIL"),
		strings.TrimSpace(os.Getenv("AURAPANEL_ADMIN_EMAIL")),
		"admin@server.com",
	)
}

func issueLetsEncryptCertificate(domains []string, webroot string, dnsChallenge bool) error {
	args := []string{"certonly", "--non-interactive", "--agree-tos", "-m", panelAdminEmailValue(), "--keep-until-expiring"}
	
	// Use staging API if AURAPANEL_DEV_SIMULATION is enabled to avoid rate limits
	if strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_DEV_SIMULATION"))) == "1" || 
	   strings.ToLower(strings.TrimSpace(os.Getenv("AURAPANEL_DEV_SIMULATION"))) == "true" {
		args = append(args, "--test-cert")
	}

	if dnsChallenge {
		credentialsPath := "/etc/letsencrypt/cloudflare.ini"
		if err := writeCloudflareCertbotCredentials(credentialsPath); err != nil {
			return err
		}
		args = append(args, "--dns-cloudflare", "--dns-cloudflare-credentials", credentialsPath)
	} else {
		// Ensure webroot and well-known directory exists right before execution
		_ = os.MkdirAll(filepath.Join(webroot, ".well-known", "acme-challenge"), 0o755)
		
		// Set webroot path
		args = append(args, "--webroot", "-w", webroot)
	}
	for _, domain := range domains {
		args = append(args, "-d", domain)
	}
	cmd := exec.Command("certbot", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func writeCloudflareCertbotCredentials(path string) error {
	creds := cloudflareEnvCredentials()
	if !creds.valid() {
		return fmt.Errorf("cloudflare credentials are required for wildcard SSL issuance")
	}
	lines := []string{}
	if creds.APIToken != "" {
		lines = append(lines, "dns_cloudflare_api_token = "+creds.APIToken)
	} else {
		lines = append(lines,
			"dns_cloudflare_email = "+creds.Email,
			"dns_cloudflare_api_key = "+creds.APIKey,
		)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
