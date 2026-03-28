# AuraPanel İleri Seviye (Senior) Geliştirme Planı

Bu belge, AuraPanel'in sektör devleriyle (cPanel, Plesk, CyberPanel) tam rekabet edebilmesi ve modern hosting ihtiyaçlarını eksiksiz karşılayabilmesi için planlanan modüllerin adım adım izleneceği ana yol haritasıdır. Yeni bir konuşma penceresine geçildiğinde bu dosya üzerinden ilerleme durumu (Status) takip edilecektir.

## 📦 Modül 1: Dosya Yöneticisi (File Manager) Geliştirmeleri
- [x] **Backend (Rust):** Dosya/Dizin sıkıştırma (Compress - Zip/Tar) endpoint'inin yazılması.
- [x] **Backend (Rust):** Sıkıştırılmış dosyayı çıkarma (Extract - Unzip) endpoint'inin yazılması.
- [x] **Backend (Rust):** Güvenli silme (Çöp Kutusu / Trash) mantığının `.trash` dizini ile kurgulanması.
- [x] **Frontend (Vue3):** FileManager.vue içerisine "Arşive Ekle", "Buraya Çıkart" ve "Çöp Kutusuna Gönder" "Editor" "Sil" "Ekle" arayüz butonlarının ve API entegrasyonlarının eklenmesi.

## 🛡️ Modül 2: Kaynak İzolasyonu (Resource Limits / cgroups)
- [x] **Backend (Rust):** `systemd slices` ve Linux `cgroups v2` kullanarak kullanıcı başına CPU (%), RAM (MB), I/O limitlerinin hesaplanması.
- [x] **Backend (Rust):** Paketlere (Packages) ait kaynak limit ayarlarının veritabanına/konfigürasyona eklenmesi.
- [x] **Backend (Rust):** Yeni VHost/Kullanıcı oluşturulurken ilgili limitlerin (slice) sisteme otomatik tanımlanması (CloudLinux LVE alternatifi).
- [x] **Frontend (Vue3):** Paket oluşturma/düzenleme ekranlarına CPU/RAM limit girişlerinin eklenmesi.

## 📟 Modül 3: Web Terminal (Tarayıcı İçi İzole SSH)
- [x] **Backend (Rust/Go):** WebSocket üzerinden çalışan güvenli bir Terminal PTY (Pseudo-Terminal) sunucusu entegrasyonu.
- [x] **Backend (Güvenlik):** Terminalin root olarak değil, sadece oturum açan kullanıcının yetkileriyle (jail/chroot) başlatılması.
- [x] **Frontend (Vue3):** `Xterm.js` kütüphanesinin projeye dahil edilmesi ve Dashboard/Websites menüsünde "Terminal" penceresinin kodlanması.

## 🚚 Modül 4: Kapsamlı Taşıma (Migration) Aracı
- [x] **Backend:** Yüklenen `cpmove-*.tar.gz` (cPanel) yedeğini analiz edecek ve ayrıştıracak (parse) modülün yazılması.
- [x] **Backend:** cPanel MySQL dump'larının MariaDB'ye, email hesaplarının MailManager'a ve VHost dosyalarının mevcut servis katmanına dönüştürülüp import edilmesi.
- [x] **Backend:** CyberPanel yedek yapısı için benzer dönüştürücü (Converter) fonksiyonlarının eklenmesi.
- [x] **Frontend (Vue3):** "Migration / Taşıma Sihirbazı" arayüzünün oluşturulması (Yükleme çubuğu, log izleme ve durum bildirimleri).

## 📊 Modül 5: Gelişmiş Trafik ve İstatistikler (Analytics)
- [x] **Backend (Rust):** OpenLiteSpeed `access.log` verilerini parse edip JSON formatında istatistik (Hit, Ziyaretçi, Bant Genişliği) dönecek endpoint'in yazılması. (Alternatif: GoAccess entegrasyonu).
- [x] **Frontend (Vue3):** Websites detay ekranına "Trafik & İstatistikler" sekmesinin eklenip grafiklerin (`Chart.js` veya `ApexCharts`) çizdirilmesi.

## 🦠 Modül 6: Zararlı Yazılım Taraması (Malware Scanner)
- [x] **Backend (Rust):** `ClamAV` daemon veya `Yara` kuralları ile belirli bir dizini asenkron (background task) tarayacak yapının kurulması.
- [x] **Backend (Rust):** Tarama sonuçlarını, tespit edilen zararlı dosyaları listeleyen ve "Karantinaya Al" komutu veren API'lerin yazılması.
- [x] **Frontend (Vue3):** Güvenlik (Security) sekmesi altına "Malware Scanner" arayüzünün ve Karantina yöneticisinin eklenmesi.

## 🚀 Modül 7: 1-Tık Uygulama Marketi (App Installer)
- [ ] **Backend (Rust):** PrestaShop, Laravel uygulamaların resmi depolarından indirilip konfigüre edilmesini sağlayacak indirme/kurulum şablonlarının (Recipe) hazırlanması.
- [ ] **Frontend (Vue3):** Mevcut WordPress yöneticisinin yanına genel bir "Uygulama Marketi" arayüzünün yapılması.

## 💳 Modül 8: Faturalandırma (Billing) Panel Entegrasyonu
- [ ] **Backend (Rust/Go):** Ek Olarak Panel ile Tam Entegre bir Faturalandırma yazılımı güvenle panele bağlanabilmesi için izole edilmiş bir API Anahtarı (API Key) yönetim sisteminin oluşturulması.
- [ ] **Harici Modül (PHP):** Faturalandırma Sitesi için "Hesap Aç, Askıya Al (Suspend), Askıdan Kurtar (Unsuspend), Şifre Sıfırla, Hesabı Sil" komutlarını AuraPanel'e iletecek olan Server Provisioning modül kodlarının yazılması.
- [ ] **Frontend (Vue3):** Bayi (Reseller) veya Yönetici paneline Faturalandırma entegrasyonu rehberini ve API Key oluşturma ekranını içeren "Billing Integration" sayfasının eklenmesi.

---
*Not: Bu plan esnektir, geliştirmeler yapıldıkça "[x]" olarak işaretlenecektir.*
