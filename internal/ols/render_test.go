package ols

import (
	"strings"
	"testing"
)

const (
	testSitesRoot = "/srv/aurapanel/sites"
	testCertsRoot = "/srv/aurapanel/state/certs"
)

func TestRenderContainsCore(t *testing.T) {
	v := validVhost()
	artifacts, err := RenderVhost(testSitesRoot, testCertsRoot, v)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(artifacts) != 1 || artifacts[0].RelPath != "vhconf.conf" {
		t.Fatalf("tek vhconf.conf bekleniyordu: %+v", artifacts)
	}
	c := string(artifacts[0].Content)

	checks := []string{
		"docRoot                   /srv/aurapanel/sites/site001/home/",
		"vhDomain                  example.com",
		"vhAliases                 www.example.com",
		"add                     lsapi:lsphp83 php",
		"extUser                 www-site001",
		"extGroup                www-site001",
		"path                    /usr/local/lsws/lsphp83/bin/lsphp",
		"indexFiles              index.php, index.html",
		"errorlog /srv/aurapanel/sites/site001/logs/error.log",
	}
	for _, want := range checks {
		if !strings.Contains(c, want) {
			t.Errorf("çıktıda bulunamadı: %q", want)
		}
	}
	// SSL kapalıyken vhssl bloğu OLMAMALI.
	if strings.Contains(c, "vhssl") {
		t.Error("SSL kapalıyken vhssl bloğu üretildi")
	}
	if strings.Contains(c, "rewrite") {
		t.Error("TLSRedirect kapalıyken rewrite bloğu üretildi")
	}
	if strings.Contains(c, "mod_security") {
		t.Error("WAF kapalıyken mod_security bloğu üretildi")
	}
}

func TestRenderSSLAndFlags(t *testing.T) {
	v := validVhost()
	v.SSL = &SSLConfig{
		CertPath: testCertsRoot + "/example.com/fullchain.pem",
		KeyPath:  testCertsRoot + "/example.com/privkey.pem",
	}
	v.TLSRedirect = true
	v.WAF = true
	artifacts, err := RenderVhost(testSitesRoot, testCertsRoot, v)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	c := string(artifacts[0].Content)
	for _, want := range []string{"vhssl", "keyFile", "certFile", "RewriteRule ^(.*)$ https://", "mod_security"} {
		if !strings.Contains(c, want) {
			t.Errorf("çıktıda bulunamadı: %q", want)
		}
	}
}

// Çıktı deterministik olmalı: aynı girdi → aynı çıktı (drift karşılaştırması).
func TestRenderDeterministic(t *testing.T) {
	v := validVhost()
	a1, err := RenderVhost(testSitesRoot, testCertsRoot, v)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := RenderVhost(testSitesRoot, testCertsRoot, v)
	if err != nil {
		t.Fatal(err)
	}
	if string(a1[0].Content) != string(a2[0].Content) {
		t.Fatal("render deterministik değil")
	}
}

// PHP sürümüne göre lsapi adı/soketi değişmeli.
func TestRenderPHPVersionSwitch(t *testing.T) {
	for ver, wantSock := range map[string]string{
		"8.2": "uds://tmp/lshttpd/lsphp82.sock",
		"8.3": "uds://tmp/lshttpd/lsphp83.sock",
		"8.4": "uds://tmp/lshttpd/lsphp84.sock",
	} {
		v := validVhost()
		v.PHPVersion = ver
		a, err := RenderVhost(testSitesRoot, testCertsRoot, v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(a[0].Content), wantSock) {
			t.Errorf("%s için soket beklenmiyor: %s", ver, wantSock)
		}
	}
}
