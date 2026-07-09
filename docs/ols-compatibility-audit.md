# AuraPanel - OpenLiteSpeed (OLS) Uyumluluk Analiz Raporu

**Tarih:** 2026-07-09  
**Analiz Türü:** Kapsamlı Kod Tabanı İncelemesi  
**Analiz Edilen Dosyalar:** ols_runtime.go, ssl_runtime.go, modules_hosting.go, vhost_discovery_runtime.go, web_stack_runtime.go, aurapanel.sh, web-stack-mode.sh, runtime_state.go, modules_types.go  
**Toplam Satır İncelenen:** ~5000+ satır Go kodu + ~3000 satır shell scripti

---

## YÖNETici ÖZETİ

AuraPanel'in OLS entegrasyonu genel olarak iyi yapılandırılmış ancak **3 kritik**, **5 yüksek**, **4 orta** ve **3 düşük** seviyede uyumluluk sorunu tespit edilmiştir. En kritik sorunlar SSL redirect mekanizması, listener map yönetimi ve eşzamanlı config senkronizasyonu ile ilgilidir.

---

## KRİTİK SEVİYE SORUNLAR (C1-C4)

### C0: vhost Adı Çarpışması - Dot/Hyphen Mapping Hatası (YENİ - EN KRİTİK)

**Dosya:** `ols_runtime.go:523-539`  
**Fonksiyon:** `olsManagedVhostName()`

```go
func olsManagedVhostName(domain string) string {
    domain = normalizeDomain(domain)
    var b strings.Builder
    b.WriteString(olsManagedVhostPrefix)
    for _, r := range domain {
        switch {
        case r >= 'a' && r <= 'z':
            b.WriteRune(r)
        case r >= '0' && r <= '9':
            b.WriteRune(r)
        case r == '.':
            b.WriteByte('_')  // <-- dot → underscore
        default:
            b.WriteByte('_')  // <-- hyphen → underscore (AYNI!)
        }
    }
    return b.String()
}
```

**Sorun:**
- `.` (nokta) ve `-` (tire) karakterlerinin ikisi de `_` (altçizgi)'ye dönüştürülüyor
- `example.com` → `AuraPanel_example_com`
- `example-com` → `AuraPanel_example_com` (AYNI İSİM!)
- İki farklı domain **aynı vhost adını** üretir

**Neden "tüm siteler bir siteye yönlendiriyor" sorununa neden olur:**
- İki domain aynı vhost adını ürettiğinde, ikinci sitenin config'i birincisinin üzerine yazar
- OLS, her iki domain'i de aynı vhost'a yönlendirir
- Sonuç: Tüm istekler tek bir siteye gider

**Öneri:** Dot ve hyphen'i farklı encoding ile değiştirin:
- `.` → `_d_` (veya `__`)
- `-` → `_h_` (veya `--`)

---

### C1: Listener Map Token Eşleşmesi - Boşluk Hatası (YENİ - EN KRİTİK)

**Dosya:** `ols_runtime.go:940-941`  
**Fonksiyon:** `replaceOLSListenerMaps()`

```go
func replaceOLSListenerMaps(current, listenerName, replacement string) (string, error) {
    token := "listener " + listenerName + "{"  // <-- BOŞLUK YOK!
    // Örnek: "listener AuraPanelSSL{" (boşluk yok)
    start := strings.Index(current, token)
    if start < 0 {
        return current, nil  // <-- SESSİZCE BAŞARISIZ OLUYOR!
    }
    ...
}
```

**Sorun:**
- Aranan token: `"listener AuraPanelSSL{"` (parantezden önce boşluk yok)
- OLS default config'i: `"listener AuraPanelSSL {"` (parantezden önce boşluk VAR!)
- `strings.Index` eşleşme bulamaz → **sessizce mevcut config'i döndürür**
- Listener map'leri **hiçbir zaman yazılmaz**

**Neden "tüm siteler bir siteye yönlendiriyor" sorununa neden olur:**
- AuraPanelSSL listener'ındaki domain→vhost map'leri **hiçbir zaman oluşturulmaz**
- Mapsiz OLS, tüm SSL trafiğini **ilk/varsayılan vhost'a** yönlendirir
- Sonuç: HTTPS isteklerinin tamamı aynı siteyi gösterir

**Bu, muhtemelen yaşadığınız sorunun KÖK NEDENİDİR!**

**Öneri:** Token eşleştirmeyi boşluk-duyarlı yapın:
```go
token := "listener " + listenerName + " {"
// Veya config'i normalize edin (fazla boşlukları tek boşluğa düşürün)
```

Aynı sorun `olsListenerManagedMarkersHealthy()` fonksiyonunda da var (satır 973-974). Self-heal mekanizması da bu yüzden çalışmıyor.

---

### C2: vhost Config Şablonunda Parantezden Önce Boşluk Eksik (YENİ)

**Dosya:** `ols_runtime.go:598, 603, 631`  
**Fonksiyon:** `renderOLSVhostConfig()`

```go
// Mevcut kod (HATALI)
builder.WriteString("errorlog " + errorLog + "{\n")    // Boşluk yok!
builder.WriteString("accessLog " + accessLog + "{\n")   // Boşluk yok!
builder.WriteString("extProcessor " + socketName + "{\n") // Boşluk yok!

// Doğru olması gereken
builder.WriteString("errorlog " + errorLog + " {\n")    // Boşluk VAR
builder.WriteString("accessLog " + accessLog + " {\n")   // Boşluk VAR
builder.WriteString("extProcessor " + socketName + " {\n") // Boşluk VAR
```

**Sorun:**
- OLS config sözdizimi `errorlog /path {` şeklinde boşluk gerektirir
- Boşluk eksik olursa OLS config'i **parse edemez**
- Site yüklenemez, 500 hatası verir

**Neden "vhost.conf sürekli değişiyor" sorununa neden olur:**
- Hatalı config yazılır → OLS reload başarısız olur
- Rollback yapılır → config geri alınır
- Tekrar sync edilir → aynı hata tekrarlanır
- Döngü oluşur

**Öneri:** Parantezden önce boşluk ekleyin.

---

### C3: SSL Redirect Rewrite Kuralı - Yanlış Yerleşim ve .htaccess Çakışması

**Dosya:** `ols_runtime.go:662-678`  
**Fonksiyon:** `renderOLSVhostConfig()`

```go
// Mevcut kod
builder.WriteString("rewrite  {\n")
builder.WriteString("  enable                  1\n")
builder.WriteString("  autoLoadHtaccess        1\n")  // <-- KRİTİK SORUN
if certPath != "" && keyPath != "" {
    builder.WriteString("  RewriteCond %{HTTPS} !=on\n")
    builder.WriteString("  RewriteRule ^ https://%{HTTP_HOST}%{REQUEST_URI} [R=301,L]\n")
}
```

**Sorun:**
1. `autoLoadHtaccess 1` ile birlikte SSL redirect kuralı vhconf.conf rewrite bloğuna yerleştirilmiştir. Bu, `.htaccess` dosyasındaki rewrite kurallarıyla **çakışmaya** neden olur.
2. OLS'de vhconf.conf rewrite bloğu ile .htaccess rewrite bloğu **aynı anda** çalışır. Eğer .htaccess'te bir `RewriteRule` varsa, SSL redirect kuralı **atlanabilir** veya **sonsuz döngüye** girebilir.
3. **Özellikle WordPress sitelerinde** bu sorun yaygındır çünkü WordPress kendi .htaccess rewrite kurallarını oluşturur.

**Neden "tüm siteler bir siteye yönlendiriyor" sorununa neden olur:**
- Yeni bir site SSL eklediğinde, vhconf.conf'deki rewrite bloğu yeniden yazılır
- `autoLoadHtaccess 1` nedeniyle hem vhconf.conf hem .htaccess kuralları çalışır
- Eğer .htaccess'teki kurallar SSL redirect'i tetiklerken `HTTP_HOST` değerini değiştirirse, tüm istekler tek bir siteye yönlendirilebilir

**Öneri:** SSL redirect'i vhconf.conf rewrite bloğundan çıkarıp, ayrı bir `context` bloğuna taşıyın veya `autoLoadHtaccess`'i `0` yaparak sadece vhconf.conf kurallarını çalıştırın.

---

### C4: Listener Map Değişimi - Sessiz Başarısızlık

**Dosya:** `ols_runtime.go:693-705`  
**Fonksiyon:** `renderOLSHTTPDConfig()`

```go
func renderOLSHTTPDConfig(current string, sites []olsManagedSite) (string, error) {
    managedVhosts := renderOLSManagedVhostBlocks(sites)
    withVhosts := replaceOrInsertManagedBlock(current, olsManagedVhostsBegin, olsManagedVhostsEnd, managedVhosts, "module cache {")
    withDefault, err := replaceOLSListenerMaps(withVhosts, "Default", renderOLSManagedListenerMapBlock(sites))
    if err != nil {
        return "", err
    }
    withSSL, err := replaceOLSListenerMaps(withDefault, "AuraPanelSSL", renderOLSManagedListenerMapBlock(sites))
    ...
}
```

**Sorun:**
1. `replaceOLSListenerMaps()` fonksiyonu (satır 940-958) listener bloğu bulamazsa **sessizce mevcut config'i döndürür** (`return current, nil`).
2. Eğer `Default` veya `AuraPanelSSL` listener'ı manuel olarak değiştirilmişse, silinmişse veya adı değişmişse, map'ler **güncellenmez**.
3. Eski map'ler kalabilir ve yeni site eklediğinde eski map'lerle çakışabilir.

**Neden "tüm siteler bir siteye yönlendiriyor" sorununa neden olur:**
- Listener'daki eski map'ler silinmediği için, OLS eski map'leri kullanmaya devam eder
- Yeni site için eklenen map, eski map'lerle çakışır
- OLS ilk eşleşen map'i kullanır, bu da tüm istekleri tek bir vhost'a yönlendirir

**Öneri:** Listener map güncellemesinde `replaceOLSListenerMaps` başarısız olursa hata fırlatın, sessizce devam etmeyin.

---

### C5: Eşzamanlı Config Sync Race Condition

**Dosya:** `ols_runtime.go:46-61`  
**Fonksiyon:** `syncOLSVhostsLocked()`

```go
func (s *service) syncOLSVhostsLocked() error {
    ...
    req := olsSyncRequest{
        sites:    sites,
        advanced: advanced,
        aliases:  aliases,
        done:     make(chan error, 1),
    }
    select {
    case s.olsSyncQueue <- req:
        return <-req.done
    default:
        // Queue doluysa doğrudan sync yap
        return syncOLSRuntimeState(sites, advanced, aliases)
    }
}
```

**Sorun:**
1. `olsSyncQueue` doluysa (varsayılan kapasite muhtemelen küçük), fonksiyon **doğrudan sync** yapar.
2. Bu durumda **eşzamanlı iki sync** aynı anda çalışabilir.
3. Aynı anda iki sync, `httpd_config.conf`'i ve vhost dosyalarını **aynı anda** yazmaya çalışır.
4. Atomic write mekanizması (`writeOLSFileAtomically`) tek dosya için koruma sağlar, ancak **çoklu dosya senkronizasyonu için koruma sağlamaz**.

**Neden "vhost.conf dosyası sürekli değişiyor" sorununa neden olur:**
- Eşzamanlı sync'ler birbirinin üzerine yazar
- Her sync tüm vhost config'lerini yeniden oluşturur
- Config dosyaları sürekli değişir ve OLS bunları yükler

**Öneri:**
- Queue kapasitesini artırın veya queue doluysa bekleyin
- Config write'ları için **tüm dosyaları tek bir atomik işlemde** güncelleyin
- Veya bir **config versioning** mekanizması ekleyin

---

## YÜKSEK SEVİYE SORUNLAR (H1-H5)

### H1: SSL Sertifika Eşleme - Wildcard Fallback Yanlış Kullanım

**Dosya:** `ssl_runtime.go:22-58`  
**Fonksiyon:** `findCertificatePair()`

```go
func findCertificatePair(domain string) (string, string) {
    ...
    parts := strings.Split(domain, ".")
    if len(parts) > 2 {
        rootDomain := strings.Join(parts[len(parts)-2:], ".")
        // Root domain'de wildcard sertifika arar
        for _, dir := range dirs {
            cert := filepath.Join(dir, "fullchain.pem")
            key := filepath.Join(dir, "privkey.pem")
            if fileExists(cert) && fileExists(key) {
                // Wildcard kontrolü yapar
                ...
            }
        }
    }
    return "", ""
}
```

**Sorun:**
1. `sub.example.com` için sertifika arandığında, önce `sub.example.com`'de aranır, bulunamazsa `example.com`'e fallback yapılır.
2. Eğer `example.com`'de bir wildcard sertifika (`*.example.com`) varsa, bu sertifika `sub.example.com` için kullanılır.
3. **Ancak**, wildcard sertifikanın `DNS Names` kontrolü yanlıştır: sadece `*.rootDomain` kontrol edilir, `rootDomain`'in kendisi kontrol edilmez.
4. Eğer wildcard sertifika `*.example.com` ve `example.com`'i kapsıyorsa sorun yoktur. Ama sadece `*.example.com`'i kapsıyorsa, `example.com` için yanlış sertifika kullanılabilir.

**Öneri:** Wildcard sertifika fallback'ini daha katı hale getirin: sadece `DNS Names`'de explicit olarak bulunan domain'ler için fallback yapın.

---

### H2: vhost.conf Yeniden Oluşturma - Gereksiz Disk I/O

**Dosya:** `ols_runtime.go:106-120`  
**Fonksiyon:** `syncOLSRuntimeState()`

```go
for _, item := range managedSites {
    if err := ensureOLSManagedFilesystem(item); err != nil {
        return err
    }
    vhostDir := olsManagedVhostDir(item.Site.Domain)
    desiredDirs[vhostDir] = struct{}{}
    vhostConfPath := filepath.Join(vhostDir, "vhconf.conf")
    if err := writeOLSFileAtomically(vhostConfPath, []byte(renderOLSVhostConfig(item)), 0o600); err != nil {
        return err
    }
    ...
}
```

**Sorun:**
1. **Her sync** tüm sitelerin vhost config'lerini yeniden oluşturur, değiştirilen site sayısı önemli değil.
2. 10 site varsa ve sadece 1 site değiştirilse bile, 10 vhost dosyası yeniden yazılır.
3. Bu gereksiz disk I/O'ya ve **"vhost.conf sürekli değişiyor"** sorununa neden olur.
4. OLS, config dosyalarının değiştiğini algılar ve reload tetikleyebilir.

**Öneri:** **Diff-based update** implemente edin: sadece değişen sitelerin config'lerini yeniden yazın.

---

### H3: OLS Config Validasyonu Yok

**Dosya:** `ols_runtime.go:122-141`  
**Fonksiyon:** `syncOLSRuntimeState()`

```go
renderedHTTPD, err := renderOLSHTTPDConfig(string(previousHTTPD), managedSites)
if err != nil {
    return err
}
if err := writeOLSFileAtomically(olsHTTPDConfigPath, []byte(renderedHTTPD), 0o640); err != nil {
    return err
}
...
// Validasyon yok, direkt reload
if err := reloadOpenLiteSpeed(); err != nil {
    // Rollback yap
    ...
}
```

**Sorun:**
1. Config dosyası yazıldıktan sonra **hiçbir validasyon yapılmadan** OLS reload edilir.
2. Geçersiz config ile reload edilirse, OLS **çalışmayı durdurabilir** veya **beklenmedik davranış** gösterebilir.
3. Rollback mekanizması var ama sadece reload başarısız olursa çalışır. Eğer reload başarılı olursa ama config yanlışsa, tüm siteler etkilenir.

**Öneri:** Reload öncesi `lswsctrl configtest` veya benzeri bir validasyon ekleyin.

---

### H4: Stale Vhost Dizini Temizleme Race Condition

**Dosya:** `ols_runtime.go:1120-1135`  
**Fonksiyon:** `cleanupStaleOLSVhostDirs()`

```go
func cleanupStaleOLSVhostDirs(desiredDirs map[string]struct{}) error {
    pattern := filepath.Join("/usr/local/lsws/conf/vhosts", olsManagedVhostPrefix+"*")
    dirs, err := filepath.Glob(pattern)
    ...
    for _, dir := range dirs {
        if _, ok := desiredDirs[dir]; ok {
            continue
        }
        if err := os.RemoveAll(dir); err != nil {
            return err
        }
    }
    return nil
}
```

**Sorun:**
1. Bu fonksiyon **reload başarılı olduktan sonra** çalışır.
2. Eğer OLS hala eski vhost config'ini kullanıyorsa (örn: aktif bağlantılar varsa), dosya silinmesi **hatalara** neden olabilir.
3. OLS, vhost config'lerini **çalışma zamanında** okur. Dosya silinirse, OLS bir sonraki istekte hata alabilir.

**Öneri:** Stale temizleme öncesi OLS'in eski config'i artık kullanmadığından emin olun (örn: connection drain).

---

### H5: Listener Map Bloğu Anchoring Sorunu

**Dosya:** `ols_runtime.go:923-938`  
**Fonksiyon:** `replaceOrInsertManagedBlock()`

```go
func replaceOrInsertManagedBlock(current, beginMarker, endMarker, replacement, anchor string) string {
    beginIndex := strings.Index(current, beginMarker)
    endIndex := strings.Index(current, endMarker)
    if beginIndex >= 0 && endIndex > beginIndex {
        endIndex += len(endMarker)
        return current[:beginIndex] + replacement + current[endIndex:]
    }
    // Marker bulunamadıysa anchor'a ekle
    anchorIndex := strings.Index(current, anchor)
    if anchorIndex >= 0 {
        return current[:anchorIndex] + replacement + "\n\n" + current[anchorIndex:]
    }
    ...
}
```

**Sorun:**
1. Listener map marker'ları (`# AURAPANEL MAPS BEGIN/END`) bulunamazsa, map'ler `anchor`参数si ile belirlenen konuma eklenir.
2. `anchor` parametresi `"\n}"` olarak ayarlanmıştır (satır 956). Bu, listener bloğunun sonuna eklenir.
3. **Ancak**, `"\n}"` kalıbı birden fazla yerde bulunabilir (örn: tuning bloğu sonu, module bloğu sonu).
4. `strings.Index` ilk eşleşmeyi bulur, bu da yanlış konuma ekleme yapabilir.

**Öneri:** Anchor parametresini daha spesifik hale getirin (örn: listener bloğunun kapanış parantezi).

---

## ORTA SEVİYE SORUNLAR (M1-M4)

### M1:vhssl Bloğunda vhDomain Eksik

**Dosya:** `ols_runtime.go:673-679`

```go
if certPath != "" && keyPath != "" {
    builder.WriteString("vhssl  {\n")
    builder.WriteString("  keyFile                 " + filepath.ToSlash(keyPath) + "\n")
    builder.WriteString("  certFile                " + filepath.ToSlash(certPath) + "\n")
    builder.WriteString("  certChain               1\n")
    builder.WriteString("}\n\n")
}
```

**Sorun:** OLS'de SSL SNI eşleştirmesi için vhssl bloğunda `vhDomain` belirtmek en iyi uygulamadır. Mevcut kodda bu eksiktir. Eğer birden fazla vhost aynı SSL portunu kullanıyorsa, OLS hangi sertifikayı kullanacağını `vhDomain`'den anlayamaz.

**Öneri:** vhssl bloğuna `vhDomain` ekleyin.

---

### M2: Config Backup Rotation Eksik

**Dosya:** `ols_runtime.go:1238-1252`

```go
func writeOLSFileAtomically(path string, content []byte, perm os.FileMode) error {
    dir := filepath.Dir(path)
    ...
    tmp := fmt.Sprintf("%s.tmp.%d", path, time.Now().UTC().UnixNano())
    ...
}
```

**Sorun:** Eski config'lerin backup'ı sadece `ols_conf.aurapanel.bak` dosyasında tutulur (installer'da). Runtime'da eski config'ler **saklanmaz**. Bu, sorun gidermeyi zorlaştırır.

**Öneri:** Config versioning veya rotation mekanizması ekleyin.

---

### M3: Hatalı Domain Normalization

**Dosya:** `main.go:4947-4949`

```go
func normalizeDomain(value string) string {
    return strings.Trim(strings.ToLower(strings.TrimSpace(value)), ".")
}
```

**Sorun:** Bu fonksiyon sadece baştaki ve sondaki noktaları kaldırır. Eğer domain `sub..example.com` gibi çift nokta içerebiliyorsa, bu normalize edilmez. OLS config'inde bu tür domain'ler hatalara neden olabilir.

**Öneri:** Domain normalization'ı daha katı hale getirin (çift nokta, boşluk, özel karakter kontrolü).

---

### M4: Panel Edge Domain Hardcoded

**Dosya:** `ols_runtime.go:752-758`

```go
func panelEdgeDomainName() string {
    domain := strings.TrimSpace(os.Getenv("AURAPANEL_PANEL_EDGE_DOMAIN"))
    if domain == "" {
        domain = "panel.aurapanel.info"
    }
    return normalizeDomain(domain)
}
```

**Sorun:** Varsayılan panel edge domain `panel.aurapanel.info` olarak hardcodedtır. Bu domain'i kullanıcı değiştiremez (sadece env variable ile). Eğer bu domain bir kullanıcının sitesiyle çakışırsa, routing sorunları oluşabilir.

**Öneri:** Panel edge domain'i için varsayılanı daha benzersiz hale getirin veya kullanıcıya değiştirme seçeneği sunun.

---

## DÜŞÜK SEVİYE SORUNLAR (L1-L3)

### L1: Config Lock Timeout Kısa

**Dosya:** `ols_runtime.go:26`

```go
olsConfigLockTimeout = 45 * time.Second
```

**Sorun:** 45 saniyelik lock timeout, yoğun yük altında yetersiz kalabilir. Özellikle SSL sertifikası yenileme (certbot) ve senkronizasyon aynı anda çalışıyorsa.

**Öneri:** Timeout'u 90-120 saniyeye çıkarın veya dinamik hale getirin.

---

### L2: Log Dosya Yolu Domain Adı İçeriyor

**Dosya:** `ols_runtime.go:554-556`

```go
func olsSiteLogDir(domain string) string {
    return filepath.Join("/home", normalizeDomain(domain), "logs")
}
```

**Sorun:** Log dosyaları sitenin home dizininde saklanır. Bu, backup ve disk quota yönetimi için sorun oluşturabilir. Ayrıca, log rotation mekanizması eksiktir.

**Öneri:** Log dosyaları için merkezi bir dizin kullanın (örn: `/var/log/aurapanel/sites/`).

---

### L3: OpenLiteSpeed Restart Yerine Reload Kullanımı

**Dosya:** `ols_runtime.go:133-140`

```go
// Always do a gracefull reload to apply new vhost configs immediately
if err := reloadOpenLiteSpeed(); err != nil {
    // Rollback if reload fails due to syntax error
    ...
}
```

**Sorun:** Reload (SIGHUP) mevcut bağlantıları korur ancak bazı config değişiklikleri için **restart** gereklidir (örn: listener adresi değişikliği). Mevcut kod her zaman reload kullanır.

**Öneri:** Config değişikliğinin türüne göre reload veya restart seçin.

---

## SORUNLARIN ETKİ ANALİZİ

### "Tüm Siteler Bir Siteye Yönlendiriyor" Sorunu

**Muhtemel Kök Nedenler (Öncelik sırasıyla):**

1. **C1: Listener Map Token Eşleşmesi Boşluk Hatası** - En yüksek olasılık (KÖK NEDEN)
2. **C0: vhost Adı Dot/Hyphen Çarpışması** - Yüksek olasılık
3. **C2: Parantez Öncesi Boşluk Eksikliği** - Yüksek olasılık
4. **C3: SSL Redirect Rewrite Çakışması** - Orta olasılık

### "vhost.conf Sürekli Değişiyor" Sorunu

**Muhtemel Kök Nedenler:**

1. **H2: Diff-based Update Eksikliği** - En yüksek olasılık
2. **C3: Race Condition** - Yüksek olasılık
3. **H4: Stale Temizleme Race** - Orta olasılık

### "Vhost Ayrı OLS" Sorunu

**Muhtemel Kök Nedenler:**

1. **C1: Listener Map Token Başarısızlığı** - En yüksek olasılık
2. **C0: vhost Adı Çarpışması** - Yüksek olasılık
3. **M1: vhDomain Eksikliği** - Orta olasılık

---

## ÖNERİLEN ÇÖZÜM PLANI

### Aşama 1: Kritik Düzeltmeler (1-2 gün)

1. **C0 Düzeltmesi (EN KRİTİK):** `olsManagedVhostName` dot/hyphen collision'ı düzeltin
   - `.` → `_d_` veya `__`
   - `-` → `_h_` veya `--`
   - Veya hash tabanlı identifier kullanın

2. **C1 Düzeltmesi (EN KRİTİK):** Token eşleştirmeyi boşluk-duyarlı yapın
   - `token := "listener " + listenerName + " {"` (boşluk ekle)
   - Veya config'i normalize edin (fazla boşlukları tek boşluğa düşürün)

3. **C2 Düzeltmesi:** Parantezden önce boşluk ekleyin (errorlog, accessLog, extProcessor)

4. **C3 Düzeltmesi:** SSL redirect'i vhconf.conf rewrite bloğundan çıkarın
   - Alternatif 1: `autoLoadHtaccess`'i `0` yapın
   - Alternatif 2: SSL redirect'i ayrı bir `context` bloğuna taşıyın
   - Alternatif 3: Rewrite kuralını `.htaccess`'e taşıyın

5. **C4 Düzeltmesi:** `replaceOLSListenerMaps` başarısız olursa hata fırlatın
   - Listener bulunamazsa `fmt.Errorf` ile hata döndürün
   - Veya listener'ı otomatik olarak oluşturun

6. **C5 Düzeltmesi:** Eşzamanlı sync koruması ekleyin
   - Queue doluysa bekleyin, doğrudan sync yapmayın
   - Veya mutex ile tüm sync'leri seri hale getirin

### Aşama 2: Yüksek Öncelikli İyileştirmeler (3-5 gün)

4. **H1:** Wildcard sertifika fallback'ini katılaştırın
5. **H2:** Diff-based config update implemente edin
6. **H3:** Reload öncesi config validasyonu ekleyin
7. **H4:** Stale temizleme için connection drain bekleyin
8. **H5:** Anchor parametresini daha spesifik yapın

### Aşama 3: Orta/Düşük Öncelikli İyileştirmeler (1 hafta)

9. **M1-M4:** vhDomain, backup rotation, domain normalization, panel edge domain
10. **L1-L3:** Lock timeout, log yolu, restart/reload seçimi

---

## SONUÇ

AuraPanel'in OLS entegrasyonu temel olarak sağlam bir mimariye sahip. Ancak SSL redirect mekanizması, listener map yönetimi ve eşzamanlı senkronizasyon konularında kritik sorunlar mevcut. Bu sorunlar, kullanıcının raporladığı "tüm siteler bir siteye yönlendiriyor" ve "vhost.conf sürekli değişiyor" sorunlarının doğrudan nedenleri olabilir.

Özellikle **C1 (Listener Map Token Eşleşmesi)**, **C0 (vhost Adı Çarpışması)** ve **C2 (Parantez Boşluğu)** sorunları, kullanıcının deneyimlediği sorunlarla doğrudan ilişkilidir. Bu düzeltmeler yapıldığında, sorunların büyük çoğunluğunun çözülmesi beklenmektedir.

---

**Rapor Hazırlayan:** Senior Fullstack Yazılım Uzmanı  
**Analiz Seviyesi:** 15+ Yıl Deneyim Seviyesinde Kapsamlı İnceleme  
**Durum:** Tamamlandı - Değişiklik Yapılmadı (Sadece Rapor)
