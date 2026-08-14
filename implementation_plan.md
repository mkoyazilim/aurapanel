# AuraPanel Faz Tamamlama Planı

Kalan MVP özelliklerinin tamamlanması için `ROADMAP.md` referans alınarak sırasıyla yürütülecek iş paketleri:

## Proposed Changes

### 1. W7: Let's Encrypt (SSL) Doğrulaması
* `mkoyazilim.com` sitesine HTTP-01 solver üzerinden Let's Encrypt sertifikası kurulumunun VPS'te uçtan uca doğrulanması.
* Otomatik yenileme (auto-renew) scheduler'ının testi.

### 2. W8: MariaDB Yönetimi ve Adminer
* Veritabanı ve kullanıcı CRUD operasyonlarının (ayrıcalıklı SQL injection korumalarıyla) test edilmesi.
* Geçici token tabanlı izole Adminer oturumunun açılması.

### 3. W9: SFTP (W9.4) ve Dosya Yöneticisi (W9.3)
* `sshd -t` destekli OpenSSH jail kurulumu ve SFTP erişimi doğrulaması.
* Dosya yöneticisi arayüzünde (File Manager) WebSocket tabanlı upload progress bar entegrasyonu.
* Server-side pagination.

### 4. W10 & W11: Yedekleme ve Güvenlik
* Veritabanı (mysqldump) ve S3/MinIO yedekleme motorunun entegrasyonu.
* WebAuthn (Passkey) ve Google reCAPTCHA sistemlerinin aktifleşmesi.

### 5. W12 & W14: Son Arayüz Dokunuşları ve Güncelleme Akışı
* WebSocket canlı logları ve sistem kaynak grafikleri (Dashboard).
* OLS/LSPHP güvenli güncelleme pipeline'ının (rollback dahil) testi.
* Kurulum Script'i (Installer) son doğrulama (versions.json ile).

## Verification Plan
Her adım tamamlandığında, sunucuda çalışırlığı doğrudan test edilecek ve çalışmayan noktalar koda yansıtılacaktır. En son adımda AuraPanel %100 kullanıma hazır hale gelmiş olacaktır.
