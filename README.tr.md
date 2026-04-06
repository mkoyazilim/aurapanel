# AuraPanel

<p align="right">
  <a href="./README.md">English</a> | Turkce
</p>

AuraPanel, modern hosting operasyonları için geliştirilen performans odaklı, güvenlik merkezli ve operasyonel olarak dürüst bir kontrol düzlemidir.

Temel üç ilke:

- `Performans`: düşük overhead, deterministik otomasyon
- `Stabilite`: kontrol düzlemi ile sunum düzlemi ayrık
- `Güvenlik`: fail-closed API tasarımı ve sıkı RBAC

## İçindekiler

- [Vizyon](#vizyon)
- [Neden AuraPanel](#neden-aurapanel)
- [Mimari](#mimari)
- [Web Sunucu Stratejisi](#web-sunucu-stratejisi)
- [Özellik Yüzeyi](#özellik-yüzeyi)
- [Güvenlik Modeli](#güvenlik-modeli)
- [Performans Modeli](#performans-modeli)
- [Karşılaştırma Özeti](#karşılaştırma-özeti)
- [Doküman ve Wiki](#doküman-ve-wiki)
- [Production Kurulum](#production-kurulum)
- [Yerel Geliştirme](#yerel-geliştirme)
- [Build ve Paketleme](#build-ve-paketleme)
- [Repo Yapısı](#repo-yapısı)
- [Yol Haritası](#yol-haritası)
- [Katkı Prensipleri](#katkı-prensipleri)
- [Lisans](#lisans)

## Vizyon

AuraPanel, shell komutlarının üstüne yerleştirilmiş bir arayüz değildir.

Amaç, çok kiracılı hosting ekipleri ve altyapı operasyon ekipleri için:

- yüksek görünürlük
- ölçülebilir güvenlik
- düşük sürtünmeli operasyon
- üretim ortamında güvenilir otomasyon

sunmaktır.

Ana prensip: panel yeniden başlasa bile web siteleri çalışmaya devam etmelidir.

## Neden AuraPanel

AuraPanel operasyonel dürüstlük ilkesine dayanır:

- Gerçek host/API/config bağı olmayan özellikler aktif gibi gösterilmez.
- Desteklenmeyen akışlar sahte başarı yerine `501 Not Implemented` döner.
- Smoke/acceptance kontrolleri operasyonun doğal parçasıdır.

## Mimari

```text
Tarayıcı
  -> Vue Frontend
  -> Go API Gateway
  -> Go Panel Service
  -> Host Servisleri / Entegrasyonlar
     - OpenLiteSpeed
     - MariaDB
     - PostgreSQL
     - Postfix
     - Dovecot
     - Pure-FTPd
     - PowerDNS
     - Redis
     - MinIO
     - Docker
     - WP-CLI
     - Cloudflare
```

### Kontrol Düzlemi Katmanları

`frontend/`
- Vue + Vite operatör arayüzü
- workflow odaklı yönetim ekranları

`api-gateway/`
- kimlik doğrulanmış trafiğin merkezi girişi
- JWT, middleware, request-id, CORS, RBAC
- panel-service için kontrollü proxy

`panel-service/`
- host otomasyonu ve runtime orkestrasyonu
- provisioning, tuning, hardening, backup ve migration uçları

## Web Sunucu Stratejisi

AuraPanel web katmanında net bir strateji uygular:

- `OpenLiteSpeed (birincil)`: panel akışlarında ana ve resmi entegrasyon hedefidir.
- `Nginx (edge uyumlu)`: OpenLiteSpeed'in önünde opsiyonel reverse proxy edge olarak TLS termination, cache ve istek şekillendirme için uyumludur.
- `Apache`: proje kapsamı dışındadır.

Apache modu, projenin modern performans, operasyonel tutarlılık ve platform yönü standartlarıyla uyumlu olmadığı için AuraPanel yol haritasına dahil edilmemiştir.

Varsayilan kurulum modu `ols-only` modudur. Ilk kurulumda edge mod ile baslatmak icin:

```bash
export AURAPANEL_WEB_STACK_MODE=nginx-edge
```

degiskenini ayarlayip installer'i calistirin. Modu daha sonra `Panel Kontrol -> Web Stack` ekranindan da degistirebilirsiniz.

## Özellik Yüzeyi

Gerçek entegrasyonlar:

- Website yaşam döngüsü: domain onboarding, vhost sync, rewrite ve htaccess
- SSL/TLS: issuance, custom cert, wildcard binding
- DNS: PowerDNS zone/record yönetimi
- Mail stack: Postfix + Dovecot, mailbox ve forward akışları
- FTP/SFTP yönetimi
- MariaDB/PostgreSQL provisioning ve tuning
- Backup akışları ve MinIO hedef desteği
- Docker runtime yönetimi
- Cloudflare entegrasyon yüzeyleri
- `wp-cli` ile WordPress yönetimi
- Malware scan/quarantine
- Firewall ve SSH key yönetimi
- Panel port ve servis görünürlüğü
- Migration upload/analysis/import

Detaylı runtime durum özeti için: [ENDPOINT_AUDIT.md](./ENDPOINT_AUDIT.md)

## Güvenlik Modeli

AuraPanel zero-trust ve fail-closed yaklaşımı uygular:

- tüm korumalı isteklerde kimlik doğrulama
- gateway seviyesinde RBAC zorlaması
- sahte başarı yanıtlarından kaçınma
- kontrollü izinlerle environment/runtime dosya üretimi
- manifest + hash doğrulamalı release bootstrap
- yalnızca gerekli portları açan firewall otomasyonu
- credential üretimi sonrası smoke doğrulama
- ModSecurity + OWASP CRS desteği

## Performans Modeli

- `Ayrık sunum yolu`: siteler OpenLiteSpeed üzerinden servis edilir
- `Go servisleri`: öngörülebilir kaynak kullanımı
- `Odaklı proxy`: `/api/v1/` trafiğinde minimal katman
- `Deterministik host entegrasyonları`
- `Operasyonel izolasyon`: panel restart = site kesintisi değildir

## Karşılaştırma Özeti

Bu tablo fiyat/lisans değil, teknik konumlandırma odaklıdır.

| Alan | AuraPanel | CyberPanel | cPanel/WHM | Plesk |
|---|---|---|---|---|
| Temel yaklaşım | Ayrık kontrol düzlemi + runtime dürüstlüğü | OLS merkezli hızlı kurulum | Ticari shared hosting standardı | Geniş extension ekosistemi |
| Sunum/Kontrol ayrımı | Güçlü odak | Kuruluma göre değişebilir | Çoğunlukla entegre akışlar | Entegre + extension tabanlı |
| Runtime şeffaflığı | Doğrulanabilir host-backed endpoint yaklaşımı | Modüle göre değişken | Soyutlama yüksek | Soyutlama + extension katmanı |
| Güvenlik hedefi | Zero-trust + fail-closed varsayılanlar | Temel hardening araçları | Ticari katmanlarda güçlü | Ticari/extension güvenlik setleri |
| Extensibility yönü | API/gRPC-first, GitOps uyumlu | Plugin odaklı | Ticari entegrasyon pazarı | Extension odaklı |
| Operasyon felsefesi | Deterministik otomasyon | Hız ve pratiklik | Pazar olgunluğu | Geniş uyumluluk |

Detaylı analiz: [Wiki Comparisons](./wiki/Comparisons.md)

## Doküman ve Wiki

### Teknik Dokümanlar

- [Documentation Index](./docs/documentation-index.md)
- [API Contract v1](./docs/api_contract_v1.md)
- [Final System Audit (2026-03-30)](./docs/final-system-audit-2026-03-30.md)
- [Product Overview](./docs/product-overview.md)
- [Hosting Panel Comparison](./docs/hosting-panel-comparison.md)
- [Endpoint Audit](./ENDPOINT_AUDIT.md)
- [Changelog](./CHANGELOG.md)

### Wiki Sayfaları (GitHub Wiki Seed)

- [Wiki Home](./wiki/Home.md)
- [Install Guide](./wiki/Install-Guide.md)
- [Architecture](./wiki/Architecture.md)
- [Security Model](./wiki/Security-Model.md)
- [Performance Model](./wiki/Performance-Model.md)
- [Operations Runbook](./wiki/Operations-Runbook.md)
- [Migration Guide](./wiki/Migration-Guide.md)
- [Comparisons](./wiki/Comparisons.md)
- [FAQ](./wiki/FAQ.md)
- [Troubleshooting](./wiki/Troubleshooting.md)

Bu sayfaları GitHub Wiki'ye yayınlamak için:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/publish-wiki.ps1
```

## Production Kurulum

### Desteklenen Hedefler

- Ubuntu `22.04` ve `24.04`
- Debian `12+`
- AlmaLinux `8/9`
- Rocky Linux `8/9`

### Standart Uzak Kurulum

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
```

### Acil Mirror Bypass (Installer)

Mirror güncel değilse sadece installer giriş dosyalarını GitHub `main` üzerinden zorlayabilirsiniz:

```bash
export AURAPANEL_INSTALLER_BASE="https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/installer"
export AURAPANEL_MAIN_INSTALLER_URL="https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/installer/aurapanel.sh"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

### Verified Release Bootstrap

```bash
export AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

### Mevcut Host Güncelleme

```bash
cd /opt/aurapanel
bash scripts/deploy-main.sh
```

### Mirror Senkron Otomasyonu

`main` branch'ine her push'tan sonra mirror'ı otomatik güncelleyen workflow: [mirror-sync.yml](.github/workflows/mirror-sync.yml)

GitHub ayarlarında tanımlanması gerekenler:

- `Secrets`
- `MIRROR_SSH_HOST`
- `MIRROR_SSH_USER`
- `MIRROR_SSH_PRIVATE_KEY`
- `Variables` (opsiyonel)
- `MIRROR_SSH_PORT` (varsayılan: `22`)
- `MIRROR_BASE_DIR` (varsayılan: `/home/downloads.aurapanel.info/public_html/mirror`)
- `MIRROR_OWNER` (örnek: `aura:aura`)
- `MIRROR_PUBLIC_BASE_URL` (varsayılan: `https://downloads.aurapanel.info/mirror`)

Mirror senkronu artık pinlenmiş OpenLiteSpeed paketlerini de yayınlar:

- `deps/litespeed/openlitespeed-manifest.env`
- `deps/litespeed/pinned/*`

Installer davranışı:

- Önce mirror'daki pinli OLS paketini dener (`AURAPANEL_OLS_USE_MIRROR_PACKAGE=1` varsayılan)
- Pinli paket uygun değilse repository kurulumuna fallback yapar
- Manifest URL'i `AURAPANEL_OLS_MANIFEST_URL` ile override edilebilir

## Yerel Geliştirme

Gereksinimler:

- Go `1.22+`
- Node.js `20+`

Windows helper:

```powershell
.\start-dev.ps1
```

Varsayılan yerel endpointler:

- Frontend: `http://127.0.0.1:5173`
- Gateway: `http://127.0.0.1:8090`
- Panel Service: `http://127.0.0.1:8081`

Varsayılan test girişi:

- Email: `admin@server.com`
- Şifre: `password123`

## Build ve Paketleme

```bash
make build
make package
make clean
```

## Repo Yapısı

```text
aurapanel/
|-- api-gateway/
|-- panel-service/
|-- frontend/
|-- web-site/
|-- installer/
|-- docs/
|-- wiki/
|-- install.sh
`-- ENDPOINT_AUDIT.md
```

## Yol Haritası

Yakın dönem odakları:

- servisler arası güven modeli sıkılaştırma
- eBPF tabanlı telemetri ve drift detection
- GitOps döngülerinin güçlendirilmesi
- cPanel/Plesk/CyberPanel migration yardımcılarının derinleştirilmesi
- operasyonel analytics + öneri motoru

## Katkı Prensipleri

- runtime iddialarını dürüst tut
- simülasyon yerine gerçek entegrasyonlara öncelik ver
- ölçülebilir fayda yoksa ağır bağımlılıktan kaçın
- kontrol düzlemi ve sunum düzlemi ayrımını koru
- host otomasyonunu production-grade altyapı kodu olarak ele al

## Lisans

AuraPanel, [MIT License](./LICENSE) ile dağıtılır.

## Geliştirici

Mkoyazilim ([www.mkoyazilim.com](https://www.mkoyazilim.com)) ve Tahamada
