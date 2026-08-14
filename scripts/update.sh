#!/usr/bin/env bash
# AuraPanel Güncelleme Scripti
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_err()  { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

if [ "$EUID" -ne 0 ]; then
  log_err "Bu betik root yetkileriyle calistirilmalidir."
fi

# NOT: Sürüm bilgisi vs parametre veya release API'den alınabilir
# Şimdilik doğrudan update binary'si indirme simülasyonu yapıyoruz.

log_info "AuraPanel guncelleniyor..."
systemctl stop aurapanel || true

# TODO: Gerçek URL'den indirin.
# curl -L -o /usr/local/sbin/aurapanel "https://github.com/mkoyazilim/downloadaurapanel/releases/latest/download/aurapanel-linux-amd64"
touch /usr/local/sbin/aurapanel
chmod +x /usr/local/sbin/aurapanel

# SUID onarımı
ln -sf /usr/local/sbin/aurapanel /usr/local/sbin/aurapanel-priv
chmod u+s /usr/local/sbin/aurapanel-priv

# Veritabanı şema göçleri (Migrations)
log_info "Veritabani sema guncellemeleri (migrations) denetleniyor..."
/usr/local/sbin/aurapanel -check || log_err "Sema kontrolu sirasinda bir hata olustu."

# Servisi başlat
systemctl start aurapanel
systemctl enable aurapanel

log_info "Guncelleme basariyla tamamlandi!"
