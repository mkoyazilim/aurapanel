# AuraPanel

<p align="right">
  <a href="./README.md">English</a> | TÃ¼rkÃ§e
</p>

AuraPanel, hÄ±zlÄ±, gÃ¼venlik odaklÄ± ve operasyonel olarak dÃ¼rÃ¼st bir hosting kontrol dÃ¼zlemi arayan operatÃ¶rler iÃ§in geliÅŸtirilmiÅŸ modern bir hosting panelidir.

Platform, ayrÄ±k bir mimari etrafÄ±nda tasarlanmÄ±ÅŸtÄ±r:

- yÃ¶netim arayÃ¼zÃ¼ iÃ§in `Vue 3 + Vite`
- kimlik doÄŸrulama, RBAC, statik panel sunumu ve kontrollÃ¼ proxy katmanÄ± iÃ§in `Go API Gateway`
- host otomasyonu, runtime entegrasyonlarÄ± ve sistem seviyesinde orkestrasyon iÃ§in `Go Panel Service`
- web sunum katmanÄ± olarak `OpenLiteSpeed`

Temel tasarÄ±m hedefi nettir: kontrol dÃ¼zlemi ile sunum dÃ¼zlemi birbirinden ayrÄ±lmalÄ±dÄ±r. BÃ¶ylece panel yeniden baÅŸlatÄ±lsa, gÃ¼ncellense veya geÃ§ici olarak eriÅŸilemez olsa bile web siteleri Ã§alÄ±ÅŸmaya devam eder.

## Neden AuraPanel

AuraPanel, shell komutlarÄ±nÄ±n Ã¼stÃ¼ne ince bir arayÃ¼z eklemek iÃ§in tasarlanmadÄ±. GerÃ§ek bir hosting platformu olarak ÅŸu prensiplerle ÅŸekillenmektedir:

- performans Ã¶ncelikli operasyon tasarÄ±mÄ±
- fail-closed gÃ¼venlik varsayÄ±lanlarÄ±
- aÃ§Ä±k ve dÃ¼rÃ¼st runtime davranÄ±ÅŸÄ±
- deterministik altyapÄ± otomasyonu
- sahte baÅŸarÄ± yanÄ±tlarÄ± yerine gerÃ§ek host entegrasyonlarÄ±

Bir yetenek hosta, harici bir APIâ€™ye veya yÃ¶netilen bir dosya/konfigÃ¼rasyon yoluna baÄŸlÄ± deÄŸilse aktifmiÅŸ gibi sunulmamalÄ±dÄ±r.

## Mimari

```text
TarayÄ±cÄ±
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

### Kontrol DÃ¼zlemi KatmanlarÄ±

`frontend/`
- Vue 3, Vite ve router/store odaklÄ± bir frontend mimarisi ile geliÅŸtirilmiÅŸ operatÃ¶r arayÃ¼zÃ¼
- operasyonel iÅŸ akÄ±ÅŸlarÄ±, gÃ¶rÃ¼nÃ¼rlÃ¼k ve dÃ¼ÅŸÃ¼k sÃ¼rtÃ¼nmeli host yÃ¶netimi iÃ§in tasarlanmÄ±ÅŸtÄ±r

`api-gateway/`
- kimliÄŸi doÄŸrulanmÄ±ÅŸ trafiÄŸin merkezi giriÅŸ noktasÄ±dÄ±r
- request middleware, JWT doÄŸrulama, rol tabanlÄ± yetkilendirme, CORS, request ID ve servis proxy mantÄ±ÄŸÄ±nÄ± uygular
- production ortamÄ±nda derlenmiÅŸ panel arayÃ¼zÃ¼nÃ¼ sunar

`panel-service/`
- host seviyesinde otomasyonu yÃ¼rÃ¼tÃ¼r ve gerÃ§ek runtime aksiyonlarÄ±nÄ± koordine eder
- website oluÅŸturma, mail provisioning, veritabanÄ± yÃ¶netimi, firewall iÅŸlemleri, tuning endpointâ€™leri, backup akÄ±ÅŸlarÄ±, runtime app akÄ±ÅŸlarÄ± ve servis kontrolÃ¼nÃ¼ yÃ¶netir

## Performans YaklaÅŸÄ±mÄ±

AuraPanel performans Ã¶ncelikli bir anlayÄ±ÅŸla tasarlanmÄ±ÅŸtÄ±r:

- `AyrÄ±k sunum yolu`: web siteleri panel runtimeâ€™Ä± ile deÄŸil, OpenLiteSpeed ile servis edilir
- `Go tabanlÄ± kontrol servisleri`: dÃ¼ÅŸÃ¼k overhead, Ã¶ngÃ¶rÃ¼lebilir aÃ§Ä±lÄ±ÅŸ sÃ¼resi ve bellek davranÄ±ÅŸÄ±
- `Minimal proxy katmanÄ±`: API Gateway, ana `/api/v1/` yÃ¼zeyini doÄŸrudan panel-service katmanÄ±na iletir
- `HÄ±zlÄ± yerel entegrasyonlar`: sistem aksiyonlarÄ± aÄŸÄ±r orkestrasyon katmanlarÄ± yerine deterministik CLI, servis ve config baÄŸlarÄ±yla yÃ¼rÃ¼tÃ¼lÃ¼r
- `Operasyonel izolasyon`: panel yeniden baÅŸlatmalarÄ± website kesintisi anlamÄ±na gelmez
- `OdaklÄ± tuning yÃ¼zeyleri`: yÃ¼ksek etkili tuning yalnÄ±zca gerekli alanlarda sunulur; Ã¶rneÄŸin OpenLiteSpeed, veritabanlarÄ±, FTP, PHP ve mail stack

## GÃ¼venlik YaklaÅŸÄ±mÄ±

AuraPanel, zero-trust ve fail-closed yaklaÅŸÄ±mÄ±yla geliÅŸtirilmektedir:

- korumalÄ± tÃ¼m istekler kimlik doÄŸrulamadan geÃ§er
- RBAC gateway katmanÄ±nda uygulanÄ±r
- desteklenmeyen endpointâ€™ler sahte baÅŸarÄ± yerine `501 Not Implemented` dÃ¶ndÃ¼rÃ¼r
- installer akÄ±ÅŸÄ± kontrollÃ¼ izinlerle environment dosyalarÄ± Ã¼retir
- imzalÄ± manifest doÄŸrulamasÄ± ile verified release bootstrap desteklenir
- firewall otomasyonu yalnÄ±zca gerekli hosting ve panel portlarÄ±nÄ± aÃ§ar
- panel ve servis kimlik bilgileri kurulum sÄ±rasÄ±nda Ã¼retilir, senkronize edilir ve smoke-check ile doÄŸrulanÄ±r
- ModSecurity ve OWASP CRS entegrasyonu WAF korumasÄ± iÃ§in desteklenir
- SSH key iÅŸ akÄ±ÅŸlarÄ±, 2FA akÄ±ÅŸlarÄ± ve security status endpointâ€™leri birinci sÄ±nÄ±f bileÅŸenlerdir

## GerÃ§ek Runtime YÃ¼zeyi

AuraPanel ÅŸu anda aÅŸaÄŸÄ±daki alanlarda gerÃ§ek entegrasyonlar iÃ§erir:

- website provisioning ve OpenLiteSpeed vhost senkronizasyonu
- `.htaccess` write-through ve OpenLiteSpeed rewrite yÃ¶netimi
- PHP sÃ¼rÃ¼m atama ve `php.ini` yÃ¶netimi
- MariaDB ve PostgreSQL provisioning, kullanÄ±cÄ± bilgileri, remote access ve tuning
- Postfix ve Dovecot provisioning, mailbox, forward, catch-all ve mail SSL akÄ±ÅŸlarÄ±
- Pure-FTPd ve SFTP provisioning
- PowerDNS zone ve record yÃ¶netimi
- SSL issuance, custom certificate, wildcard ve hostname binding akÄ±ÅŸlarÄ±
- backup, database backup ve dahili MinIO backup target desteÄŸi
- Docker runtime ve uygulama yÃ¶netimi
- Cloudflare durum ve entegrasyon akÄ±ÅŸlarÄ±
- `wp-cli` Ã¼zerinden WordPress yÃ¶netimi
- malware scan ve quarantine akÄ±ÅŸlarÄ±
- firewall ve SSH key yÃ¶netimi
- panel port yÃ¶netimi ile servis/process gÃ¶rÃ¼nÃ¼rlÃ¼ÄŸÃ¼
- migration upload, analiz ve import akÄ±ÅŸlarÄ±

Daha net bir runtime durum Ã¶zeti iÃ§in [ENDPOINT_AUDIT.md](./ENDPOINT_AUDIT.md) dosyasÄ±na bakabilirsiniz.

## Desteklenen Kurulum Hedefleri

Production installer ÅŸu iÅŸletim sistemlerini hedeflemektedir:

- Ubuntu `22.04` ve `24.04`
- Debian `12+`
- AlmaLinux `8/9`
- Rocky Linux `8/9`

## Production Kurulumu

### 1. Standart Uzak Kurulum

GitHub Ã¼zerinden uzak kurulum baÅŸlatmanÄ±n en basit yolu:

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
```

Bu akÄ±ÅŸ ana installerâ€™Ä± kullanÄ±r ve host Ã¼zerinde gerekli runtime stackâ€™i hazÄ±rlar.

### 2. DoÄŸrulanmÄ±ÅŸ Release Bootstrap

AuraPanel, imzalÄ± manifest ve SHA-256 doÄŸrulamalÄ± release bundle tabanlÄ± verified bootstrap akÄ±ÅŸÄ±nÄ± da destekler.

Ã–rnek:

```bash
export AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

Bootstrap sÃ¼recini belirli bir manifest dosyasÄ±na da yÃ¶nlendirebilirsiniz:

```bash
export AURAPANEL_MANIFEST_URL="https://example.com/releases/latest/aurapanel_release_manifest.env"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

### 3. DoÄŸrudan Bootstrap Script KullanÄ±mÄ±

Verified bootstrap aÅŸamasÄ±nÄ± doÄŸrudan Ã§alÄ±ÅŸtÄ±rmak isterseniz:

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/aurapanel_bootstrap.sh -o aurapanel_bootstrap.sh
chmod +x aurapanel_bootstrap.sh
sudo AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download" ./aurapanel_bootstrap.sh
```

## Production Installer Neleri Kurar

Installer, tam panel hostâ€™u kurmak Ã¼zere ÅŸu bileÅŸenleri hazÄ±rlayacak ÅŸekilde tasarlanmÄ±ÅŸtÄ±r:

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
- AuraPanel bileÅŸenleri iÃ§in systemd servisleri
- firewall temel kurallarÄ±
- panel, gateway, OpenLiteSpeed, MinIO ve auth akÄ±ÅŸlarÄ± iÃ§in smoke checkâ€™ler

### OluÅŸturulan systemd Servisleri

Production kurulum ÅŸu servisleri oluÅŸturur ve yÃ¶netir:

- `aurapanel-service`
- `aurapanel-api`

Host durumuna ve etkin modÃ¼llere baÄŸlÄ± olarak AuraPanel ÅŸu servislerle de Ã§alÄ±ÅŸÄ±r:

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

## Yerel GeliÅŸtirme

### Gereksinimler

- Go `1.22+`
- Node.js `20+`

### Windows YardÄ±mcÄ± Scripti

Repository iÃ§inde tÃ¼m yerel stackâ€™i baÅŸlatan yardÄ±mcÄ± bir script bulunmaktadÄ±r:

```powershell
.\start-dev.ps1
```

VarsayÄ±lan yerel endpointâ€™ler:

- Frontend: `http://127.0.0.1:5173`
- Gateway: `http://127.0.0.1:8090`
- Panel Service: `http://127.0.0.1:8081`

VarsayÄ±lan development giriÅŸi:

- E-posta: `admin@server.com`
- Åifre: `password123`

### Manuel GeliÅŸtirme BaÅŸlatma

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

TÃ¼m bileÅŸenleri derlemek iÃ§in:

```bash
make build
```

Release tarball Ã¼retmek iÃ§in:

```bash
make package
```

Artifact temizliÄŸi iÃ§in:

```bash
make clean
```

## Repository YapÄ±sÄ±

```text
aurapanel/
|-- api-gateway/        # Go API Gateway
|-- panel-service/      # Go host otomasyonu ve runtime orkestrasyonu
|-- frontend/           # Vue 3 + Vite kontrol paneli
|-- installer/          # Production kurulum mantÄ±ÄŸÄ±
|-- docs/               # YardÄ±mcÄ± teknik dokÃ¼mantasyon
|-- aurapanel_bootstrap.sh
|-- aurapanel_installer.sh
|-- install.sh
|-- start-dev.ps1
|-- Makefile
`-- ENDPOINT_AUDIT.md
```

## Operasyonel Prensipler

AuraPanel birkaÃ§ temel prensipten taviz vermez:

- `Kontrol dÃ¼zlemi != sunum dÃ¼zlemi`
- `Kozmetik tamlÄ±k yerine operasyonel dÃ¼rÃ¼stlÃ¼k`
- `Konfor yerine gÃ¼venlik varsayÄ±lanlarÄ±`
- `KÄ±rÄ±lgan gizli state yerine deterministik otomasyon`
- `Performans hassas yollar mÃ¼mkÃ¼n olduÄŸunca sade kalmalÄ±dÄ±r`

## KatkÄ± SaÄŸlayacak GeliÅŸtiriciler Ä°Ã§in Notlar

- runtime iddialarÄ±nÄ± dÃ¼rÃ¼st tutun
- simÃ¼le edilmiÅŸ baÅŸarÄ± yanÄ±tlarÄ± yerine gerÃ§ek entegrasyonlarÄ± tercih edin
- Ã¶lÃ§Ã¼lebilir operasyonel fayda olmadan aÄŸÄ±r baÄŸÄ±mlÄ±lÄ±klar eklemeyin
- panel arÄ±zalarÄ±nÄ±n website Ã§alÄ±ÅŸma yolunu etkilememesi prensibini koruyun
- host seviyesindeki otomasyonu production-grade altyapÄ± kodu olarak ele alÄ±n

## Lisans

AuraPanel, [MIT License](./LICENSE) ile daÄŸÄ±tÄ±lmaktadÄ±r.

## GeliÅŸtirici

MkoyazÄ±lÄ±m ([www.mkoyazilim.com](https://www.mkoyazilim.com)) & Tahamada

## Git Pull Deploy (Guncelleme)

Kurulum yapilmis sunucuda guncelleme icin:

```bash
cd /opt/aurapanel
bash scripts/deploy-main.sh
```

Bu akis `main` icin `git pull --ff-only` calistirir, backend ve frontend derlemesini yapar, `aurapanel-service` ve `aurapanel-api` servislerini yeniden baslatir ve health check dogrulamasi yapar.
