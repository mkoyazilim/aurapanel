# AuraPanel — Bağımlılık Envanteri ve Lisans Politikası

**Sürüm:** 1.0 · **Tarih:** 2026-08-14 · **Dağıtım modeli:** Tamamen ücretsiz dağıtım

## 1. Altın Kurallar

1. **Binary'ye (Go) ve frontend bundle'ına yalnızca permissive lisanslı bağımlılıklar girer:** MIT, BSD (2/3), Apache-2.0, ISC, MPL-2.0, PHP License, IPL/EPL. **GPL/AGPL kütüphane asla derlenmez/bundle edilmez.**
2. **Copyleft programlar (GPL/AGPL) yalnızca "ayrı program" olarak kurulur:** değiştirilmeden, resmi paketlerden, ayrı süreç olarak. AuraPanel onlarla yalnızca config dosyaları / Unix socket / TCP üzerinden konuşur → linking yok → copyleft bulaşması yok ("mere aggregation").
3. **Değiştirmediğimiz sürece copyleft kaynak yükümlülüğü doğmaz.** Bir copyleft programı yamalarsak, yalnızca o programın yamaları aynı lisansla ayrı olarak yayınlanır.
4. Dağıtımda tüm lisans metinleri + telif bildirimleri korunur; binary/bundle içindeki lib'ler için `THIRD_PARTY_LICENSES` dosyası CI'da otomatik üretilir (go-licenses / license-checker).
5. Yeni bağımlılık eklenirken: **lisansı belirle → kategoriye koy → bu dokümanı güncelle** (threat model kuralının lisans karşılığı).

## 2. Kategori A — Go Kütüphaneleri (binary'ye derlenir)

| Kütüphane | Amaç | Lisans | Risk |
|---|---|---|---|
| modernc.org/sqlite | Pure-Go SQLite sürücüsü (CGO yasağı gereği) | BSD-3 | ✅ |
| golang.org/x/crypto | XChaCha20-Poly1305, argon2id, ACME istemcisi | BSD-3 | ✅ |
| golang.org/x/sys, x/term | syscall, terminal | BSD-3 | ✅ |
| go-sql-driver/mysql | MariaDB sürücüsü (clean-room Go, libmariadb CGO'suz) | MPL-2.0 | ✅ |
| github.com/pquerna/otp | TOTP (2FA) | Apache-2.0 | ✅ |
| github.com/go-webauthn/webauthn | WebAuthn/Passkey | BSD-3 | ✅ |
| github.com/gorilla/websocket (veya coder/websocket) | Canlı log / upload progress | BSD-3 | ✅ |
| github.com/go-chi/chi (veya net/http) | Router | MIT | ✅ |
| gopkg.in/yaml.v3 | Config | MIT + Apache-2.0 | ✅ |
| github.com/google/uuid | ID üretimi | BSD-3 | ✅ |
| testify (yalnızca test) | Test | MIT | ✅ |

> **Not:** CGO yasak olduğu için CGO'lu `mattn/go-sqlite3` kullanılamaz → `modernc.org/sqlite` seçildi. ACL/firewall CLI çağrıları için Go nftables kütüphanesi şart değil; `nft -f` helper üzerinden çalışır.

## 3. Kategori B — Frontend (bundle'a girer, go:embed)

| Paket | Amaç | Lisans | Risk |
|---|---|---|---|
| Vue 3 + Vue Router + Pinia | SPA çekirdeği | MIT | ✅ |
| Vite + esbuild | Build zinciri (yalnızca geliştirme) | MIT | ✅ |
| Monaco Editor | Kod editörü | MIT | ✅ |
| ECharts (veya Chart.js) | Kaynak kullanım grafikleri | Apache-2.0 / MIT | ✅ |
| UI kütüphanesi (Element Plus / Naive UI / PrimeVue — W12'de seçilir) | Bileşenler | MIT | ✅ |
| Lucide | İkon seti | ISC | ✅ |

> **Kural:** Bundle'a giren hiçbir JS paketi GPL/AGPL olamaz (ikon setleri ve tema paketleri dahil her paket kontrol edilir).

## 4. Kategori C — Sistem Servisleri (kurulum script'i, ayrı süreç)

| Paket | Amaç | Lisans | Not |
|---|---|---|---|
| **OpenLiteSpeed** | Web server (çekirdek) | **GPLv3** | ⚠️ Resmi depodan **değiştirilmeden** kurulur; entegrasyon yalnızca config dosyaları + admin API üzerinden. Yamalanırsa yamalar GPLv3 ile yayınlanır. |
| **lsphp** (8.2/8.3/8.4) | PHP runtime | **PHP License 3.01** | Permissive — derlenip dağıtılması kolay |
| **MariaDB Server** | Site DB'leri | **GPLv2** | ⚠️ Ayrı servis; panel Unix socket üzerinden konuşur (Go sürücü clean-room MPL-2.0) |
| ModSecurity (libmodsecurity, OLS modülü) | WAF | Apache-2.0 | ✅ |
| OWASP CRS | WAF kural seti | Apache-2.0 | ✅ |
| OpenSSH (internal-sftp) | SFTP jail | BSD | ✅ |
| nftables | Firewall | GPLv2 | ⚠️ Sistem paketi; `nft -f` ile yönetilir |
| fail2ban | Brute force koruması | GPLv2 | ⚠️ Sistem paketi |
| logrotate | Log rotasyonu | GPLv2 | ⚠️ Sistem paketi |
| quota | Disk/inode kotası | GPLv2 | ⚠️ Sistem paketi |
| cron | Sistem zamanlayıcısı | ISC | ✅ (site cron'ları panel scheduler'da çalışır) |
| systemd + cgroup araçları | Servis yönetimi, cgroups v2 | LGPLv2.1 | ✅ Sistem bileşeni |

> **Dağıtım notu (pinned versions):** OLS deposu eski sürümleri barındırmayabilir; test edilen OLS `.deb`'i release asset'lerimizde **değiştirilmeden** mirror'lanır (GPLv3 gereği kaynak işareti korunur). Kurulum asla repo'dan "latest" çekmez — ARCHITECTURE §5.3.

## 5. Kategori D — Faz 2 (Mail vb.) — Dikkat Gerektirenler

| Paket | Amaç | Lisans | Risk notu |
|---|---|---|---|
| Postfix | MTA | IPL + EPL | ✅ Permissive |
| Dovecot | IMAP/POP3 | MIT/LGPLv2.1 karma | ✅ |
| Rspamd | Spam filtresi | Apache-2.0 | ✅ |
| **SnappyMail** | Webmail | **AGPLv3** | ⚠️⚠️ **Network copyleft:** değiştirilip servis edilirse, ağ üzerinden kullanan herkese kaynak sunma yükümlülüğü doğar. Değiştirilmeden, izole modül olarak dağıtılırsa sorun yok. Karar Faz 2'de; öneri: değiştirilmemiş kurulum + modül izolasyonu (mail modülü zaten core'dan izole tasarlandı). |
| Roundcube (alternatif webmail) | Webmail | GPLv3 | ⚠️ AGPL kadar katı değil (ağ copyleft'i yok); eklenti istisnası mevcut |

## 6. Kaçınılan / Yasaklı Bağımlılıklar

- **CGO'lu hiçbir kütüphane** (sqlite3 vb.) — CGO yasağı.
- **GPL/AGPL Go kütüphanesi** — binary'ye giremez (Go ekosisteminde nadir ama her bağımlılıkta SPDX kontrolü yapılır).
- **GPL/AGPL JS paketleri** — bundle'a giremez.
- `rsync` (GPLv3) — kullanılırsa yalnızca ayrı program olarak; MVP'de yedekleme senkronu Go ile in-house yapılır.
- Panel runtime'ında **PHP/Node.js yok** (Node yalnızca frontend build zamanı).

## 7. Kurulum Zamanı vs Build Zamanı

- **Build zamanı (yalnızca geliştirici/CI):** Go toolchain (BSD), Node.js (MIT), Vite/esbuild (MIT) — dağıtılan binary'de yer almaz.
- **Kurulum zamanı (hedef sunucu):** Kategori C paketleri apt + OLS resmi deposundan kurulur; LSPHP sürümleri kurulum script'iyle derlenir (veya hazır paket). Script, tüm lisans metinlerini `NOTICE` dosyasına yazar.
- **Dağıtılan binary:** yalnızca Kategori A + B (derlenmiş/bundle edilmiş) içerir.

## 8. Uyumluluk Kontrol Süreci

1. Yeni bağımlılık önerisi → SPDX lisans tespiti.
2. Kategori ataması + risk notu bu dokümanda güncellenir.
3. CI'da otomatik lisans raporu (go-licenses + license-checker) üretilir; copyleft tespitinde build kırılır.
4. Kategori C/D'de bir paket yamalanacaksa: yama, o paketin kendi lisansıyla **ayrı depoda** yayınlanır — asla AuraPanel koduna karışmaz.
