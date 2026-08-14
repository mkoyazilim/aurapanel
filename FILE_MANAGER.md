# AuraPanel — File Manager Modülü ve Güvenlik Gereksinimleri

**Sürüm:** 1.0 · **Tarih:** 2026-08-14 · **Durum:** Onaylı modül spesifikasyonu · **Kapsam:** MVP birinci sınıf modülü

> **Amaç:** Kullanıcının FTP/SFTP kullanmadan web sitesinin dosyalarını güvenli, hızlı ve modern bir arayüz üzerinden yönetebilmesi.
>
> File Manager yalnızca kullanıcının yetkili olduğu site üzerinde çalışır ve **hiçbir koşulda site root'unun dışına erişemez.**

## 1. Mimari Konum

File Manager basit bir CRUD modülü **değildir**. Güvenlik açısından:

```
File Manager  =  Filesystem Access Layer
```

Bütün filesystem erişimi merkezi bir soyutlama üzerinden yapılır:

```
FileService
    +-- ResolvePath()
    +-- ValidatePath()
    +-- Open()
    +-- Read()
    +-- Write()
    +-- Delete()
    +-- Move()
    +-- Copy()
    +-- Extract()
    +-- Archive()
    +-- Upload()
```

UI veya HTTP handler **doğrudan `os.Open`, `os.Remove`, `os.Rename` vb. kullanmaz.** Tüm dosya işlemleri güvenlik pipeline'ından geçer.

## 2. Erişim Pipeline'ı (kritik güvenlik kuralı)

Hiçbir endpoint `user input → os.Open(userInput)` veya `user input → exec(...)` biçiminde çalışmaz. Her filesystem işlemi şu sıradan geçer:

```
Authenticate
   ↓
Authorize
   ↓
Resolve Site                 (site context zorunlu; yetkisiz site → 401/403)
   ↓
Resolve Canonical Path
   ↓
Verify Site Root
   ↓
Verify Symlink Policy
   ↓
Verify Permission
   ↓
Apply Resource Limits        (işlem site cgroup'unda, timeout'lu)
   ↓
Execute Operation            (site UID tier'i veya allowlist'li özel işlem)
   ↓
Audit Log
```

### 2.1 Site-Scoped Kuralı

File Manager her zaman bir Site context'i içinde çalışır (`example.com → File Manager`). Root = sistemde tanımlı gerçek site root'u (`/srv/aurapanel/sites/<name>/home`).

Kullanıcı şunlara **asla** erişemez:

```
/etc   /root   /home/other-user   /var/lib/aurapanel
/proc  /sys    /dev               diğer site dizinleri
```

Site A'nın File Manager'ı Site B'nin dosyalarını göremez.

### 2.2 Path Çözümleme Kuralları

`filepath.Clean()` **tek başına yeterli değildir**. Uygulanan sıra:

```go
// 1. Normalize:  Unicode NFC; null byte + kontrol karakterleri reddedilir (400)
// 2. Join:       filepath.Join(siteRoot, relPath)   → Clean dahil
// 3. Canonical:  filepath.EvalSymlinks(final)        → tüm ara bileşenler çözülür
// 4. Verify:     canonical, canonicalRoot altında mı? (root'un kendisi dahil)
// 5. Policy:     Safe Mode → ara bileşenlerde root dışına çıkan symlink = red
// 6. Permission: site UID'sinin gerçek izinleri (OS backstop)
```

`../`, `../../../etc/passwd`, `/etc/passwd`, `/root/.ssh/`, `/home/site002/` → final canonical path site root altında değilse → **HTTP 403**.

### 2.3 Symlink Escape Protection

- **Varsayılan policy:** Site root dışına işaret eden symlink = **erişim engelli**.
- `public/link → /etc/` varsa `public/link/passwd` ile `/etc/passwd` okunamaz.
- Site root **içine** işaret eden symlink'lere izin verilir (çözümlenmiş hedef yine root içinde kalır).
- Symlink **oluşturma** sırasında da hedef doğrulanır: root dışı hedefe symlink oluşturulamaz.

## 3. Safe Mode

MVP'de **varsayılan olarak aktif**. Safe Mode aktifken:

- Site root dışı erişim yok
- External symlink erişimi yok
- Sistem dizinleri yok (`/proc`, `/sys`, `/dev` dahil)
- Panel metadata dizinleri yok (`/var/lib/aurapanel` dahil)
- Panel secrets yok
- SSH private key erişimi engelli
- Device files yok

## 4. Çalışma Katmanları (Security Boundary)

File Manager her işlemde root yetkisi kullanmaz. Öncelik: **Site UID + normal filesystem permission**. Özel işlemler yalnızca permission-check'li privileged helper üzerinden yürür.

| Katman | Mekanizma | Kullanım |
|---|---|---|
| **Tier 1 (site UID)** | `aurapanel-priv` `file.op`: fork → site cgroup'una attach → setuid(site UID) → iç doğrulanmış Go işlemi çalıştır → yapılandırılmış sonuç döndür | read/write/delete/rename/move/copy, chmod preset'leri, archive oluşturma/çıkarma |
| **Tier 2 (root, allowlist)** | `trash.move / trash.restore / trash.empty`, stat bilgisi toplama | Çapraz dizin taşıma gerektiren trash işlemleri, sahip/grup görüntüleme |

- **Doğrulama panelde, enforcement OS kimliğinde:** Panel tarafı tam pipeline'ı çalıştırır; helper, OS kimliği (site UID) ile backstop sağlar — panelde bir doğrulama hatası olsa bile site UID'nin normal izinleri dışına çıkılamaz.
- Tüm Tier 1 işlemleri site cgroup'u içinde ve timeout'lu çalışır → §12 kaynak limitleri doğal olarak uygulanır.
- Helper yalnızca allowlist edilmiş operasyonları kabul eder (bkz. ARCHITECTURE.md §3.2).

## 5. Özellikler

### 5.1 Dosya İşlemleri

Dosya yükleme · Çoklu yükleme · Drag & Drop · Klasör oluşturma · Dosya oluşturma · Dosya silme · Klasör silme · Yeniden adlandırma · Taşıma · Kopyalama · Dosya indirme · Klasör indirme · ZIP oluşturma · ZIP açma · TAR.GZ oluşturma · TAR.GZ açma · Dosya arama · Boyuta/tarihe/isime göre sıralama · Dosya türüne göre filtreleme · Gizli dosyaları göster/gizle · Dosya bilgileri · İzin görüntüleme · Güvenli izin değiştirme · Sahip/grup görüntüleme

### 5.2 Kod ve Metin Editörü

- Formatlar: PHP, HTML, CSS, JavaScript, JSON, XML, YAML, Markdown, ENV, INI, CONF, TXT
- Özellikler: syntax highlighting, Search, Replace, Go to line, auto indentation, basic formatting
- Editör: **Monaco Editor** (onaylandı) — SPA içinde çalışır, go:embed ile paketlenir.

### 5.3 Güvenli Dosya Editleme (atomic write)

```
Existing File → Temporary Backup → Write New Content → Validate → Atomic Rename → Success
```

- Aynı dizinde geçici dosyaya yaz → validate (boyut/hash; PHP'de mümkünse `php -l`) → `rename` ile atomik değiştir.
- Doğrudan üzerine yazıp dosyayı yarım bırakma riski **yok**. Özellikle `.env`, `config.php`, `wp-config.php`, JSON, YAML kritik.

### 5.4 Dosya Backup / Undo

- Merkezi revision store: `/srv/aurapanel/state/edits/<site-id>/` (site root dışında; dosya başına N revision, varsayılan 10).
- UI: `Save` · `Save & Backup` · `Restore Previous Version`.
- Restore = `file.op` ile geri yazma, audit'li.

## 6. Trash / Recycle Bin

- Silme varsayılan olarak **permanent delete değildir**.
- Konum: `/var/lib/aurapanel/trash/<site-id>/` — site root altında `.trash` oluşturulmaz.
- İşlemler: `Delete` · `Restore` · `Empty Trash`.
- Retention: **7 / 14 / 30 gün** yapılandırılabilir (varsayılan 14); scheduler ile otomatik cleanup.
- Kota entegrasyonu: trash boyutu site başına izlenir ve site disk limiti kapsamında uygulanır (delete/restore anında doğrulanır).

## 7. Upload Sistemi

- **Chunked upload** (varsayılan 50 MB/chunk, 2 GB'a kadar testli), **resume** (manifest: chunk listesi + hash'ler), canlı progress (WebSocket).

```
File → 50MB chunk → 50MB chunk → ... → Reassemble → SHA-256 verify → Atomic move
```

- Chunk'lar panel staging alanına (`/srv/aurapanel/uploads/<site-id>/<upload-id>/`) yazılır; finalize: birleştir → **SHA-256 doğrula** → `file.op` ile site FS'e atomik taşı (sahiplik `www-siteNNN` korunur).
- Bağlantı kopsa upload baştan başlamaz.
- Progress örneği: `wordpress.zip — 1.4 GB / 2.0 GB · %70 · 18 MB/s · ETA 33s`

### 7.1 Upload Policy

| Kural | |
|---|---|
| Max file size / max total upload / max files per request | yapılandırılabilir (system_settings) |
| Extension policy | **tek başına güvenlik kontrolü değildir** |
| MIME validation | magic bytes ile (uzantıya güvenilmez) |
| Filename validation | NFC normalize; null byte + kontrol karakterleri red; ad içinde path ayraçları yok |

### 7.2 Executable File Policy

- ELF binary (magic: `\x7fELF`) ve shell script (shebang) tespiti → UI'da uyarı + audit.
- **PHP upload tamamen engellenmez** (hosting kullanımının normal parçası); güvenlik filesystem/process isolation üzerinden sağlanır.

## 8. Archive Güvenliği

- **Extraction hiçbir koşulda site root dışına yazamaz:** her entry için Clean + mutlak yol reddi + `..` reddi + çözümlenmiş hedefin root içinde kalma doğrulaması (zip-slip koruması).
- Archive içindeki **symlink'ler ayrıca kontrol edilir:** root dışına çıkan symlink entry'leri reddedilir; içerideki symlink'lere policy uygulanır.
- **Archive bomb korumaları:** maximum extracted size · maximum file count · compression ratio limiti · nested archive derinliği (iç içe archive otomatik açılmaz).
- Örnek: `1 MB ZIP → 100 GB extracted` engellenir.
- Extraction Tier 1'de (site UID + site cgroup + timeout) çalışır; `io.LimitedReader` vb. sınırlı okuyucular kullanılır.

## 9. Permissions Management

- Sınırsız chmod erişimi **yok**. Güvenli preset'ler: `644, 640, 600, 755, 750, 700`.
- UI: Owner/Group/Other → Read/Write/Execute checkbox'ları (yalnızca preset kümesine eşlenebilen kombinasyonlar).
- `777` için: güçlü uyarı → mümkünse engelleme → gerekçeli override (audit'li).
- Ownership değişikliği normal site kullanıcısına **açık değil** (sahip/grup yalnızca görüntülenir).

## 10. Preview Güvenliği

### 10.1 Metin
TXT, LOG, JSON, XML, HTML, CSS, JS, PHP source, Markdown, CSV — boyut limiti (varsayılan 1 MB, ilk N KB render edilir).

### 10.2 Görsel
PNG, JPEG, GIF, WebP, SVG.

- Content-Type doğrulaması (magic bytes; uzantıya güvenilmez)
- Image decoding limits: maksimum piksel/çözünürlük (decompression bomb koruması; varsayılan max 8192px kenar / 50MP)
- **SVG:** asla inline trusted HTML olarak render edilmez — yalnızca `<img src="preview-endpoint">` ile gösterilir; preview endpoint `X-Content-Type-Options: nosniff` + kısıtlayıcı CSP (sandbox) header'larıyla servis edilir. `<img>` bağlamında SVG script çalıştıramaz.
- PDF preview opsiyonel (MVP dışı); video/audio sonraki faza.

## 11. Search

- Kriterler: filename, extension, size, modified date. İçerik araması sonraki faza.
- Sınırlar: max derinlik (varsayılan 6), max sonuç (varsayılan 1000), timeout; recursive search site cgroup'unda çalışır — büyük sitelerde kaynak tüketimi sınırlıdır.

## 12. Resource Limits

File Manager işlemleri de site kaynak limitlerine uyar: **CPU, RAM, IO, PID** (Tier 1 cgroup attach ile otomatik).

- `zip -r /` benzeri bir işlemle sunucunun tüketilmesi engellenir (cgroup + timeout + sınırlı okuyucular).
- Archive ve recursive filesystem operasyonlarında timeout + resource limit zorunludur.

## 13. Concurrency — Optimistic Locking

- Aynı dosyada eş zamanlı iki edit → veri kaybı engellenir.
- `GET content` → `{content, file_version, modified_at, content_hash}`; `PUT content` → `{content, expected_version, expected_hash}`.
- Eşleşmezse **409**: `"File has changed on disk. Reload before saving."`

## 14. Rate Limiting

Site ve kullanıcı bazlı; varsayılanlar (yapılandırılabilir):

| Kategori | Varsayılan |
|---|---|
| Upload | 60 istek/dk + eş zamanlı 3 |
| Download | 60 istek/dk |
| Archive / Extract | 10 işlem/dk |
| Search | 30 istek/dk |
| Delete | 60 işlem/dk |

Aşım → HTTP 429, audit'li.

## 15. API Tasarımı

```
GET    /api/sites/{site}/files                 → list (path, page, page_size, sort, order, type, hidden)
GET    /api/sites/{site}/files/download        → dosya / klasör (ZIP)
POST   /api/sites/{site}/files/upload          → chunk (upload_id, index, sha256) / finalize
POST   /api/sites/{site}/files/mkdir
POST   /api/sites/{site}/files/create
POST   /api/sites/{site}/files/copy
POST   /api/sites/{site}/files/move
POST   /api/sites/{site}/files/rename
DELETE /api/sites/{site}/files                 → trash'e taşı (kalıcı silme ayrı akış)
POST   /api/sites/{site}/files/archive         → ZIP / TAR.GZ oluştur
POST   /api/sites/{site}/files/extract
GET    /api/sites/{site}/files/preview         → metin / görsel (nosniff + CSP)
GET    /api/sites/{site}/files/content         → {content, version, hash, modified_at}
PUT    /api/sites/{site}/files/content         → atomic save (optimistic lock)
POST   /api/sites/{site}/files/restore         → trash / revision restore
```

**Tüm endpoint'lerde zorunlu:** Authentication · Authorization · Site ownership · Path validation · Permission check · Rate limit · Audit log.

Hata kodları: `400` (geçersiz girdi/adi), `401`, `403` (site root dışı / symlink policy ihlali), `404`, `409` (edit çakışması), `413` (boyut aşımı), `429` (rate limit).

## 16. File Manager ve SFTP

```
File Manager ──┐
               ├── Site FS
SFTP ──────────┘
```

- İkisi **aynı filesystem'i** kullanır: web panelinden yüklenen dosya SFTP'de de görünür.
- SFTP kullanıcısı da site root dışına çıkamaz (jail, bkz. ARCHITECTURE §11.2).

## 17. Performans

- Büyük dizinler tek JSON response olarak gönderilmez: **server-side pagination** (varsayılan 200 kayıt/sayfa) + lazy loading.
- 100.000 dosyalık dizinde akıcılık hedefi.

## 18. Audit Log

Kaydedilen işlemler: Upload, Download, Create file, Create directory, Rename, Move, Copy, Delete, Restore, Empty trash, chmod, ownership change, Edit, Archive create, Archive extract.

Şema:

```
timestamp, user, IP, site_id, path, action, result, request_id
```

**Dosya içeriği asla audit log'a yazılmaz.**

## 19. MVP Kapsamı

Browse · Upload · Multi-upload · Drag & drop · Chunked upload · Resume upload · Download · Create file · Create directory · Rename · Move · Copy · Delete · Trash · Restore · ZIP · TAR.GZ · Extract · Search · Sort · Hidden files · File info · Permissions · Code editor · Text preview · Image preview · Audit log · Path traversal protection · Symlink escape protection · Archive traversal protection · Archive bomb protection · Site-root isolation · Rate limiting

## 20. Öncelik Sırası ve İlkeler

```
Security → Site Isolation → Data Integrity → Reliability → Performance → UX
```

1. Her filesystem işlemi §2 pipeline'ından geçer; istisna yok.
2. FM hiçbir koşulda başka sitenin veya işletim sisteminin dosyalarına erişemez.
3. Kullanıcı girdisi asla `os.Open(userInput)` veya `exec(...)` biçiminde kullanılmaz.
4. Dosya içeriği asla log'a yazılmaz.
5. Uzantı ve MIME, tek başına güvenlik kontrolü değildir.
6. Safe Mode MVP'de varsayılan olarak açıktır.
7. Güvenlik filesystem/process isolation üzerinden sağlanır (PHP upload engellenmez).

## 21. Nihai Hedef

> "cPanel tarzı kolaylık + gerçek site izolasyonu + modern dosya yönetimi + güvenli filesystem abstraction"

File Manager, AuraPanel MVP'sinin **çekirdek modüllerinden biridir**; geliştirilirken ARCHITECTURE.md §13'teki değiştirilemez ilkelerle birlikte bu dokümandaki ilkeler bağlayıcıdır.
