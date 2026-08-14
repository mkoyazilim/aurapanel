# AuraPanel — Mimari ve Güvenlik Tasarımı

**Sürüm:** 1.0 · **Tarih:** 2026-08-14 · **Durum:** Onaylı tasarım

## 1. Vizyon ve Hedef

AuraPanel; OpenLiteSpeed için çok hafif, OLS-native, site başına güçlü izolasyon sağlayan, güvenlik öncelikli ve tek sunucuda minimum saldırı yüzeyi oluşturan bir hosting control panelidir.

> **Nihai hedef:** "OpenLiteSpeed için çok hafif, güvenlik öncelikli, site başına gerçek izolasyon sağlayan, tek binary ile çalışan modern hosting control plane."

cPanel'i birebir kopyalamak hedef değildir. Gereksiz servisler, microservice yapısı ve gateway katmanı kullanılmaz. Amaç; **OLS + Linux İzolasyonu + PHP + SSL + MariaDB + SFTP + Backup + Security** işlerini mümkün olan en küçük ve güvenli kontrol düzleminde birleştirmektir.

### 1.1 Öncelik Sırası (değiştirilemez)

```
1. Security            → "Bu özellik saldırı yüzeyini ne kadar artırıyor?"
2. Isolation
3. OLS correctness
4. Reliability / rollback
5. Performance
6. Features
7. UI polish
```

Yeni bir özellik için **threat model + permission model + audit log + rollback davranışı** düşünülmeden implementasyona başlanmaz.

## 2. Teknoloji Kararları

| Alan | Karar | Not |
|---|---|---|
| Backend | Go, CGO **yok**, tek statik binary | Düşük RAM, tek makinede minimum servis |
| API | REST + WebSocket | Aynı API hem Web UI hem CLI tarafından kullanılır |
| Metadata DB | SQLite (WAL, FK, busy_timeout, atomic transactions) | Yalnızca backend yazar; otomatik yedeklenir |
| Site DB | MariaDB (PostgreSQL ileride opsiyonel) | Site verileri asla metadata DB'de tutulmaz |
| Web server | OpenLiteSpeed 1.7+ native vhost'lar | Source of truth: SQLite desired state |
| PHP | Çoklu LSPHP sürümü, site başına pool | Her site farklı sürüm kullanabilir |
| Firewall | nftables | iptables'a doğrudan bağımlılık yok |
| Frontend | **Vue 3 + Vite** SPA + **Monaco Editor** | Açık renkli, sade-profesyonel tasarım; hiyerarşik kategorili menüler; go:embed ile tek binary |
| Node.js / PHP | Panel backend'i olarak **kullanılmaz** | — |

## 3. Süreç Mimarisi — Privilege Modeli

Panel **root olarak çalışmaz**. İki süreç vardır:

```
AuraPanel
    |
    +-- aurapanel (unprivileged)
    |       REST/WS server · OLS renderer · apply pipeline · drift detector
    |       ACME istemcisi · backup engine · scheduler · sağlık kontrolleri
    |
    +-- aurapanel-priv (minimal privileged helper)
            |   Linux user yönetimi
            |   cgroups (delegation + kurulum)
            |   OLS config apply (yalnızca dosya yerleştirme + reload)
            |   firewall (nftables) işlemleri
            |   sertifika kurulumu
            |   sshd SFTP jail config · logrotate config
            |   diğer gerekli privileged işlemler
```

- `aurapanel-priv` **allowlist tabanlı** çok küçük bir helper'dır; yalnızca önceden tanımlanmış operasyonları kabul eder.
- Kullanıcı girdisi **asla** `sudo sh -c "..."` veya shell'e string aktaran yöntemlerle çalıştırılmaz.
- Helper içinde komutlar sabit binary + argüman dizisi (`exec` ailesi) ile çalışır; shell yorumlayıcısı asla kullanılmaz.
- Dağıtım: helper **aynı statik binary'nin priv modudur** (busybox tarzı — `aurapanel-priv` symlink'i veya flag ile başlatılır); mod seçimi başlangıçta yapılır, kod yolları ayrıdır.
- Helper, systemd **socket activation** ile yönetilir: `aurapanel-priv.socket` (root'a ait Unix socket, `SocketGroup=aurapanel`) → çağrı geldiğinde helper root olarak kısa süreliğine ayağa kalkar, işlemi yapar, kapanır.

### 3.1 Helper Haberleşme Protokolü

- **Kanal:** Unix socket `/run/aurapanel/priv.sock` (root:aurapanel).
- **Kimlik doğrulama:** `SO_PEERCRED` ile bağlanan sürecin UID'si doğrulanır (yalnızca panel kullanıcısı kabul edilir).
- **Format:** satır tabanlı JSON — istek `{"op":"...","args":{...}}` → yanıt `{"ok":true|false,"data":...,"error":...}`.
- Her op ayrı şema ile doğrulanır; bilinmeyen op veya şema ihlali reddedilir (fuzz testleri zorunludur).
- Helper, tüm çağrıları **root'a ait append-only** `/var/log/aurapanel/priv.log` dosyasına yazar — panel compromise olsa bile priv log değiştirilemez/silinemez.

### 3.2 Helper Operasyon Allowlist'i (MVP)

```
user.create / user.delete / user.modify      → useradd/userdel/usermod (nologin)
cgroup.bootstrap / cgroup.limits             → subtree kurulumu + cpu/memory/pids/io limitleri
quota.set / quota.get                        → filesystem quota (ext4 user quota; ileride xfs project quota)
file.op                                      → fork + site cgroup'una attach + setuid(site UID) + iç doğrulanmış işlem (read/write/delete/move/copy/extract/archive) — detay: FILE_MANAGER.md §4
trash.move / trash.restore / trash.empty     → çapraz dizin taşıma gerektiren trash işlemleri (root gerekli, allowlist'li)
ols.install_config                           → önceden render + validate edilmiş paketi atomik yerleştir, reload tetikle
ols.webadmin_credentials                     → OLS WebAdmin kullanıcı/şifre senkronizasyonu
cert.install / cert.remove                   → sertifika dosyaları (0600, doğru sahiplik)
firewall.apply                               → panelin render ettiği nftables ruleset dosyasını `nft -f` ile uygula
fail2ban.configure                           → jail yapılandırması
sshd.install_config                          → SFTP jail Match blokları (`sshd -t` doğrulamalı)
logrotate.install_config                     → site log rotate kuralları
bootstrap.*                                  → kurulum anı operasyonları (tek kullanımlık)
```

Bu liste dışında hiçbir işlem helper'da yoktur. Liste genişletilirken threat model bölümü güncellenir.

## 4. Veri Katmanı

### 4.1 SQLite (metadata)

- WAL mode, foreign keys, busy timeout, atomic transactions, migration sistemi.
- Yazma yetkisi yalnızca AuraPanel backend'indendir.
- Otomatik yedekleme: periyodik, yedek dosyası ayrı şifreli depoda tutulur.

**Tablolar (v1):**

```
users, roles, sessions, sites, domains, ssl_certificates,
php_versions, php_pools, databases, database_users, sftp_accounts,
cron_jobs, backups, audit_logs, security_profiles, system_settings
```

(+ uygulama sırasında: `drift_events`, `metrics` zaman serisi)

### 4.2 Secrets Yönetimi

| Tür | Yöntem |
|---|---|
| Kullanıcı parolaları, PAT hash'leri | Argon2id |
| Geri okunması gereken secret'lar (DB şifresi, SMTP, Cloudflare token, S3 secret, backup credential) | **Encrypted-at-rest**: XChaCha20-Poly1305 (AEAD) |
| Master key | `/var/lib/aurapanel/keys/master.key` (0600, panel kullanıcısı, kurulumda 32 byte üretilir) — **asla metadata DB içinde plaintext tutulmaz** |

Key rotation: tüm secret kayıtlarını yeni key ile yeniden şifreleme akışı (ileride).

## 5. OLS Entegrasyonu — Config Yaşam Döngüsü

OLS birinci sınıf ve native serving engine'dir; vhost yapısı `conf/vhosts/<domain>/vhost.conf`.

**Source of truth: AuraPanel SQLite desired state** — OLS config dosyaları değil.

### 5.1 Apply Pipeline

```
SQLite Desired State
      │
      ▼
OLS Config Renderer        (template'ler sürüm kontrolünde; yalnızca panel yazar)
      │
      ▼
Generated Config           (geçici dizinde)
      │
      ▼
Şema / Path / Permission validation
      │
      ▼
OLS config validation      (OLS'nin kendi test mekanizması — hedef OLS sürümüne göre sabitlenir)
      │
      ▼
Snapshot                   (her değişiklik öncesi mevcut çalışan config kopyalanır)
      │
      ▼
Atomic apply               (dosya değişimi rename ile; kısmi yazım imkânsız)
      │
      ▼
Graceful reload            (siteler kesilmeden)
      │
      ▼
HTTP health check          (etkilenen vhost'lar + OLS canlılık probu)
      │
      ├── SUCCESS → tamam
      └── FAILURE → ROLLBACK (snapshot geri yüklenir → yeniden reload → tekrar doğrula)
```

- Yanlış OLS configuration **AuraPanel'i veya çalışan siteleri devirmez**.
- Site düzeyindeki değişiklikler yalnızca ilgili `vhost.conf`'u hedefler; global config değişiklikleri ayrı pipeline'da ve daha sıkı doğrulamayla işlenir (blast radius küçük tutulur).
- **Panel restart/redeploy olduğunda siteler çalışmaya devam eder:** OLS panelden bağımsız yaşar; panel yalnızca config yönetir (İlke 7).

### 5.2 OLS ve Bağımlılık Güncelleme Stratejisi

**İlke:** Hiçbir büyük değişiklik doğrulanmadan uygulanmaz; her güncelleme geri alınabilir.

- **Otomatik güncelleme yok:** OLS/LSPHP/MariaDB güncellemeleri yalnızca panel **Güncelleme Merkezi** üzerinden admin onayıyla yapılır. Panelin yönettiği paketler `apt-mark hold` ile tutulur — apt upgrade ile kontrolsüz OLS güncellemesi engellenir.
- **Güvenli güncelleme akışı:**
  1. Mevcut durum snapshot'ı (config + paket sürümleri, eski .deb yedeği)
  2. **Uyumluluk kontrolü:** hedef sürüm bizim renderer'a karşı test edilmiş mi? (uyumluluk matrisi)
  3. Güncelleme kurulumu
  4. Desired state yeniden render + OLS config validation
  5. §5.1 apply pipeline + HTTP health check
  6. Başarısızlık → **otomatik rollback** (config + paket geri alma)
- **Büyük sürüm davranışı:** Yeni OLS sürümü CI'da henüz doğrulanmadıysa panel "**bu sürüm test edilmemiştir**" uyarısıyla yine de güncellemeye izin verir (snapshot + rollback güvencesiyle). Kritik güvenlik yamaları öncelikli işaretlenir.
- **Uyumluluk matrisi:** Her AuraPanel sürümü CI'da belirli OLS/LSPHP/MariaDB sürümlerine karşı entegrasyon testinden geçer (Ubuntu VM: kur → apply/rollback/health senaryoları). Yeni OLS sürümü çıktığında CI otomatik test eder, sonuç "desteklenen sürümler" listesine işlenir.
- **Renderer sürüm farkındalığı:** Config renderer'ı tespit edilen OLS sürümüne göre direktif üretir (değişen/eklenen direktifler için sürüm anahtarlı şablonlar); eski sürümler için uyumlu çıktı üretmeye devam eder.
- **Panel kendi güncellemesi (self-update):** İmzalı sürüm kanalından yeni binary indirilir → imza doğrulaması → atomik değişim → graceful restart. **Siteler etkilenmez** (İlke 7: OLS panelden bağımsız çalışır).
- **LSPHP güvenlik güncellemeleri:** PHP CVE'leri acil akış — hızlı güncelleme yolu; site bazlı PHP sürüm geçişi (W6) ile kademeli geçiş mümkün.

### 5.3 Sürüm Sabitleme (Pinned Versions) ve Dağıtım Manifesti

Dağıtılmış kurulumlarda **"son sürümü indir" yaklaşımı yasaktır.** Kurulum her zaman test edilmiş sabit sürümleri kurar:

- **Sürüm manifesti:** Her AuraPanel sürümüyle birlikte `versions.json` yayınlanır: panel binary'si, OLS, her LSPHP sürümü, MariaDB — kesin sürümler + SHA-256. Manifest, o sürümün CI'da test edildiği kombinasyonu yansıtır.
- **Kurulum sabitler:** Installer manifest'teki kesin sürümleri kurar (asla repo'dan "latest" çekmez) ve kurulum sonunda panelin yönettiği tüm paketleri `apt-mark hold` ile kilitler.
- **Installer de sabitlenir:** Önerilen kurulum komutu `main` dalını değil sürüm tag'ini işaret eder: `.../aurapanel/v1.0.0/installer/aurapanel.sh` — 6 ay sonra kuran kullanıcı da bugün kuran kullanıcı da **aynı test edilmiş seti** alır.
- **Vendor'lanmış paketler:** OLS deposu eski sürümleri barındırmayabilir → test edilen OLS `.deb`'i bizim release asset'lerimizde değiştirilmeden mirror'lanır (GPLv3 gereği kaynak işareti korunur). LSPHP sürümleri zaten bizim tarafımızdan derlenir.
- **Güncellemeler yalnızca doğrulanmış yoldan:** Güncelleme Merkezi yalnızca CI'da doğrulanıp yeni manifest ile yayınlanan sürümleri önerir (§5.2). CVE durumunda acil manifest güncellemesi + panel bildirimi; güncelleme yine doğrulama pipeline'ından geçer.
- **EOL politikası:** Desteklenmeyen eski sürüm kombinasyonları panelde "destek dışı" uyarısıyla gösterilir; yükseltme yolu panel üzerinden test edilmiş akışla sunulur.

## 6. Configuration Drift Detection

Örnek: SQLite'ta `site.example.com → PHP 8.3 + SSL` iken sistemde vhost silinmiş veya pool değiştirilmişse → **CONFIGURATION DRIFT DETECTED**.

- **Scanner kaynakları:** vhost dizinleri ve içerikleri, `/etc/passwd` site kullanıcıları, cgroup limit değerleri, quota raporu, sertifika dosyaları, LSPHP pool konfigürasyonları.
- **Diff:** desired state (SQLite) vs gerçek sistem → `drift_events` (etkilenen servis, beklenen, gerçek, şiddet).
- **UI/API:** drift listesi + `Repair` (apply pipeline üzerinden reconciliation) + `Baseline kabul et` + opsiyonel **otomatik reconciliation** politikası.
- Periyodik tarama (panel scheduler) + isteğe bağlı tarama.

## 7. Site İzolasyonu

**Ana model: User-per-site + cgroups v2 + filesystem quota.**

| Kaynak | Mekanizma |
|---|---|
| Kimlik | Ayrı Linux UID/GID (`www-site001`, `www-site002`, …) |
| Dosya sistemi | Ayrı home/root; site dizinleri 0750; grup kapsamında erişim |
| Süreç | Ayrı LSPHP pool + ayrı süreç kimliği |
| CPU / RAM / PID / IO | Ayrı cgroup v2 |
| Disk / inode | Filesystem quota — hedef ortam tek ext4 kök disk: **ext4 user quota**; ayrı disk eklenirse xfs project quota |
| tmp | Ayrı private tmp (0700) |
| Log | Ayrı log dizini |

Bir sitenin PHP süreci başka bir sitenin dosyalarını **okuyamaz** (UID/GID + izinler temel sınır).

### 7.1 Dizin Düzeni

```
/srv/aurapanel/
├── sites/<name>/
│   ├── home/        # site document root (www-siteNNN:www-siteNNN)
│   ├── logs/        # access/error
│   ├── tmp/         # private tmp (0700)
│   └── conf/        # site'a özel php.ini vb.
├── state/
│   ├── certs/       # yönetilen sertifikalar (0700)
│   └── snapshots/   # OLS config snapshot'ları
└── backups/         # lokal yedekler

/var/lib/aurapanel/  → aurapanel.db, keys/
/run/aurapanel/      → panel.sock, priv.sock
```

### 7.2 chroot ve defense-in-depth

chroot **ana güvenlik sınırı değildir**; yalnızca defense-in-depth katmanıdır. Ana izolasyon: **UID/GID + filesystem permissions + process isolation + cgroups + quota + PHP hardening**. İleride namespace tabanlı ek izolasyon opsiyonu değerlendirilebilir.

## 8. PHP İzolasyonu ve Hardening

- Her site için ayrı LSPHP pool: `site001 → LSPHP, UID www-site001, cgroup site001, private tmp, site001 filesystem`.
- Her site farklı PHP sürümü kullanabilir (`example.com → 8.3`, `test.com → 8.4`, `legacy.com → 8.2`); sürüm değişimi yeni render + apply pipeline ile yapılır, mevcut siteleri bozmaz; hata olursa rollback devreye girer.
- Hardening katmanları (defense-in-depth): filesystem permissions, UID/GID, private tmp, `open_basedir`, `disable_functions`, session hardening, PHP sürüm izleme (EOL uyarıları).
- `open_basedir` **ana güvenlik sınırı sayılmaz** — asıl mekanizma filesystem + process isolation'dır.

## 9. Güvenlik Mimarisi

### 9.1 Panel Authentication

- Öncelik: **server-side session** (SQLite `sessions` tablosu) + `HttpOnly` + `Secure` + `SameSite` cookie + **CSRF token**.
- Oturum: idle/absolute timeout, login/logout/şifre değişiminde rotasyon.
- İlk kurulumda üretilen admin parolası için **zorunlu değiştirme**: ilk girişten itibaren kalıcı uyarı banner'ı; değiştirilene kadar hatırlatılır.
- CLI ve external API için **Personal Access Token** (256-bit rastgele, argon2id hash ile saklanır, yalnızca bir kez gösterilir, rol kapsamlı, opsiyonel expiry + IP kısıtı).
- **JWT yalnızca gerçek bir kullanım senaryosu ortaya çıkarsa** (ör. Faz 3 multi-server agent kimliği — karar o faza ertelenir).

### 9.2 MFA

- TOTP + WebAuthn/Passkey.
- Production'da admin için MFA **zorunlu yapılabilir** (varsayılan: açık).

### 9.3 Brute Force / Intrusion Protection

- Rate limiting, login throttling (per-IP + per-hesap, üstel geri çekilme), failed login tracking, audit logging.
- **Google reCAPTCHA (login)** — ayarlar sayfasından açılır/kapanır; public modda önerilir; internetsiz ortamda otomatik devre dışı kalır.
- Fail2ban entegrasyonu; ileride CrowdSec opsiyonel modül.

### 9.4 Panel Ağ Güvenliği — Erişim Modları

Kurulum sırasında seçilir (sonradan system_settings'ten değiştirilebilir):

**A) Private mod (internet erişimi yok) — varsayılan:**
- Bind: `127.0.0.1:<port>` veya Unix socket `/run/aurapanel/panel.sock`.
- Kurulum sonunda **SSH tunnel bağlantı tarifi** basılır (hazır komut: `ssh -L 8080:127.0.0.1:8080 root@<ip>`).
- Firewall default-deny; panel portuna gelen tüm dış bağlantılar reddedilir.

**B) Public mod (internet erişimi var) — bilinçli seçim:**
- Panel `http(s)://sunucu-ip:port` üzerinden erişilebilir; kurulum sonunda login adresi gösterilir.
- **Zorunlu hardening:** TLS (kurulumda otomatik self-signed; sonradan LE/custom ile değiştirilebilir), rate limiting + fail2ban, ilk girişte zorunlu şifre değişimi; isteğe bağlı: Google reCAPTCHA (login), TOTP/WebAuthn 2FA.
- reCAPTCHA internet gerektirir; internetsiz kurulumlarda otomatik kapalı kalır.
- Alternatif public yol: `panel.example.com → OLS reverse proxy → AuraPanel localhost`.

**Her iki modda:** OLS WebAdmin (7080) her zaman yalnızca `127.0.0.1`'e bağlı kalır; panel üzerinden "OLS Admin" proxy linki ile erişilir.

### 9.5 Firewall

- **nftables**; iptables'a doğrudan bağımlılık yok.
- Yönetim: panel tiplenmiş kurallardan ruleset dosyası render eder → `firewall.apply` helper op'u `nft -f` ile uygular. Kullanıcı girdisi komut satırına aktarılmaz.

### 9.6 WAF

- ModSecurity + OWASP CRS (OLS modülü).
- Her site için zorunlu değil; site security profile ile eşleşir: `Off / Basic / Hardened`.
- Performans maliyeti nedeniyle kullanıcıya kontrollü olarak açılır.

### 9.7 SSL

- Let's Encrypt (ACME — Go istemcisi), custom certificate, otomatik yenileme (son kullanıma 30 gün kala scheduler).
- Akış: `Request → ACME → Certificate → OLS SSL Config → Validate → Graceful Reload → Health Check` (5.1 pipeline'ı).
- HTTP→HTTPS redirect, TLS 1.2/1.3, modern cipher policy, HSTS (preload varsayılan **kapalı**), certificate expiry monitoring, OCSP stapling (mümkünse).

### 9.8 Audit Log

| Alan | İçerik |
|---|---|
| Zorunlu kayıtlar | login, logout, failed login, MFA değişiklikleri, site oluşturma/silme, domain değişiklikleri, SSL değişiklikleri, DB oluşturma/silme, şifre değişiklikleri, PHP sürüm değişiklikleri, OLS config değişiklikleri, firewall değişiklikleri, backup, restore, dosya işlemleri, kullanıcı/rol değişiklikleri |
| Kayıt şeması | `timestamp, user, IP, action, target, result, request_id` |
| Bütünlük | Append-only (kod UPDATE/DELETE yapmaz), retention politikası, export |

### 9.9 Backup Encryption

- Akış: `Site → Backup → Encrypt → Remote Storage`. Encryption key **backup dosyasının yanında tutulmaz** (ayrı yönetilen key).
- Destek: local, S3-compatible, MinIO, remote server.
- Restore işlemleri audit log'a yazılır.

### 9.10 İlk Kurulum (Bootstrap) ve Yönetici Kimlik Bilgileri

- Kurulum sonunda panel **rastgele admin kullanıcı adı + güçlü rastgele şifre** üretir ve kurulum çıktısında gösterir.
- Aynı kimlik bilgileri **OLS WebAdmin** için de ayarlanır (tek giriş çifti); admin şifresi panelden değiştirildiğinde OLS WebAdmin'e helper üzerinden otomatik senkronize edilir (`ols.webadmin_credentials`).
- İlk girişte **şifre değiştirme zorunludur**; değiştirilene kadar kalıcı uyarı banner'ı gösterilir.
- İlk kurulum sihirbazı: erişim modu özeti, şifre değişimi, MFA kurulum önerisi (public modda önerilir).

## 10. Özellik Modeli

### 10.1 Feature Capability (site başına)

```
site001 → PHP, Database, Cron, SFTP, Mail, Node, Git, SSH
```

Her özellik ayrı ayrı açılıp kapatılabilir; **kullanılmayan runtime siteye bağlanmaz** (Node kapalıysa Node config/runtime üretilmez).

### 10.2 Site Security Profiles

```
Compatibility  → temel PHP kısıtları
Balanced       → + open_basedir, tehlikeli fonksiyonlar kapalı, private tmp, dir listing kapalı
Hardened       → + WAF (Basic+), sıkı disable_functions, session hardening, sıkı izinler, shell kapalı
```

**Not:** İzolasyon altyapısı (UID/GID, cgroup, quota, süreç kimliği) her profilde **her zaman** aktiftir; profiller yalnızca PHP düzeyindeki defense-in-depth katmanlarını ayarlar.

## 11. Alt Sistemler

### 11.1 Dosya Yöneticisi (birinci sınıf MVP modülü)

File Manager, mimaride basit bir CRUD modülü değil **Filesystem Access Layer** olarak kabul edilir; tüm dosya erişimi merkezi `FileService` soyutlamasından geçer (UI/handler asla doğrudan `os.Open`/`os.Remove` kullanmaz).

- Yalnızca ilgili site root içinde çalışır; canonical path + symlink çözümlemesi sonrası site root dışı → **HTTP 403**.
- İşlemler öncelikle **site UID'si ile normal dosya izinleri** üzerinden yürür; root yalnızca allowlist'li özel işlemlerde (trash vb.) kullanılır.
- Safe Mode (MVP'de varsayılan açık), upload/archive güvenliği, trash, kod editörü, atomic write, optimistic locking, rate limiting ve audit kuralları dahil tam spesifikasyon: **[FILE_MANAGER.md](FILE_MANAGER.md)**.

### 11.2 Shell / SFTP

- Site kullanıcılarına varsayılan **shell yok**; SFTP jail içinde (OpenSSH `internal-sftp` + `ChrootDirectory`).
- Root shell hiçbir site kullanıcısına verilmez.
- Panel terminali MVP'de **kapalı**; ileride eklenecekse site UID + site cgroup + restricted environment + jail + command policy ile. Asla `exec("sh -c " + userInput)` yapısı yok.

### 11.3 Veritabanı Yönetimi

- MariaDB: DB oluşturma/silme, kullanıcı oluşturma, şifre sıfırlama, grant yönetimi, mümkünse kota/limit.
- SQL injection-safe: prepared statements; identifier'lar katı regex (`^[a-zA-Z0-9_]{1,64}$`) ile doğrulanıp quote edilir; **string concatenation yasak**.
- **Adminer gate:** Adminer panel içinden, geçici doğrulanmış oturumla açılır (site/DB scope sınırlı), timeout ile kendiliğinden kapanır; sürekli public açık bırakılmaz.

### 11.4 Cron

- Panel scheduler, her site job'ını **site UID'si ile, site cgroup'u içinde** çalıştırır; çıktı log'a yazılır.
- Komutlar shell operatörleri olmadan (komut + argüman listesi) tutulur; string'den shell inşa edilmez.

### 11.5 İzleme ve Sağlık

- Cgroup metrikleri (CPU/RAM/PID/IO) + quota (disk/inode) periyodik toplanır, zaman serisi tutulur, UI'da grafiklenir.
- Sağlık kontrolleri: OLS canlılık probu, MariaDB, sertifika süreleri, kota doluluk uyarıları.

## 12. Faz Kapsamları

### MVP
Domain yönetimi · Site oluşturma/silme · SSL (Let's Encrypt + custom) · PHP sürüm yönetimi + ayarlar · LSPHP/PHP-FPM pool yönetimi · OLS virtual host yönetimi · File manager · SFTP · MariaDB + DB/kullanıcı yönetimi + Adminer · Access/error loglar + live log · Backup/restore + planlı yedek · Cron yönetimi · CPU/RAM/PID/IO limitleri · Disk/inode quota · Security profiles · Audit log · Configuration drift detection · Otomatik rollback · Health checks

### Faz 2
Mail (Postfix, Dovecot, Rspamd, SnappyMail, DKIM/SPF/DMARC — core'dan izole opsiyonel modül) · WordPress installer · Git deploy · Staging · Node.js · Application manager · Cloudflare entegrasyonu · Advanced WAF · CDN entegrasyonu

### Faz 3
Reseller · Multi-server · Remote agents · PowerDNS · External DNS providers · Advanced WAF rules · CDN yönetimi · Cluster management

> Multi-server MVP'ye sokulmaz; mail MVP'den çıkarılmıştır.

## 13. Değiştirilemez Tasarım İlkeleri

1. Panel root olarak sürekli çalışmayacak.
2. Privileged işlemler minimal helper üzerinden yapılacak.
3. Kullanıcı girdisi doğrudan shell'e aktarılmayacak.
4. Kullanıcı girdisi doğrudan SQL string'ine aktarılmayacak.
5. OLS configuration değişiklikleri validate edilmeden uygulanmayacak.
6. OLS configuration başarısız olursa otomatik rollback olacak.
7. Panel restart/redeploy olduğunda siteler çalışmaya devam edecek.
8. Bir site compromise olduğunda diğer sitelere erişim engellenecek.
9. Site isolation UID/GID + process isolation + cgroups + quota üzerine kurulacak.
10. `open_basedir` ve `chroot` defense-in-depth olarak kullanılacak.
11. Panel varsayılan olarak public interface'e açık olmayacak; public erişim yalnızca kurulumda bilinçli seçilirse, TLS + rate limiting + fail2ban + ilk girişte zorunlu şifre değişimi katmanlarıyla açılacak.
12. Admin MFA destekleyecek.
13. Tüm kritik işlemler audit log'a yazılacak.
14. Secrets hash yerine gerektiğinde encrypted-at-rest tutulacak.
15. Gereksiz runtime ve servis kurulmayacak.
16. MVP mümkün olduğunca küçük tutulacak.
17. Microservice/gateway mimarisi kullanılmayacak.
18. SQLite panel metadata için yeterli olduğu sürece PostgreSQL kullanılmayacak.
19. OLS birinci sınıf ve native serving engine olacak.
20. Panelin temel amacı OLS'yi güvenli ve hatasız yönetmek olacak.
