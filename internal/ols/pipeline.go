package ols

import (
	"context"
	"fmt"
	"sync"
)

// Installer, OLS config değişikliklerini uygulayan arayüz (priv helper üzerinden).
type Installer interface {
	ReadBundle(ctx context.Context, site string) ([]Artifact, error)
	InstallBundle(ctx context.Context, site string, files []Artifact) error
	RemoveBundle(ctx context.Context, site string, names []string) error
	TestConfig(ctx context.Context) error
	Reload(ctx context.Context) error
}

// Prober, uygulanan değişikliğin gerçekten çalıştığını doğrular.
type Prober interface {
	Probe(ctx context.Context, spec ProbeSpec) error
}

// ProbeSpec, tek bir HTTP sağlık kontrolü.
type ProbeSpec struct {
	Addr string // 127.0.0.1:443
	TLS  bool
	Host string
	Path string
}

// bundleFileNames, bir site bundle'ında bulunabilecek dosya adları
// (priv helper'daki olsFileAllowlist ile birebir eşleşir).
var bundleFileNames = []string{"vhconf.conf"}

// Pipeline, ARCHITECTURE §5.1 apply akışını yürütür:
//
//	render+validate → snapshot → install → OLS config testi → reload
//	→ health check → (başarısızlıkta) OTOMATİK ROLLBACK + doğrulama
type Pipeline struct {
	sitesRoot string
	certsRoot string
	installer Installer
	prober    Prober

	mu sync.Mutex // OLS'e eş zamanlı müdahale yasak
}

// NewPipeline, Pipeline oluşturur.
func NewPipeline(sitesRoot, certsRoot string, installer Installer, prober Prober) *Pipeline {
	return &Pipeline{sitesRoot: sitesRoot, certsRoot: certsRoot, installer: installer, prober: prober}
}

// Apply, bir vhost desired state'ini güvenle uygular.
// Herhangi bir aşama başarısız olursa önceki duruma dönülür ve
// dönüş (TestConfig + Reload) doğrulanır.
func (p *Pipeline) Apply(ctx context.Context, v Vhost) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	artifacts, err := RenderVhost(p.sitesRoot, p.certsRoot, v)
	if err != nil {
		return fmt.Errorf("render/validate: %w", err)
	}

	// Değişiklik öncesi mevcut durum snapshot'ı (İlke 5-6).
	snapshot, err := p.installer.ReadBundle(ctx, v.SiteID)
	if err != nil {
		return fmt.Errorf("snapshot alınamadı (değişiklik yapılmadı): %w", err)
	}

	if err := p.installer.InstallBundle(ctx, v.SiteID, artifacts); err != nil {
		return composeErr("install", err, p.rollback(ctx, v.SiteID, snapshot))
	}
	if err := p.installer.TestConfig(ctx); err != nil {
		return composeErr("ols config doğrulaması", err, p.rollback(ctx, v.SiteID, snapshot))
	}
	if err := p.installer.Reload(ctx); err != nil {
		return composeErr("reload", err, p.rollback(ctx, v.SiteID, snapshot))
	}
	// Prober bağlıysa health check yapılır; bağlı değilse doğrulama
	// config testi + reload ile sınırlıdır (ör. drift onarımında doğrulama
	// sonraki taramadaki içerik karşılaştırmasıyla yapılır).
	if p.prober != nil {
		if err := p.prober.Probe(ctx, p.probeFor(v)); err != nil {
			return composeErr("health check", err, p.rollback(ctx, v.SiteID, snapshot))
		}
	}
	return nil
}

// probeFor, vhost için loopback sağlık probunu üretir.
func (p *Pipeline) probeFor(v Vhost) ProbeSpec {
	spec := ProbeSpec{Addr: "127.0.0.1:80", TLS: false, Host: v.Domain, Path: "/"}
	if v.SSL != nil {
		spec.Addr = "127.0.0.1:443"
		spec.TLS = true
	}
	return spec
}

// composeErr, asıl hatayı rollback sonucuyla birlikte raporlar.
// Rollback doğrulaması da başarısızsa bu KRİTİK bir durumdur ve
// hata mesajında açıkça belirtilir.
func composeErr(stage string, err, rbErr error) error {
	if rbErr != nil {
		return fmt.Errorf("%s başarısız: %w (AYRICA ROLLBACK BAŞARISIZ: %v)", stage, err, rbErr)
	}
	return fmt.Errorf("%s başarısız: %w (rollback uygulandı ve doğrulandı)", stage, err)
}

// Remove, bir sitenin vhost'unu güvenle kaldırır: snapshot → dosyaları
// kaldır → config testi → reload; herhangi bir aşama başarısız olursa
// snapshot geri yüklenir (site hizmette kalır).
func (p *Pipeline) Remove(ctx context.Context, site string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	snapshot, err := p.installer.ReadBundle(ctx, site)
	if err != nil {
		return fmt.Errorf("snapshot alınamadı (değişiklik yapılmadı): %w", err)
	}

	names := append([]string{}, bundleFileNames...)
	if err := p.installer.RemoveBundle(ctx, site, names); err != nil {
		return composeErr("vhost kaldırma", err, p.rollback(ctx, site, snapshot))
	}
	if err := p.installer.TestConfig(ctx); err != nil {
		return composeErr("ols config doğrulaması", err, p.rollback(ctx, site, snapshot))
	}
	if err := p.installer.Reload(ctx); err != nil {
		return composeErr("reload", err, p.rollback(ctx, site, snapshot))
	}
	return nil
}

// rollback, snapshot'ı geri yükler: önceki içerikler yazılır, yeni
// oluşturulan dosyalar silinir, ardından config testi + reload ile
// eski hâlin çalışır durumda olduğu doğrulanır.
func (p *Pipeline) rollback(ctx context.Context, site string, snapshot []Artifact) error {
	snapNames := map[string]bool{}
	for _, a := range snapshot {
		snapNames[a.RelPath] = true
	}

	if err := p.installer.InstallBundle(ctx, site, snapshot); err != nil {
		return fmt.Errorf("snapshot geri yazılamadı: %w", err)
	}
	var toRemove []string
	for _, n := range bundleFileNames {
		if !snapNames[n] {
			toRemove = append(toRemove, n)
		}
	}
	if len(toRemove) > 0 {
		if err := p.installer.RemoveBundle(ctx, site, toRemove); err != nil {
			return fmt.Errorf("yeni dosyalar kaldırılamadı: %w", err)
		}
	}
	if err := p.installer.TestConfig(ctx); err != nil {
		return fmt.Errorf("rollback sonrası config testi: %w", err)
	}
	if err := p.installer.Reload(ctx); err != nil {
		return fmt.Errorf("rollback sonrası reload: %w", err)
	}
	return nil
}
