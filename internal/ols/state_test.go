package ols

import (
	"testing"
)

func validVhost() Vhost {
	return Vhost{
		SiteID:     "site001",
		Domain:     "example.com",
		Aliases:    []string{"www.example.com"},
		PHPVersion: "8.3",
		IndexFiles: []string{"index.php", "index.html"},
	}
}

func TestValidateDomainTable(t *testing.T) {
	good := []string{"example.com", "www.example.com", "a.co", "xn--rnek-9za.com", "sub.domain.example.org"}
	bad := []string{"", "EXAMPLE.com", "example", "-example.com", "example..com", "example.c", "exa mple.com", "example.com/", "*.example.com", "example.com; rm -rf"}
	for _, d := range good {
		v := validVhost()
		v.Domain = d
		v.Aliases = nil // domain testinde alias çakışması olmamalı
		if err := v.Validate("/srv/aurapanel/sites", "/srv/aurapanel/state/certs"); err != nil {
			t.Errorf("geçerli domain reddedildi %q: %v", d, err)
		}
	}
	for _, d := range bad {
		v := validVhost()
		v.Domain = d
		v.Aliases = nil
		if err := v.Validate("/srv/aurapanel/sites", "/srv/aurapanel/state/certs"); err == nil {
			t.Errorf("geçersiz domain kabul edildi: %q", d)
		}
	}
}

func TestValidateMisc(t *testing.T) {
	// Desteklenmeyen PHP
	v := validVhost()
	v.PHPVersion = "8.9"
	if err := v.Validate("/srv/aurapanel/sites", "/srv/aurapanel/state/certs"); err == nil {
		t.Error("desteklenmeyen PHP kabul edildi")
	}
	// Yinelenen domain/alias
	v = validVhost()
	v.Aliases = []string{"example.com"}
	if err := v.Validate("/srv/aurapanel/sites", "/srv/aurapanel/state/certs"); err == nil {
		t.Error("yinelenen alias kabul edildi")
	}
	// Sertifika yolu cert deposu dışında
	v = validVhost()
	v.SSL = &SSLConfig{CertPath: "/etc/letsencrypt/live/x/fullchain.pem", KeyPath: "/srv/aurapanel/state/certs/x/privkey.pem"}
	if err := v.Validate("/srv/aurapanel/sites", "/srv/aurapanel/state/certs"); err == nil {
		t.Error("cert deposu dışındaki sertifika yolu kabul edildi")
	}
	// Sertifika yolu geçerli
	v = validVhost()
	v.SSL = &SSLConfig{
		CertPath: "/srv/aurapanel/state/certs/example.com/fullchain.pem",
		KeyPath:  "/srv/aurapanel/state/certs/example.com/privkey.pem",
	}
	if err := v.Validate("/srv/aurapanel/sites", "/srv/aurapanel/state/certs"); err != nil {
		t.Errorf("geçerli SSL reddedildi: %v", err)
	}
}
