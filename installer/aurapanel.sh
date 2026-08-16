#!/usr/bin/env bash
# ============================================================
# AuraPanel kurulum script'i (ARCHITECTURE §5.3: pinned versions)
# Kullanım:  curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/v<VERSION>/installer/aurapanel.sh | sudo -E bash
#           veya indirip: sudo bash aurapanel.sh [--public|--private] [--no-ols]
#
# İlkeler:
#   - "Latest" YOKTUR: her bileşen versions.json manifestindeki sabit
#     sürümden kurulur (SHA-256 doğrulamalı).
#   - Kurulum sonrası panelin yönettiği paketler apt-mark hold ile kilitlenir.
#   - Erişim modu: --private (varsayılan, SSH tunnel) | --public (IP:port).
# ============================================================
set -euo pipefail

# --- Sabitler (bu satırlar release ile birlikte versiyonlanır) ---
PANEL_VERSION="${AP_PANEL_VERSION:-0.1.0}"
PANEL_SHA256="${AP_PANEL_SHA256:-}"
OLS_VERSION="${AP_OLS_VERSION:-1.9.2-1.noble}"
OLS_SHA256="${AP_OLS_SHA256:-8efb81d547a8f605d341c5ab20da35355cf7c4c742990aae698c00c322399515}"
LSPHP_VERSIONS="${AP_LSPHP_VERSIONS:-8.2 8.3 8.4}"
LSPHP82_VER="${AP_LSPHP82_VER:-8.2.33-1.noble}"
LSPHP83_VER="${AP_LSPHP83_VER:-8.3.33-1.noble}"
LSPHP84_VER="${AP_LSPHP84_VER:-8.4.24-1.noble}"
PHP_MODULES="${AP_PHP_MODULES:-common curl intl mysql opcache redis}"
DOWNLOAD_BASE="${AP_DOWNLOAD_BASE:-https://github.com/mkoyazilim/downloadaurapanel/releases/download/v${PANEL_VERSION}}"
PANEL_BASE="${AP_PANEL_BASE:-https://github.com/mkoyazilim/aurapanel/releases/download/v${PANEL_VERSION}}"
PANEL_PORT=8080
ACCESS_MODE="private"
SKIP_OLS=0

for arg in "$@"; do
  case "$arg" in
    --public) ACCESS_MODE="public" ;;
    --private) ACCESS_MODE="private" ;;
    --no-ols) SKIP_OLS=1 ;;
    --port=*) PANEL_PORT="${arg#*=}" ;;
    *) echo "Bilinmeyen argüman: $arg" >&2; exit 2 ;;
  esac
done

if [[ $EUID -ne 0 ]]; then echo "root olarak çalıştırın (sudo)." >&2; exit 1; fi

if ! grep -Eq "Ubuntu|Debian" /etc/os-release 2>/dev/null; then
  echo "UYARI: Ubuntu veya Debian tabanlı bir işletim sistemi bekleniyor (Önerilen: Ubuntu 24.04 veya Debian 12)." >&2
fi

log() { echo "[aurapanel] $*"; }
fail() { echo "[aurapanel] HATA: $*" >&2; exit 1; }

# --- 1) Temel paketler ---
log "Paketler kuruluyor…"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq curl ca-certificates quota openssh-server nftables fail2ban \
  mariadb-server logrotate cron 2>&1 | tail -2

# --- 2) OpenLiteSpeed (pinned .deb, değiştirilmeden) ---
if [[ $SKIP_OLS -eq 0 ]]; then
  if [[ -z "$OLS_VERSION" ]]; then
    log "UYARI: OLS sürümü manifestte tanımsız (AP_OLS_VERSION) — OLS kurulumu atlanıyor."
  else
    OLS_DEB="openlitespeed_${OLS_VERSION}_amd64.deb"
    log "OpenLiteSpeed ${OLS_VERSION} indiriliyor…"
    curl -fsSL -o /tmp/$OLS_DEB "${DOWNLOAD_BASE}/${OLS_DEB}"
    if [[ -n "$OLS_SHA256" ]]; then
      echo "${OLS_SHA256}  /tmp/$OLS_DEB" | sha256sum -c - >/dev/null || fail "OLS SHA-256 doğrulaması BAŞARISIZ"
    fi
    apt-get install -y -qq /tmp/$OLS_DEB 2>&1 | tail -2 || apt-get install -y -qq -f
    rm -f /tmp/$OLS_DEB

    # LSPHP sürümleri ve temel modüller (WordPress vb. için zorunlu)
    for v in $LSPHP_VERSIONS; do
      dig=${v//./}
      if [ "$dig" = "82" ]; then VER=$LSPHP82_VER; fi
      if [ "$dig" = "83" ]; then VER=$LSPHP83_VER; fi
      if [ "$dig" = "84" ]; then VER=$LSPHP84_VER; fi
      
      log "LSPHP ${v} ve modülleri indiriliyor…"
      CORE_DEB="lsphp${dig}_${VER}_amd64.deb"
      curl -fsSL -o /tmp/$CORE_DEB "${DOWNLOAD_BASE}/${CORE_DEB}" || log "UYARI: $CORE_DEB atlanıyor"
      
      for mod in $PHP_MODULES; do
        if [ "$mod" = "common" ]; then ARCH="all"; else ARCH="amd64"; fi
        if [ "$mod" = "redis" ]; then MOD_VER="6.3.0-1.noble"; else MOD_VER=$VER; fi
        MOD_DEB="lsphp${dig}-${mod}_${MOD_VER}_${ARCH}.deb"
        curl -fsSL -o /tmp/$MOD_DEB "${DOWNLOAD_BASE}/${MOD_DEB}" || true
      done
      
      log "LSPHP ${v} kuruluyor…"
      apt-get install -y -qq /tmp/lsphp${dig}*.deb 2>&1 | tail -1 || apt-get install -y -qq -f
      rm -f /tmp/lsphp${dig}*.deb
    done
  fi
fi

# --- 3) Sistem hazırlığı: kullanıcı, dizinler, anahtar ---
log "Kullanıcı ve dizinler…"
id -u aurapanel >/dev/null 2>&1 || useradd --system --home /var/lib/aurapanel --shell /usr/sbin/nologin aurapanel
install -d -o aurapanel -g aurapanel -m 750 /var/lib/aurapanel /var/lib/aurapanel/keys /var/lib/aurapanel/state /var/lib/aurapanel/state/certs /var/lib/aurapanel/uploads /var/lib/aurapanel/trash
install -d -o aurapanel -g aurapanel -m 755 /srv/aurapanel /srv/aurapanel/sites /srv/aurapanel/backups
install -d -m 750 /run/aurapanel && chown root:aurapanel /run/aurapanel
install -d -m 750 /var/log/aurapanel && chown root:aurapanel /var/log/aurapanel
# /run tmpfs'tir: reboot'ta silinir. tmpfiles.d, priv socket dizinini her açılışta
# yeniden oluşturur (priv.sock'un kendisi systemd socket unit'ine aittir).
cat > /usr/lib/tmpfiles.d/aurapanel.conf <<'EOF'
d /run/aurapanel 0750 root aurapanel -
EOF
systemd-tmpfiles --create /usr/lib/tmpfiles.d/aurapanel.conf >/dev/null 2>&1 || true
if [[ ! -f /var/lib/aurapanel/keys/master.key ]]; then
  head -c 32 /dev/urandom > /var/lib/aurapanel/keys/master.key
  chown aurapanel:aurapanel /var/lib/aurapanel/keys/master.key
  chmod 600 /var/lib/aurapanel/keys/master.key
fi

# --- 4) cgroup v2 delegasyonu ---
log "cgroup v2 delegasyonu…"
mkdir -p /sys/fs/cgroup/aurapanel
chown -R aurapanel:aurapanel /sys/fs/cgroup/aurapanel 2>/dev/null || log "UYARI: cgroup sahiplik devri başarısız (kernel sınırı olabilir)"

# --- 5) ext4 user quota etkinleştirme ---
log "Disk kotası (ext4 user quota)…"
ROOTDEV=$(findmnt -no SOURCE /)
if mount | grep -q " on / "; then
  mount -o remount,usrquota / 2>/dev/null || log "UYARI: usrquota remount başarısız (bulut kısıtı olabilir)"
fi
if [[ -x /usr/sbin/quotacheck ]]; then
  quotacheck -cum / 2>/dev/null || true
  quotaon -u / 2>/dev/null || log "UYARI: quotaon başarısız — kota kurulumu kontrol edilmeli"
fi

# --- 6) Panel binary (pinned + SHA-256) ---
log "AuraPanel ${PANEL_VERSION} kuruluyor…"
BIN="/tmp/aurapanel-${PANEL_VERSION}"
curl -fsSL -o "$BIN" "${PANEL_BASE}/aurapanel_${PANEL_VERSION}_linux_amd64"
if [[ -n "$PANEL_SHA256" ]]; then
  echo "${PANEL_SHA256}  $BIN" | sha256sum -c - >/dev/null || fail "Panel SHA-256 doğrulaması BAŞARISIZ"
fi
chmod 755 "$BIN"
install -m 755 "$BIN" /usr/local/sbin/aurapanel
ln -sf /usr/local/sbin/aurapanel /usr/local/sbin/aurapanel-priv
rm -f "$BIN"

# --- 7) Yapılandırma ---
log "Yapılandırma yazılıyor…"
mkdir -p /etc/aurapanel
if [[ "$ACCESS_MODE" == "public" ]]; then
  LISTEN_ADDR="0.0.0.0:${PANEL_PORT}"
else
  LISTEN_ADDR="127.0.0.1:${PANEL_PORT}"
fi
cat > /etc/aurapanel/aurapanel.yaml <<EOF
listen:
  address: "${LISTEN_ADDR}"
  mode: ${ACCESS_MODE}
database:
  path: /var/lib/aurapanel/aurapanel.db
mariadb:
  socket: /var/run/mysqld/mysqld.sock
  user: root
  password: ""
log:
  level: info
  format: json
paths:
  data_dir: /var/lib/aurapanel
  sites_root: /srv/aurapanel/sites
  backup_dir: /srv/aurapanel/backups
  trash_dir: /var/lib/aurapanel/trash
EOF
chmod 640 /etc/aurapanel/aurapanel.yaml
chown root:aurapanel /etc/aurapanel/aurapanel.yaml

cat > /etc/systemd/system/aurapanel.service <<'EOF'
[Unit]
Description=AuraPanel (OpenLiteSpeed control panel)
After=network.target mariadb.service aurapanel-priv.socket
Wants=aurapanel-priv.socket

[Service]
User=aurapanel
Group=aurapanel
ExecStart=/usr/local/sbin/aurapanel -config /etc/aurapanel/aurapanel.yaml
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/aurapanel-priv.socket <<'EOF'
[Unit]
Description=AuraPanel privileged helper socket

[Socket]
ListenStream=/run/aurapanel/priv.sock
SocketUser=root
SocketGroup=aurapanel
SocketMode=0660

[Install]
WantedBy=sockets.target
EOF

cat > /etc/systemd/system/aurapanel-priv.service <<'EOF'
[Unit]
Description=AuraPanel privileged helper
[Service]
Type=simple
ExecStart=/usr/local/sbin/aurapanel-priv
StandardInput=socket
StandardOutput=journal
Restart=on-failure
RestartSec=2
EOF

systemctl daemon-reload
systemctl enable --now aurapanel-priv.socket >/dev/null 2>&1 || true
systemctl enable aurapanel.service >/dev/null 2>&1 || true

# --- 8.5) Log rotation (priv.log append-only: copytruncate) ---
log "Log rotation…"
cat > /etc/logrotate.d/aurapanel <<'EOF'
/var/log/aurapanel/*.log {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
}
EOF

# --- 9) nftables default-deny ---
log "Firewall (nftables default-deny)…"
if command -v nft >/dev/null; then
  if [[ "$ACCESS_MODE" == "public" ]]; then
    PANEL_RULE="tcp dport ${PANEL_PORT} accept"
  else
    PANEL_RULE=""
  fi
  nft -f - <<EOF
table inet aurapanel {
  chain input {
    type filter hook input priority 0; policy drop;
    ct state established,related accept
    iif lo accept
    tcp dport 22 accept
    tcp dport 80 accept
    tcp dport 443 accept
    ip protocol icmp accept
    ip6 nexthdr icmpv6 accept
    ${PANEL_RULE}
  }
}
EOF
  # Kalıcılık: reboot'ta nftables.service ruleset'i /etc/nftables.conf'tan yükler.
  nft list ruleset > /etc/nftables.conf
  systemctl enable --now nftables >/dev/null 2>&1 || true
fi

# --- 10) Paket kilitleme (kontrollü güncelleme politikası) ---
log "Panelin yönettiği paketler kilitleniyor (apt-mark hold)…"
apt-mark hold mariadb-server 2>/dev/null || true
dpkg -l | awk '/openlitespeed|lsphp/ {print $2}' | xargs -r apt-mark hold 2>/dev/null || true

systemctl enable --now mariadb >/dev/null 2>&1 || true

# --- 10.5) OLS VirtualHost Include Hook ---
if [[ -f /usr/local/lsws/conf/httpd_config.conf ]]; then
  if ! grep -q "include /usr/local/lsws/conf/vhosts/\*/main.conf" /usr/local/lsws/conf/httpd_config.conf; then
    echo "include /usr/local/lsws/conf/vhosts/*/main.conf" >> /usr/local/lsws/conf/httpd_config.conf
  fi
fi

systemctl enable --now lsws >/dev/null 2>&1 || true
systemctl start aurapanel.service 2>/dev/null || true

sleep 2 # Logların yazılması için kısa bir bekleme süresi

AP_USERNAME=$(journalctl -u aurapanel -n 50 | grep -i "Kullanıcı Adı" | awk '{print $NF}' | tail -1)
AP_PASSWORD=$(journalctl -u aurapanel -n 50 | grep -i "Şifre" | awk '{print $NF}' | tail -1)

if [[ -z "$AP_USERNAME" ]]; then
  AP_USERNAME="admin"
  AP_PASSWORD="[Şifre loglarda üretiliyor: journalctl -u aurapanel]"
fi

# --- 11) Özet ---
PUBLIC_IP=$(curl -fsSL --max-time 5 https://api.ipify.org 2>/dev/null || echo "<SUNUCU-IP>")
echo
echo "=================================================="
echo " AuraPanel kuruldu (v${PANEL_VERSION})"
if [[ "$ACCESS_MODE" == "public" ]]; then
  echo " Panel:    http://${PUBLIC_IP}:${PANEL_PORT}"
  echo " UYARI: public mod — kurulum TLS'i etkinleştirmedi;"
  echo "        ayarlardan güçlendirmeleri (2FA/reCAPTCHA) açın."
else
  echo " Panel:    yalnızca sunucuda (127.0.0.1:${PANEL_PORT})"
  echo " Bağlantı: bilgisayarınızdan SSH tunnel:"
  echo "   ssh -L ${PANEL_PORT}:127.0.0.1:${PANEL_PORT} root@${PUBLIC_IP}"
fi
echo " --- İLK GİRİŞ BİLGİLERİ (TEK KULLANIMLIK) ---"
echo " Kullanıcı Adı: ${AP_USERNAME}"
echo " Şifre:         ${AP_PASSWORD}"
echo " "
echo " İLK GİRİŞTE ŞİFRE DEĞİŞTİRMEK ZORUNLUDUR."
echo "=================================================="
