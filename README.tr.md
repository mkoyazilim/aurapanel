# AuraPanel

<p align="right">
  <a href="./README.md">English</a> | Türkçe
</p>

AuraPanel, hızlı, güvenlik odaklı ve operasyonel olarak dürüst bir hosting kontrol düzlemi arayan operatörler için geliştirilmiş modern bir hosting panelidir.

Platform, ayrık bir mimari etrafında tasarlanmıştır:

- yönetim arayüzü için `Vue 3 + Vite`
- kimlik doğrulama, RBAC, statik panel sunumu ve kontrollü proxy katmanı için `Go API Gateway`
- host otomasyonu, runtime entegrasyonları ve sistem seviyesinde orkestrasyon için `Go Panel Service`
- web sunum katmanı olarak `OpenLiteSpeed`

Temel tasarım hedefi nettir: kontrol düzlemi ile sunum düzlemi birbirinden ayrılmalıdır. Böylece panel yeniden başlatılsa, güncellense veya geçici olarak erişilemez olsa bile web siteleri çalışmaya devam eder.

## Neden AuraPanel

AuraPanel, shell komutlarının üstüne ince bir arayüz eklemek için tasarlanmadı. Gerçek bir hosting platformu olarak şu prensiplerle şekillenmektedir:

- performans öncelikli operasyon tasarımı
- fail-closed güvenlik varsayılanları
- açık ve dürüst runtime davranışı
- deterministik altyapı otomasyonu
- sahte başarı yanıtları yerine gerçek host entegrasyonları

Bir yetenek hosta, harici bir API’ye veya yönetilen bir dosya/konfigürasyon yoluna bağlı değilse aktifmiş gibi sunulmamalıdır.

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
- Vue 3, Vite ve router/store odaklı bir frontend mimarisi ile geliştirilmiş operatör arayüzü
- operasyonel iş akışları, görünürlük ve düşük sürtünmeli host yönetimi için tasarlanmıştır

`api-gateway/`
- kimliği doğrulanmış trafiğin merkezi giriş noktasıdır
- request middleware, JWT doğrulama, rol tabanlı yetkilendirme, CORS, request ID ve servis proxy mantığını uygular
- production ortamında derlenmiş panel arayüzünü sunar

`panel-service/`
- host seviyesinde otomasyonu yürütür ve gerçek runtime aksiyonlarını koordine eder
- website oluşturma, mail provisioning, veritabanı yönetimi, firewall işlemleri, tuning endpoint’leri, backup akışları, runtime app akışları ve servis kontrolünü yönetir

## Performans Yaklaşımı

AuraPanel performans öncelikli bir anlayışla tasarlanmıştır:

- `Ayrık sunum yolu`: web siteleri panel runtime’ı ile değil, OpenLiteSpeed ile servis edilir
- `Go tabanlı kontrol servisleri`: düşük overhead, öngörülebilir açılış süresi ve bellek davranışı
- `Minimal proxy katmanı`: API Gateway, ana `/api/v1/` yüzeyini doğrudan panel-service katmanına iletir
- `Hızlı yerel entegrasyonlar`: sistem aksiyonları ağır orkestrasyon katmanları yerine deterministik CLI, servis ve config bağlarıyla yürütülür
- `Operasyonel izolasyon`: panel yeniden başlatmaları website kesintisi anlamına gelmez
- `Odaklı tuning yüzeyleri`: yüksek etkili tuning yalnızca gerekli alanlarda sunulur; örneğin OpenLiteSpeed, veritabanları, FTP, PHP ve mail stack

## Güvenlik Yaklaşımı

AuraPanel, zero-trust ve fail-closed yaklaşımıyla geliştirilmektedir:

- korumalı tüm istekler kimlik doğrulamadan geçer
- RBAC gateway katmanında uygulanır
- desteklenmeyen endpoint’ler sahte başarı yerine `501 Not Implemented` döndürür
- installer akışı kontrollü izinlerle environment dosyaları üretir
- imzalı manifest doğrulaması ile verified release bootstrap desteklenir
- firewall otomasyonu yalnızca gerekli hosting ve panel portlarını açar
- panel ve servis kimlik bilgileri kurulum sırasında üretilir, senkronize edilir ve smoke-check ile doğrulanır
- ModSecurity ve OWASP CRS entegrasyonu WAF koruması için desteklenir
- SSH key iş akışları, 2FA akışları ve security status endpoint’leri birinci sınıf bileşenlerdir

## Gerçek Runtime Yüzeyi

AuraPanel şu anda aşağıdaki alanlarda gerçek entegrasyonlar içerir:

- website provisioning ve OpenLiteSpeed vhost senkronizasyonu
- `.htaccess` write-through ve OpenLiteSpeed rewrite yönetimi
- PHP sürüm atama ve `php.ini` yönetimi
- MariaDB ve PostgreSQL provisioning, kullanıcı bilgileri, remote access ve tuning
- Postfix ve Dovecot provisioning, mailbox, forward, catch-all ve mail SSL akışları
- Pure-FTPd ve SFTP provisioning
- PowerDNS zone ve record yönetimi
- SSL issuance, custom certificate, wildcard ve hostname binding akışları
- backup, database backup ve dahili MinIO backup target desteği
- Docker runtime ve uygulama yönetimi
- Cloudflare durum ve entegrasyon akışları
- `wp-cli` üzerinden WordPress yönetimi
- malware scan ve quarantine akışları
- firewall ve SSH key yönetimi
- panel port yönetimi ile servis/process görünürlüğü
- migration upload, analiz ve import akışları

Daha net bir runtime durum özeti için [ENDPOINT_AUDIT.md](./ENDPOINT_AUDIT.md) dosyasına bakabilirsiniz.

## Desteklenen Kurulum Hedefleri

Production installer şu işletim sistemlerini hedeflemektedir:

- Ubuntu `22.04` ve `24.04`
- Debian `12+`
- AlmaLinux `8/9`
- Rocky Linux `8/9`

## Production Kurulumu

### 1. Standart Uzak Kurulum

GitHub üzerinden uzak kurulum başlatmanın en basit yolu:

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
```

Bu akış ana installer’ı kullanır ve host üzerinde gerekli runtime stack’i hazırlar.

### 2. Doğrulanmış Release Bootstrap

AuraPanel, imzalı manifest ve SHA-256 doğrulamalı release bundle tabanlı verified bootstrap akışını da destekler.

Örnek:

```bash
export AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

Bootstrap sürecini belirli bir manifest dosyasına da yönlendirebilirsiniz:

```bash
export AURAPANEL_MANIFEST_URL="https://example.com/releases/latest/aurapanel_release_manifest.env"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

### 3. Doğrudan Bootstrap Script Kullanımı

Verified bootstrap aşamasını doğrudan çalıştırmak isterseniz:

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/aurapanel_bootstrap.sh -o aurapanel_bootstrap.sh
chmod +x aurapanel_bootstrap.sh
sudo AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download" ./aurapanel_bootstrap.sh
```

## Production Installer Neleri Kurar

Installer, tam panel host’u kurmak üzere şu bileşenleri hazırlayacak şekilde tasarlanmıştır:

- OpenLiteSpeed
- Node.js 20
- Go toolchain
- MariaDB
- PostgreSQL
- Redis
- Docker
- PowerDNS
- Pure-FTPd
- Postfix
- Dovecot
- MinIO
- Roundcube
- ModSecurity ve OWASP CRS
- WP-CLI
- AuraPanel bileşenleri için systemd servisleri
- firewall temel kuralları
- panel, gateway, OpenLiteSpeed, MinIO ve auth akışları için smoke check’ler

### Oluşturulan systemd Servisleri

Production kurulum şu servisleri oluşturur ve yönetir:

- `aurapanel-service`
- `aurapanel-api`

Host durumuna ve etkin modüllere bağlı olarak AuraPanel şu servislerle de çalışır:

- `lshttpd`
- `mariadb`
- `postgresql`
- `redis` veya `redis-server`
- `postfix`
- `dovecot`
- `pure-ftpd`
- `minio`
- `docker`
- `pdns`

## Yerel Geliştirme

### Gereksinimler

- Go `1.22+`
- Node.js `20+`

### Windows Yardımcı Scripti

Repository içinde tüm yerel stack’i başlatan yardımcı bir script bulunmaktadır:

```powershell
.\start-dev.ps1
```

Varsayılan yerel endpoint’ler:

- Frontend: `http://127.0.0.1:5173`
- Gateway: `http://127.0.0.1:8090`
- Panel Service: `http://127.0.0.1:8081`

Varsayılan development girişi:

- E-posta: `admin@server.com`
- Şifre: `password123`

### Manuel Geliştirme Başlatma

Panel service:

```powershell
cd panel-service
go run .
```

Gateway:

```powershell
cd api-gateway
$env:AURAPANEL_SERVICE_URL='http://127.0.0.1:8081'
go run .
```

Frontend:

```powershell
cd frontend
npm install
npm run dev
```

## Build

Tüm bileşenleri derlemek için:

```bash
make build
```

Release tarball üretmek için:

```bash
make package
```

Artifact temizliği için:

```bash
make clean
```

## Repository Yapısı

```text
aurapanel/
|-- api-gateway/        # Go API Gateway
|-- panel-service/      # Go host otomasyonu ve runtime orkestrasyonu
|-- frontend/           # Vue 3 + Vite kontrol paneli
|-- installer/          # Production kurulum mantığı
|-- docs/               # Yardımcı teknik dokümantasyon
|-- aurapanel_bootstrap.sh
|-- aurapanel_installer.sh
|-- install.sh
|-- start-dev.ps1
|-- Makefile
`-- ENDPOINT_AUDIT.md
```

## Operasyonel Prensipler

AuraPanel birkaç temel prensipten taviz vermez:

- `Kontrol düzlemi != sunum düzlemi`
- `Kozmetik tamlık yerine operasyonel dürüstlük`
- `Konfor yerine güvenlik varsayılanları`
- `Kırılgan gizli state yerine deterministik otomasyon`
- `Performans hassas yollar mümkün olduğunca sade kalmalıdır`

## Katkı Sağlayacak Geliştiriciler İçin Notlar

- runtime iddialarını dürüst tutun
- simüle edilmiş başarı yanıtları yerine gerçek entegrasyonları tercih edin
- ölçülebilir operasyonel fayda olmadan ağır bağımlılıklar eklemeyin
- panel arızalarının website çalışma yolunu etkilememesi prensibini koruyun
- host seviyesindeki otomasyonu production-grade altyapı kodu olarak ele alın

## Lisans

AuraPanel, [MIT License](./LICENSE) ile dağıtılmaktadır.

## Geliştirici

Mkoyazılım ([www.mkoyazilim.com](https://www.mkoyazilim.com)) & Tahamada
