# OLS Uyumluluk Düzeltmeleri - Değişiklik Günlüğü

**Tarih:** 2026-07-09  
**Dosya:** `panel-service/ols_runtime.go`  
**Durum:** Tüm düzeltmeler uygulandı ve testler başarılı

---

## Yapılan Düzeltmeler

### C0: vhost Adı Dot/Hyphen Collision (KRİTİK)
**Satır:** 523-540  
**Sorun:** `example.com` ve `example-com` aynı vhost adını üretiyordu (`AuraPanel_example_com`)  
**Çözüm:** 
- `.` → `__` (çift altçizgi)
- `-` → `--` (çift tire)

```go
// Eski
case r == '.':
    b.WriteByte('_')
default:
    b.WriteByte('_')

// Yeni
case r == '.':
    b.WriteString("__")
case r == '-':
    b.WriteString("--")
default:
    b.WriteByte('_')
```

---

### C1: Listener Map Token Eşleştirmesi (KRİTİK)
**Satır:** 948-960, 989-1001  
**Sorun:** `"listener AuraPanelSSL{"` arıyordu, OLS'de `"listener AuraPanelSSL {"` (boşluklu)  
**Çözüm:** Önce boşluklu versiyonu arar, bulunamazsa boşlukuz versiyonu dener

```go
token := "listener " + listenerName + " {"
start := strings.Index(current, token)
if start < 0 {
    // Legacy configs için fallback
    token = "listener " + listenerName + "{"
    start = strings.Index(current, token)
}
```

---

### C2: Parantez Öncesi Boşluk (KRİTİK)
**Satır:** 600, 605, 633  
**Sorun:** `errorlog /path{` - boşluk yok, OLS parse edemiyor  
**Çözüm:** Parantezden önce boşluk eklendi

```go
// Eski
builder.WriteString("errorlog " + errorLog + "{\n")
builder.WriteString("accessLog " + accessLog + "{\n")
builder.WriteString("extProcessor " + socketName + "{\n")

// Yeni
builder.WriteString("errorlog " + errorLog + " {\n")
builder.WriteString("accessLog " + accessLog + " {\n")
builder.WriteString("extProcessor " + socketName + " {\n")
```

---

### C3: SSL Redirect Konumu (KRİTİK)
**Satır:** 663-674  
**Sorun:** SSL redirect `rewrite` bloğundaydı, .htaccess ile çakışıyordu  
**Çözüm:** SSL redirect ayrı bir `context /` bloğuna taşındı (daha yüksek öncelik)

```go
// Yeni: SSL redirect context bloğunda (rewrite'tan önce çalışır)
if certPath != "" && keyPath != "" {
    builder.WriteString("context / {\n")
    builder.WriteString("  allowBrowse             1\n")
    builder.WriteString("  rewrite  {\n")
    builder.WriteString("    enable                1\n")
    builder.WriteString("    RewriteCond           %{HTTPS} !=on\n")
    builder.WriteString("    RewriteRule           ^ https://%{HTTP_HOST}%{REQUEST_URI} [R=301,L]\n")
    builder.WriteString("  }\n")
    builder.WriteString("}\n\n")
}
```

---

### C4: Listener Map Sessiz Başarısızlık (YÜKSEK)
**Satır:** 703-715, 948-960  
**Sorun:** Listener bulunamazsa sessizce devam ediyordu  
**Çözüm:** `required` parametresi eklendi, `Default` listener zorunlu

```go
func replaceOLSListenerMaps(current, listenerName, replacement string, required bool) (string, error) {
    ...
    if start < 0 {
        if required {
            return "", fmt.Errorf("required listener %s not found", listenerName)
        }
        log.Printf("optional listener %s not found; skipping", listenerName)
        return current, nil
    }
}
```

---

### C5: Eşzamanlı Sync Race Condition (YÜKSEK)
**Satır:** 55-62  
**Sorun:** Queue doluysa eşzamanlı doğrudan sync yapılıyordu  
**Çözüm:** `default` case kaldırıldı, her zaman queue'ya gönderiyor

```go
// Eski
select {
case s.olsSyncQueue <- req:
    return <-req.done
default:
    return syncOLSRuntimeState(sites, advanced, aliases)
}

// Yeni
s.olsSyncQueue <- req
return <-req.done
```

---

### Ek: Marker Order Validation (ORTA)
**Satır:** 975-989  
**Sorun:** Marker'ların sırası kontrol edilmiyordu  
**Çözüm:** Begin marker'ın end marker'dan önce geldiği doğrulandı

```go
if strings.Index(content, olsManagedVhostsBegin) > strings.Index(content, olsManagedVhostsEnd) {
    return false
}
```

---

## Test Sonuçları

```
ok  github.com/aurapanel/panel-service  36.443s
```

Tüm testler başarılı. Frontend ve API gateway de başarıyla derlendi.

---

## Geriye Uyumluluk Notu

**C0 düzeltmesi** vhost dizin isimlerini değiştirir:
- Eski: `/usr/local/lsws/conf/vhosts/AuraPanel_example_com/`
- Yeni: `/usr/local/lsws/conf/vhosts/AuraPanel_example__com/`

Mevcut sunucularda migration gerekebilir. Yeni kurulumlarda sorun yoktur.
