# AuraPanel — Geliştirme Planı (Roadmap)

**Sürüm:** 1.0 · **Tarih:** 2026-08-14

## 1. Fazlar (özet)

| Faz | İçerik |
|---|---|
| **MVP** | Domain yönetimi, site oluşturma/silme, SSL (LE + custom), PHP sürüm yönetimi, LSPHP pool yönetimi, OLS vhost yönetimi, **File Manager (tam güvenlik modülü — FILE_MANAGER.md)**, SFTP, MariaDB + Adminer, loglar + live log, backup/restore, cron, CPU/RAM/PID/IO limitleri, disk/inode quota, security profiles, audit log, drift detection, otomatik rollback, health checks, **güvenli güncelleme yönetimi (OLS/LSPHP/MariaDB + panel self-update)** |
| **Faz 2** | Mail modülü (izole), WordPress installer, Git deploy, Staging, Node.js, Application manager, Cloudflare entegrasyonu, Advanced WAF, CDN |
| **Faz 3** | Reseller, Multi-server, Remote agents, PowerDNS, External DNS, Advanced WAF rules, CDN yönetimi, Cluster |

## 2. MVP İş Paketleri (bağımlılık sırasıyla)

Her paket kapanırken: threat model güncellemesi + audit log entegrasyonu + testler (bkz. §3) tamamlanmış olmalıdır.

| ID | Paket | İçerik | Çıktı (Definition of Done) | Bağlılık | Efor |
|---|---|---|---|---|---|
| **W1** | Temel iskelet | Go modül yapısı, config yükleyici, structured logging (slog), audit log altyapısı, SQLite katmanı + migration'lar + tam şema (v1) | Binary ayağa kalkar; DB şema v1 kurulur; audit kaydı yazılıp okunur ✅ (2026-08-14) | — | M |
| **W2** | `aurapanel-priv` | Unix socket + SO_PEERCRED, op allowlist + şema doğrulama, user/cgroup/quota/nftables/sshd/logrotate ops, append-only priv log, systemd socket activation | Helper fuzz testlerini geçer; rastgele JSON hiçbir beklenmeyen komut çalıştıramaz; op listesi dışı çağrı reddedilir ✅ (2026-08-14; sunucu smoke testi VPS erişimiyle yapılacak) | W1 | L |
| **W3** | OLS renderer + apply pipeline | Desired state → renderer → şema/path/permission validation → OLS config testi → snapshot → atomic apply → graceful reload → health check → rollback | Kasıtlı bozuk config senaryolarında panel ve siteler ayakta kalır; rollback otomatik doğrulanır (testle kanıtlı) | W2 | L |
| **W4** | Site yaşam döngüsü | Site oluştur/sil (user, home, cgroup, quota, tmp, logs), limit düzenleme, feature toggle'lar, güvenli silme akışı | Yeni site 1 komutla izole biçimde ayağa kalkar; silme tüm runtime kalıntılarını temizler; izolasyon testleri geçer | W2, W3 | L |
| **W5** | Drift detection | Scanner (vhost, /etc/passwd, cgroup, quota, cert, pool) + diff motoru + repair (W3 pipeline'ı) + auto-reconcile politikası + UI/API | Manuel bozulan config algılanır, raporlanır, Repair ile desired state'e döner | W3, W4 | M |
| **W6** | PHP yönetimi | LSPHP sürüm envanteri, site başına sürüm seçimi, php.ini editörü (doğrulamalı), pool render, sürüm switch akışı | Site PHP 8.2→8.3 geçişi çalışan siteleri bozmaz; hata durumunda rollback | W3, W4 | M |
| **W7** | SSL / ACME | Go ACME istemcisi, cert store, custom cert yükleme, auto-renew scheduler, expiry monitoring, HSTS/redirect/cipher render'ları | LE sertifikası uçtan uca kurulur + otomatik yenilenir; expiry uyarısı üretilir | W3, W4 | M |
| **W8** | MariaDB yönetimi | DB/kullanıcı/grant yönetimi (prepared + identifier doğrulama), Adminer gate (geçici oturum, scope kısıtı, auto-close) | DB işlemleri injection-safe testlerinden geçer; Adminer yalnızca scope'lu geçici oturumla açılır | W4 | M |
| **W9.1** | FileService çekirdeği | Merkezi filesystem abstraction + güvenlik pipeline'ı (canonical path, symlink policy, site root doğrulama), CRUD ops (site UID tier'i), optimistic locking, audit + rate limit hook'ları — spesifikasyon: FILE_MANAGER.md | Traversal/symlink test paketleri geçer; hiçbir işlem site root dışına çıkamaz (testle kanıtlı); UI/handler'da doğrudan os paketi çağrısı yok | W2, W4 | L |
| **W9.2** | Upload + Archive + Trash | Chunked/resume upload (SHA-256, atomic finalization), upload policy motoru, ZIP/TAR.GZ oluşturma/çıkarma (zip-slip + archive bomb korumaları), trash sistemi (retention 7/14/30, otomatik cleanup, kota entegrasyonu) | 2 GB chunked upload kesintiye dayanır; 1MB→100GB decompression bomb'ı engellenir; trash restore/empty audit'li çalışır | W9.1 | L |
| **W9.3** | File Manager UI | Browse (server-side pagination), text/image preview (SVG `<img>`-only + nosniff/CSP), kod editörü (karar §5.7'ye göre), sort/filter/search, drag&drop + canlı progress (WebSocket), izin preset'leri UI, trash UI, Safe Mode göstergesi | FILE_MANAGER.md §19'daki MVP listesinin tamamı UI'dan yapılabilir; 100k dosyalık dizin akıcı | W9.2 (W12 ile eş zamanlı) | L |
| **W9.4** | SFTP | SFTP hesap yönetimi + OpenSSH Match/jail config üretimi (`sshd -t` doğrulamalı); FM ile aynı filesystem | SFTP kullanıcısı jail dışına çıkamaz; FM'den yüklenen dosya SFTP'de görünür | W2, W4 | M |
| **W10** | Backup / restore | Encrypt-then-upload (local/S3-compatible/MinIO/remote), planlama, retention, restore akışı (audit'li) | Yedek geri yüklenir; yedek dosyası key'siz açılamaz (testle kanıtlı); restore audit log'da | W4, W8 | L |
| **W11** | Auth + RBAC | Session store + cookie/CSRF, login throttling, TOTP + WebAuthn, PAT, roller, admin MFA modu, **bootstrap admin akışı** (ilk girişte zorunlu şifre değişimi + kalıcı uyarı), isteğe bağlı **Google reCAPTCHA** (ayarlardan) | Oturum/CSRF/throttle testleri geçer; ilk girişte şifre değişimi zorunluluğu ve OLS WebAdmin senkronu doğrulanır | W1 | M |
| **W12** | Web UI | **Vue 3 + Vite SPA** (Monaco dahil), go:embed; açık renkli sade-profesyonel tema, hiyerarşik kategorili menüler; canlı log (WebSocket), limit grafikleri, drift/health dashboard | Tüm MVP işlemleri UI'dan yapılabilir; panel tek binary ile servis edilir | W4–W11 | L |
| **W13** | Operasyonel bitiş | Tek komut kurulum script'i (OLS + LSPHP derleme opsiyonu, ext4 user quota kurulumu, cgroup delegation, nftables kuralları, kullanıcılar), **pinned versions manifest (versions.json) + tag'li kurulum URL'si + vendor'lanmış OLS .deb + apt-mark hold**, **erişim modu seçimi (public/private), rastgele admin kimlik bilgisi üretimi + OLS WebAdmin senkronu, private modda SSH tunnel tarifi çıktısı, public modda TLS kurulumu**, log rotation, metadata DB otomatik yedekleme, hardening pass + güvenlik denetimi, dökümantasyon | Sıfır makinede tek komutla kurulum; 6 ay sonra kurulum yapan da bugünküyle aynı test edilmiş seti alır; §4 çıkış kriterlerinin tümü karşılanır | Tümü | M |
| **W14** | Güncelleme Merkezi | OLS/LSPHP/MariaDB sürüm görünürlüğü + **uyumluluk matrisi** + güvenli güncelleme akışı (snapshot → update → validate → health → otomatik rollback), **manifest tabanlı güncelleme önerisi (yalnızca doğrulanmış sürümler) + EOL/destek dışı uyarıları**, `apt-mark hold` politikası, kritik güvenlik yaması işaretleme, panel self-update (imzalı, atomik, kesintisiz) | Yeni OLS sürümüne güncelleme senaryosu testte doğrulanır; başarısız güncelleme otomatik geri alınır; panel güncellemesi sırasında siteler kesintisiz | W3, W4, W6, W8 | M |

**Sıralama mantığı:** Öncelik sırası gereği güvenlik çekirdeği (W2) ve OLS correctness (W3) en başta; UI (W12) en sonda.

## 3. Test Stratejisi

| Katman | Ortam | Kapsam |
|---|---|---|
| Unit | Go `go test` | Renderer çıktıları, şema doğrulama, drift diff motoru, quota/cgroup serileştirme, helper op decode (**fuzz**: rastgele JSON beklenmeyen komut çalıştırmamalı) |
| Integration | LXC/VM (Ubuntu 24.04) | **İzolasyon kanıtı:** Site A PHP süreci Site B dosyasını okuyamaz; CPU/RAM/IO/PID limitleri gerçekten uygulanır; quota aşımı reddedilir. **Pipeline:** bozuk config → rollback; sürüm switch'i kesintisiz; drift boz/onar senaryoları |
| Security | Aynı LXC/VM | **File Manager paketi:** path traversal, symlink escape, archive (zip-slip) traversal, archive bomb, SVG XSS, upload filename/null-byte, optimistic lock yarışı; priv helper fuzz + privilege escalation denemeleri; panel session/CSRF/throttle testleri; backup key ayrımı |
| E2E | Sıfır makine | Kurulum script'i → MVP kullanıcı akışları |

## 4. MVP Çıkış Kriterleri

1. Panel root çalışmıyor; tüm privileged işlemler helper allowlist'i üzerinden (denetlenebilir).
2. **Site A compromise → Site B'ye erişim yok** (izolasyon testiyle kanıtlı).
3. Yanlış OLS config paneli ve çalışan siteleri devirmiyor; rollback otomatik (testle kanıtlı).
4. Panel restart/redeploy sırasında siteler kesintisiz hizmet veriyor.
5. Drift algılama + Repair + (opsiyonel) otomatik reconciliation çalışıyor.
6. Admin MFA (TOTP + WebAuthn) çalışıyor; production'da zorunlu kılınabilir.
7. Tüm kritik işlemler audit log'da; priv log append-only ve panel tarafından değiştirilemez.
8. Geri okunması gereken tüm secret'lar encrypted-at-rest; yedekler key'siz açılamıyor.
9. Panel varsayılan olarak yalnızca loopback/Unix socket dinliyor.
10. Tek komutla sıfır makineye kurulum; tek statik binary (frontend gömülü).
11. **File Manager güvenlik paketleri geçer:** traversal, symlink, archive bomb ve SVG testleri dahil hiçbir FM işlemi site root dışına erişemez; tüm FM işlemleri audit log'da (dosya içeriği hariç).
12. **Kurulum akışı doğrulanır:** rastgele admin kimlik bilgileri üretilir + OLS WebAdmin ile senkron; ilk girişte şifre değişimi zorunlu; private modda SSH tunnel tarifi basılır; public modda TLS + rate limit + fail2ban aktif.
13. **Güncelleme akışı doğrulanır:** OLS güncellemesi snapshot + doğrulama + otomatik rollback güvencesiyle çalışır; panel güncellemesi sırasında siteler kesintisiz hizmet verir.

## 5. Açık Kararlar

| # | Konu | Karar / Öneri | Durum |
|---|---|---|---|
| 1 | **Hedef OS** | Ubuntu 24.04 LTS (sunucu hazır, kernel 6.8, cgroups v2) | ✅ Karar verildi |
| 2 | **Lisans** | **Tamamen ücretsiz dağıtım** (onaylandı); kaynak kod açık mı kapalı mı? Bağımlılık politikası her iki durumda aynı: binary/bundle'da yalnızca permissive lisanslar (bkz. DEPENDENCIES.md) | Kısmen açık |
| 3 | **Frontend** | **Vue 3 + Vite**; açık renkli tema, sade-profesyonel tasarım, hiyerarşik kategorili menüler | ✅ Karar verildi |
| 4 | **Site dosya sistemi / kota** | **ext4 user quota** (mevcut diskte en stabil; site başına farklı limitler destekli); xfs project quota ileride ayrı diskle | ✅ Karar verildi |
| 5 | **Test sunucusu** | Mevcut Ubuntu 24.04.4 LTS VPS (SSH bağlı) | ✅ Karar verildi |
| 6 | **Panel erişimi** | Kurulumda seçimli: **public mod** (IP:port + kullanıcı/şifre; reCAPTCHA + 2FA ayarlardan) veya **private mod** (SSH tunnel tarifi basılır) | ✅ Karar verildi |
| 7 | **Kod editörü** | **Monaco Editor** | ✅ Karar verildi |
