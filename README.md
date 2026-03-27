# 🌌 AuraPanel

> AI-Powered, Open Source Hosting Control Panel — Built with Rust, Go & Vue.js

![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Rust](https://img.shields.io/badge/core-Rust-orange)
![Go](https://img.shields.io/badge/gateway-Go-00ADD8)
![Vue.js](https://img.shields.io/badge/frontend-Vue.js%203-4FC08D)

---

## 🚀 Nedir?

AuraPanel, sunucu yönetimini kolaylaştırmak için sıfırdan tasarlanmış, **yapay zeka destekli**, modüler ve açık kaynaklı bir **Hosting Kontrol Paneli**dir.

cPanel/Plesk gibi kapalı kaynak panellere alternatif olarak, tamamen özgür yazılım bileşenleri üzerine kurulmuştur.

## ✨ Özellikler

| Modül | Teknoloji | Durum |
|---|---|---|
| 🌐 Web Sunucusu | OpenLiteSpeed | ✅ |
| 🔒 SSL/TLS | Let's Encrypt (ACME) | ✅ |
| 📧 E-Posta | Stalwart + Roundcube | ✅ |
| 🗄️ Veritabanı | MariaDB + AuraDB Explorer | ✅ |
| 🌍 DNS | PowerDNS (REST API) | ✅ |
| 🛡️ Güvenlik | eBPF + nftables + ML-WAF | ✅ |
| ⚡ Performans | Redis/Valkey + LSCache | ✅ |
| 🤖 AI-SRE | Prometheus + Tahminleme | ✅ |
| 📦 1-Click Installer | WordPress, Laravel | ✅ |
| 🔄 GitOps | Push-to-Deploy (Webhook) | ✅ |
| 💾 Yedekleme | Restic + MinIO (S3) | ✅ |
| 🔗 Federasyon | WireGuard Mesh VPN | ✅ |
| 👥 Reseller | Çoklu kullanıcı & Paket yönetimi | ✅ |
| 🌐 Çoklu Dil | TR, EN, DE, FR, ES | ✅ |

## 🏗️ Mimari

```
┌──────────────────────────────────────────┐
│            Vue.js 3 Frontend             │
│         (Vite + Tailwind Dark UI)        │
└──────────────┬───────────────────────────┘
               │ HTTPS / JWT
┌──────────────▼───────────────────────────┐
│          Go API Gateway (:8090)          │
│     (Auth, CORS, Rate Limit, Proxy)      │
└──────────────┬───────────────────────────┘
               │ HTTP / Internal
┌──────────────▼───────────────────────────┐
│   Rust Micro-Core (127.0.0.1:8000)       │
│  ┌─────────┬──────────┬────────────┐     │
│  │ Nitro   │ PowerDNS │ AuraDB     │     │
│  │ Engine  │ Manager  │ Explorer   │     │
│  ├─────────┼──────────┼────────────┤     │
│  │ SSL/TLS │ Mail     │ Security   │     │
│  │ ACME    │ Manager  │ eBPF+WAF   │     │
│  ├─────────┼──────────┼────────────┤     │
│  │ Perf    │ Monitor  │ Federated  │     │
│  │ Redis   │ AI-SRE   │ WireGuard  │     │
│  └─────────┴──────────┴────────────┘     │
└──────────────────────────────────────────┘
```

## 📦 Hızlı Kurulum

### Gereksinimler
- Ubuntu 22.04+ / Debian 12+ (veya RHEL 9+)
- Minimum 1 vCPU, 1 GB RAM
- Root erişimi

### Tek Satırda Kurulum

```bash
curl -sSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
```

### Manuel Derleme

```bash
git clone https://github.com/mkoyazilim/aurapanel.git
cd aurapanel
make build
make install
```

## 🛠️ Geliştirme

### Ön Koşullar
- **Rust** 1.75+
- **Go** 1.21+  
- **Node.js** 18+

### Çalıştırma

```bash
# Rust Core
cd core && cargo run

# Go API Gateway
cd api-gateway && go run main.go

# Vue.js Frontend
cd frontend && npm install && npm run dev
```

## 📁 Proje Yapısı

```
aurapanel/
├── core/               # Rust Micro-Core (Sistem Programlama)
│   └── src/
│       ├── api/        # Axum REST API Endpoints
│       ├── auth/       # JWT + TOTP
│       ├── config/     # TOML Yapılandırma
│       └── services/   # Nitro, DNS, DB, Mail, SSL, Security...
├── api-gateway/        # Go HTTP Gateway (Gin)
│   ├── controllers/    # Auth, Websites
│   ├── middleware/      # CORS, Logger, Auth Guard
│   └── main.go
├── frontend/           # Vue.js 3 + Vite + Tailwind
│   └── src/
│       ├── views/      # Dashboard, Websites, DNS, Mail...
│       ├── layouts/    # DashboardLayout (Sidebar)
│       ├── stores/     # Pinia (Auth State)
│       └── services/   # Axios API Client
├── install.sh          # Tek satır kurulum scripti
└── Makefile            # Derleme & Paketleme
```

## 🤝 Katkıda Bulunma

1. Fork edin
2. Feature branch oluşturun (`git checkout -b feature/amazing-feature`)
3. Commit atın (`git commit -m 'feat: Add amazing feature'`)
4. Push edin (`git push origin feature/amazing-feature`)
5. Pull Request açın

## 📄 Lisans

Bu proje **MIT** lisansı altında dağıtılmaktadır. Detaylar için [LICENSE](LICENSE) dosyasına bakınız.

## 👨‍💻 Geliştirici

**MKO Yazılım** — [mkoyazilim](https://github.com/mkoyazilim)
**Tahamada**
---

<p align="center">
  <b>AuraPanel</b> — Sunucu yönetiminin geleceği. 🌌
</p>

## Proje İlerleme Grafigi

**Genel İlerleme:** `78%`

```
[###############################---------] 78%
```

| Alan | Tamamlanma |
|---|---|
| Core + API + Frontend Lifecycle | 85% |
| Security + Status + SSL + FTP | 82% |
| Mail Stack Production Hardening | 74% |
| Installer + Distribution Hardening | 76% |

## Yapimcilar

- **Mkoyazilim** — [mkoyazilim](https://github.com/mkoyazilim)
- **Tahamada**
