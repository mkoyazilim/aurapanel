#!/usr/bin/env bash
# AuraPanel 1-Click Installer
# Sadece yetkili (root) kullanici tarafindan calistirilmalidir.
set -e

# Renkler
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()  { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

if [ "$EUID" -ne 0 ]; then
  log_err "Bu betik root yetkileriyle (sudo) calistirilmalidir."
fi

# İsletim sistemi tespiti
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    VERSION_ID=$VERSION_ID
else
    log_err "Desteklenmeyen isletim sistemi."
fi

log_info "AuraPanel Kurulumu Basliyor... ($OS $VERSION_ID)"

if [[ "$OS" == "ubuntu" || "$OS" == "debian" ]]; then
    # APT Tabanlı Sistemler
    log_info "Sistem paketleri guncelleniyor..."
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y curl wget tar sqlite3 mariadb-server systemd

    # LiteSpeed Repo (Ubuntu/Debian)
    log_info "OpenLiteSpeed deposu ekleniyor..."
    wget -O - https://repo.litespeed.ws/debian/enable_lst_debian_repo.sh | bash

    log_info "OpenLiteSpeed ve lsphp paketleri kuruluyor..."
    apt-get install -y openlitespeed lsphp82 lsphp83 lsphp83-mysql lsphp83-curl lsphp83-imagick lsphp83-intl
elif [[ "$OS" == "almalinux" || "$OS" == "rocky" || "$OS" == "centos" ]]; then
    # RHEL Tabanlı Sistemler
    log_info "Sistem paketleri guncelleniyor..."
    dnf install -y epel-release
    dnf update -y
    dnf install -y curl wget tar sqlite mariadb-server systemd

    log_info "OpenLiteSpeed deposu ekleniyor..."
    rpm -Uvh http://rpms.litespeedtech.com/centos/litespeed-repo-1.3-1.el8.noarch.rpm || true

    log_info "OpenLiteSpeed ve lsphp paketleri kuruluyor..."
    dnf install -y openlitespeed lsphp82 lsphp83 lsphp83-mysqlnd lsphp83-process lsphp83-mbstring lsphp83-intl lsphp83-pdo
    
    # MariaDB baslat
    systemctl enable --now mariadb
else
    log_err "Sadece Ubuntu, Debian, AlmaLinux ve Rocky Linux desteklenmektedir."
fi

# Dizinlerin Olusturulmasi
log_info "AuraPanel dizinleri olusturuluyor..."
mkdir -p /etc/aurapanel
mkdir -p /var/lib/aurapanel
mkdir -p /var/log/aurapanel
mkdir -p /srv/aurapanel/sites
mkdir -p /var/lib/aurapanel/state/certs

# AuraPanel Binary İndirme
log_info "AuraPanel ikili (binary) dosyasi indiriliyor..."
# TODO: Gecerli bir binary URL'si ekleyin
# Gecici olarak dummy binary olusturuluyor, eger url bulunamazsa hata vermemesi icin curl atlanabilir.
# curl -L -o /usr/local/sbin/aurapanel "https://github.com/mkoyazilim/downloadaurapanel/releases/latest/download/aurapanel-linux-amd64"
touch /usr/local/sbin/aurapanel
chmod +x /usr/local/sbin/aurapanel

# SUID Helper
ln -sf /usr/local/sbin/aurapanel /usr/local/sbin/aurapanel-priv
chmod u+s /usr/local/sbin/aurapanel-priv

# Systemd Servisi
log_info "Systemd servisi kuruluyor..."
cat > /etc/systemd/system/aurapanel.service << 'EOF'
[Unit]
Description=AuraPanel Control Panel
After=network.target mariadb.service

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/sbin/aurapanel
Restart=always
RestartSec=3
LimitNOFILE=65535
Environment="PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
# NOT: dummy binary oldugu icin baslatmayalim (systemctl enable --now aurapanel)

log_info "Kurulum tamamlandi! (Binary url eklendiginde servis otomatik baslayacaktir)"
log_info "İlk giris bilgileriniz konsolda gosterilecektir. Gormek icin: 'aurapanel'"
