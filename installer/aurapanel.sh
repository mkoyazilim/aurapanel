#!/usr/bin/env bash
# AuraPanel Production Installation Script
# Supported OS: Ubuntu 22.04/24.04, Debian 12+, AlmaLinux 8/9, Rocky Linux 8/9
# Usage: curl -sSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash

set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

PROJECT_DIR="/opt/aurapanel"
GATEWAY_ENV_DIR="/etc/aurapanel"
GATEWAY_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel.env"
CORE_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-core.env"
OLS_ADMIN_STATE_FILE="${GATEWAY_ENV_DIR}/aurapanel-ols-admin.env"
MINIO_ENV_FILE="/etc/default/minio"
CREDENTIALS_SUMMARY_FILE="/root/aurapanel_credentials.txt"
REPO_URL="https://github.com/mkoyazilim/aurapanel.git"
AURAPANEL_GH_OWNER="${AURAPANEL_GH_OWNER:-mkoyazilim}"
AURAPANEL_GH_REPO="${AURAPANEL_GH_REPO:-aurapanel}"
AURAPANEL_GH_REF="${AURAPANEL_GH_REF:-main}"
RAW_BASE_URL="https://raw.githubusercontent.com/${AURAPANEL_GH_OWNER}/${AURAPANEL_GH_REPO}/${AURAPANEL_GH_REF}"
RELEASE_BASE_URL="${AURAPANEL_RELEASE_BASE_URL:-https://github.com/${AURAPANEL_GH_OWNER}/${AURAPANEL_GH_REPO}/releases/latest/download}"

SOURCE_ARCHIVE_URL="${AURAPANEL_SOURCE_ARCHIVE_URL:-${RELEASE_BASE_URL}/aurapanel-source-latest.tar.gz}"
SOURCE_ARCHIVE_SHA256_URL="${AURAPANEL_SOURCE_ARCHIVE_SHA256_URL:-${SOURCE_ARCHIVE_URL}.sha256}"
AURAPANEL_ALLOW_GIT_FALLBACK="${AURAPANEL_ALLOW_GIT_FALLBACK:-1}"

NODE_SETUP_URL="${AURAPANEL_NODE_SETUP_URL:-https://deb.nodesource.com/setup_20.x}"
RUSTUP_INIT_SCRIPT_URL="${AURAPANEL_RUSTUP_INIT_SCRIPT_URL:-https://sh.rustup.rs}"
GO_VERSION="${AURAPANEL_GO_VERSION:-1.22.1}"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_TARBALL_URL="${AURAPANEL_GO_TARBALL_URL:-https://go.dev/dl/${GO_TARBALL}}"
LITESPEED_REPO_SCRIPT_URL="${AURAPANEL_LITESPEED_REPO_SCRIPT_URL:-https://repo.litespeed.sh}"
MINIO_BIN_URL="${AURAPANEL_MINIO_BIN_URL:-https://dl.min.io/server/minio/release/linux-amd64/minio}"
MINIO_MC_URL="${AURAPANEL_MINIO_MC_URL:-https://dl.min.io/client/mc/release/linux-amd64/mc}"
ROUNDCUBE_VERSION="${AURAPANEL_ROUNDCUBE_VERSION:-1.6.11}"
ROUNDCUBE_ARCHIVE_URL="${AURAPANEL_ROUNDCUBE_ARCHIVE_URL:-https://github.com/roundcube/roundcubemail/releases/download/${ROUNDCUBE_VERSION}/roundcubemail-${ROUNDCUBE_VERSION}-complete.tar.gz}"
PANEL_PORT_DEFAULT="8090"
ONE_TIME_PASSWORD_NOTE="NOTE: Passwords are generated only once. Please save them now or change them immediately."

log() {
  echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $*"
}

ok() {
  echo -e "${GREEN}$*${NC}"
}

warn() {
  echo -e "${YELLOW}$*${NC}"
}

fail() {
  echo -e "${RED}$*${NC}"
  exit 1
}

if [ "${EUID}" -ne 0 ]; then
  fail "Please run as root."
fi

if [ -f /etc/os-release ]; then
  . /etc/os-release
  OS_ID="${ID}"
else
  fail "Unsupported OS: /etc/os-release not found."
fi

PKG_MGR=""
case "${OS_ID}" in
  ubuntu|debian)
    PKG_MGR="apt"
    ;;
  almalinux|rocky|centos)
    PKG_MGR="dnf"
    ;;
  *)
    fail "Unsupported OS: ${OS_ID}."
    ;;
esac

install_packages() {
  if [ "$#" -eq 0 ]; then
    return
  fi

  if [ "${PKG_MGR}" = "apt" ]; then
    DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
  else
    dnf install -y "$@"
  fi
}

install_optional_packages() {
  for pkg in "$@"; do
    if ! install_packages "${pkg}"; then
      warn "Optional package '${pkg}' could not be installed."
    fi
  done
}

download_file() {
  local url="$1"
  local output="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output" || return 1
    return 0
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$output" || return 1
    return 0
  fi

  return 1
}

upsert_env() {
  local file="$1"
  local key="$2"
  local value="$3"

  mkdir -p "$(dirname "${file}")"
  touch "${file}"

  if grep -qE "^${key}=" "${file}"; then
    sed -i "s|^${key}=.*|${key}=${value}|" "${file}"
  else
    printf '%s=%s\n' "${key}" "${value}" >> "${file}"
  fi
}

read_env_value() {
  local file="$1"
  local key="$2"

  if [ ! -f "${file}" ]; then
    return 1
  fi

  grep -E "^${key}=" "${file}" | tail -n1 | cut -d'=' -f2- || true
}

gateway_port() {
  local addr port
  addr="$(grep -E '^AURAPANEL_GATEWAY_ADDR=' "${GATEWAY_ENV_FILE}" 2>/dev/null | tail -n1 | cut -d'=' -f2- || true)"
  addr="${addr:-:${PANEL_PORT_DEFAULT}}"
  port="${addr##*:}"

  if [[ ! "${port}" =~ ^[0-9]+$ ]] || [ "${port}" -le 0 ] || [ "${port}" -gt 65535 ]; then
    echo "${PANEL_PORT_DEFAULT}"
    return
  fi

  echo "${port}"
}

configure_panel_firewall() {
  # Backward-compatible wrapper. Main flow calls configure_standard_firewall.
  configure_standard_firewall "$1"
}

configure_ftp_firewall() {
  # Backward-compatible no-op wrapper. Standard profile covers FTP ports.
  return 0
}

to_firewalld_rule() {
  local ufw_rule="$1"
  local port_part proto
  port_part="${ufw_rule%/*}"
  proto="${ufw_rule##*/}"
  # firewalld range syntax uses hyphen, ufw uses colon.
  port_part="${port_part/:/-}"
  echo "${port_part}/${proto}"
}

configure_standard_firewall() {
  local panel_port="$1"
  local touched="0"
  local ufw_active="0"
  local firewalld_active="0"
  local firewalld_changed="0"

  if command -v ufw >/dev/null 2>&1; then
    if ufw status 2>/dev/null | grep -qi "Status: active"; then
      ufw_active="1"
    else
      warn "ufw is installed but inactive. Skipping ufw rule automation."
    fi
  fi

  if command -v firewall-cmd >/dev/null 2>&1; then
    if firewall-cmd --state >/dev/null 2>&1; then
      firewalld_active="1"
    else
      warn "firewalld is installed but inactive. Skipping firewalld rule automation."
    fi
  fi

  if [ "${ufw_active}" = "0" ] && [ "${firewalld_active}" = "0" ]; then
    warn "No active firewall manager detected for automated port opening."
    return 0
  fi

  # Standard AuraPanel exposure profile (internet-facing services).
  local -a entries=(
    "22/tcp|SSH"
    "80/tcp|HTTP (ACME challenge)"
    "443/tcp|HTTPS"
    "7080/tcp|OpenLiteSpeed WebAdmin"
    "${panel_port}/tcp|AuraPanel Gateway Panel"
    "53/tcp|DNS (TCP)"
    "53/udp|DNS (UDP)"
    "25/tcp|SMTP"
    "465/tcp|SMTPS"
    "587/tcp|SMTP Submission"
    "110/tcp|POP3"
    "995/tcp|POP3S"
    "143/tcp|IMAP"
    "993/tcp|IMAPS"
    "21/tcp|FTP"
    "30000:30049/tcp|FTP Passive Range"
  )

  declare -A seen_rules=()
  local entry rule label firewalld_rule
  for entry in "${entries[@]}"; do
    rule="${entry%%|*}"
    label="${entry#*|}"

    if [ -n "${seen_rules[${rule}]:-}" ]; then
      continue
    fi
    seen_rules["${rule}"]=1

    if [ "${ufw_active}" = "1" ]; then
      if ufw allow "${rule}" >/dev/null 2>&1; then
        ok "ufw rule ensured: ${rule} (${label})"
        touched="1"
      else
        warn "ufw is active but failed to allow ${rule} (${label})."
      fi
    fi

    if [ "${firewalld_active}" = "1" ]; then
      firewalld_rule="$(to_firewalld_rule "${rule}")"
      if firewall-cmd --permanent --add-port="${firewalld_rule}" >/dev/null 2>&1; then
        ok "firewalld rule ensured: ${firewalld_rule} (${label})"
        touched="1"
        firewalld_changed="1"
      else
        warn "firewalld is active but failed to add ${firewalld_rule} (${label})."
      fi
    fi
  done

  if [ "${firewalld_active}" = "1" ] && [ "${firewalld_changed}" = "1" ]; then
    firewall-cmd --reload >/dev/null 2>&1 || true
  fi

  if [ "${touched}" = "0" ]; then
    warn "Firewall manager is active but no rule could be applied."
  fi
}

configure_pureftpd() {
  if ! command -v pure-pw >/dev/null 2>&1; then
    warn "pure-pw binary is missing. PureFTPd may not be installed on this distro."
    return
  fi

  log "Configuring PureFTPd defaults..."
  mkdir -p /etc/pure-ftpd/conf /etc/pure-ftpd /etc/ssl/private

  if [ ! -f /etc/ssl/private/pure-ftpd.pem ]; then
    openssl req -x509 -nodes -newkey rsa:2048 \
      -keyout /etc/ssl/private/pure-ftpd.pem \
      -out /etc/ssl/private/pure-ftpd.pem \
      -days 3650 \
      -subj "/CN=$(hostname -f 2>/dev/null || hostname)" >/dev/null 2>&1 || true
  fi
  chmod 600 /etc/ssl/private/pure-ftpd.pem >/dev/null 2>&1 || true

  echo "2" > /etc/pure-ftpd/conf/TLS
  echo "30000 30049" > /etc/pure-ftpd/conf/PassivePortRange
  echo "yes" > /etc/pure-ftpd/conf/ChrootEveryone
  echo "yes" > /etc/pure-ftpd/conf/NoAnonymous
  echo "yes" > /etc/pure-ftpd/conf/UnixAuthentication
  echo "no" > /etc/pure-ftpd/conf/PAMAuthentication
  echo "/etc/pure-ftpd/pureftpd.pdb" > /etc/pure-ftpd/conf/PureDB

  touch /etc/pure-ftpd/pureftpd.passwd
  pure-pw mkdb /etc/pure-ftpd/pureftpd.pdb -f /etc/pure-ftpd/pureftpd.passwd >/dev/null 2>&1 || true

  if systemctl list-unit-files | grep -q '^pure-ftpd\.service'; then
    systemctl enable pure-ftpd >/dev/null 2>&1 || true
    systemctl restart pure-ftpd >/dev/null 2>&1 || true
    ok "PureFTPd service enabled and restarted."
  else
    warn "pure-ftpd systemd service not found. Check distro package naming."
  fi
}

configure_roundcube() {
  if [ "${AURAPANEL_INSTALL_ROUNDCUBE:-1}" != "1" ]; then
    warn "Roundcube install skipped (AURAPANEL_INSTALL_ROUNDCUBE!=1)."
    return
  fi

  local webmail_dir="/usr/local/lsws/Example/html/webmail"
  local tmpdir archive extracted
  tmpdir="$(mktemp -d /tmp/aurapanel-roundcube.XXXXXX)"
  archive="${tmpdir}/roundcube.tar.gz"
  extracted="${tmpdir}/roundcube"
  mkdir -p "${extracted}" "${webmail_dir}"

  log "Installing Roundcube ${ROUNDCUBE_VERSION}..."
  if download_file "${ROUNDCUBE_ARCHIVE_URL}" "${archive}"; then
    tar -xzf "${archive}" -C "${extracted}" --strip-components=1 || true
    rsync -a --delete "${extracted}/" "${webmail_dir}/"
  else
    warn "Roundcube archive download failed. Falling back to git clone."
    if [ -d "${webmail_dir}/.git" ]; then
      git -C "${webmail_dir}" pull --ff-only >/dev/null 2>&1 || true
    else
      rm -rf "${webmail_dir}"
      git clone --depth 1 https://github.com/roundcube/roundcubemail.git "${webmail_dir}" >/dev/null 2>&1 || true
    fi
  fi

  local rc_db_name rc_db_user rc_db_pass
  rc_db_name="${AURAPANEL_ROUNDCUBE_DB_NAME:-roundcube}"
  rc_db_user="${AURAPANEL_ROUNDCUBE_DB_USER:-roundcube}"
  rc_db_pass="${AURAPANEL_ROUNDCUBE_DB_PASS:-$(openssl rand -base64 18 | tr -d '\n')}"

  if command -v mysql >/dev/null 2>&1 && systemctl is-active --quiet mariadb; then
    mysql <<EOF >/dev/null 2>&1 || true
CREATE DATABASE IF NOT EXISTS \`${rc_db_name}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
CREATE USER IF NOT EXISTS '${rc_db_user}'@'localhost' IDENTIFIED BY '${rc_db_pass}';
ALTER USER '${rc_db_user}'@'localhost' IDENTIFIED BY '${rc_db_pass}';
GRANT ALL PRIVILEGES ON \`${rc_db_name}\`.* TO '${rc_db_user}'@'localhost';
FLUSH PRIVILEGES;
EOF

    if [ -f "${webmail_dir}/SQL/mysql.initial.sql" ]; then
      mysql "${rc_db_name}" < "${webmail_dir}/SQL/mysql.initial.sql" >/dev/null 2>&1 || true
    fi
  else
    warn "MariaDB is not active, Roundcube DB bootstrap skipped."
  fi

  mkdir -p "${webmail_dir}/config"
  cat <<EOF > "${webmail_dir}/config/config.inc.php"
<?php
\$config['db_dsnw'] = 'mysql://${rc_db_user}:${rc_db_pass}@localhost/${rc_db_name}';
\$config['default_host'] = 'ssl://127.0.0.1';
\$config['default_port'] = 993;
\$config['smtp_server'] = 'tls://127.0.0.1';
\$config['smtp_port'] = 587;
\$config['product_name'] = 'AuraPanel Webmail';
\$config['des_key'] = '$(openssl rand -hex 16 | tr -d '\n')';
\$config['plugins'] = ['archive', 'zipdownload', 'markasjunk'];
EOF

  chown -R nobody:nobody "${webmail_dir}" >/dev/null 2>&1 || true
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_WEBMAIL_BASE_URL" "http://127.0.0.1/webmail"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_MAIL_BACKEND" "vmail"

  rm -rf "${tmpdir}"
  ok "Roundcube setup completed at ${webmail_dir}"
}

configure_mail_stack_vmail() {
  local vmail_uid vmail_gid vmail_base
  vmail_uid="${AURAPANEL_MAIL_VMAIL_UID:-5000}"
  vmail_gid="${AURAPANEL_MAIL_VMAIL_GID:-5000}"
  vmail_base="${AURAPANEL_MAIL_VMAIL_BASE:-/var/mail/vhosts}"

  log "Configuring postfix+dovecot vmail stack..."

  if ! getent group vmail >/dev/null 2>&1; then
    groupadd -g "${vmail_gid}" vmail >/dev/null 2>&1 || true
  fi
  if ! id -u vmail >/dev/null 2>&1; then
    useradd -g vmail -u "${vmail_uid}" -d "${vmail_base}" -m -s /usr/sbin/nologin vmail >/dev/null 2>&1 || true
  fi

  mkdir -p "${vmail_base}" /etc/dovecot /etc/dovecot/conf.d /etc/postfix
  chown -R "${vmail_uid}:${vmail_gid}" "${vmail_base}" >/dev/null 2>&1 || true
  chmod 750 "${vmail_base}" >/dev/null 2>&1 || true

  touch /etc/dovecot/users /etc/postfix/vmailbox /etc/postfix/virtual /etc/postfix/virtual_regexp
  chmod 640 /etc/dovecot/users /etc/postfix/vmailbox /etc/postfix/virtual /etc/postfix/virtual_regexp >/dev/null 2>&1 || true

  if command -v postmap >/dev/null 2>&1; then
    postmap /etc/postfix/vmailbox >/dev/null 2>&1 || true
    postmap /etc/postfix/virtual >/dev/null 2>&1 || true
  fi

  cat <<EOF >/etc/dovecot/conf.d/90-aurapanel-vmail.conf
passdb {
  driver = passwd-file
  args = scheme=SHA512-CRYPT username_format=%u /etc/dovecot/users
}

userdb {
  driver = passwd-file
  args = username_format=%u /etc/dovecot/users
}

mail_uid = ${vmail_uid}
mail_gid = ${vmail_gid}
mail_home = ${vmail_base}/%d/%n
mail_location = maildir:~/Maildir

plugin {
  quota = maildir:User quota
  quota_rule = *:storage=1024M
}
EOF

  if command -v postconf >/dev/null 2>&1; then
    postconf -e "virtual_mailbox_base = ${vmail_base}" >/dev/null 2>&1 || true
    postconf -e "virtual_mailbox_maps = hash:/etc/postfix/vmailbox" >/dev/null 2>&1 || true
    postconf -e "virtual_alias_maps = hash:/etc/postfix/virtual,regexp:/etc/postfix/virtual_regexp" >/dev/null 2>&1 || true
    postconf -e "virtual_minimum_uid = ${vmail_uid}" >/dev/null 2>&1 || true
    postconf -e "virtual_uid_maps = static:${vmail_uid}" >/dev/null 2>&1 || true
    postconf -e "virtual_gid_maps = static:${vmail_gid}" >/dev/null 2>&1 || true
  fi

  systemctl restart dovecot >/dev/null 2>&1 || true
  systemctl restart postfix >/dev/null 2>&1 || true
  ok "vmail stack baseline configured."
}

ensure_node20() {
  local has_node="0"
  if command -v node >/dev/null 2>&1; then
    local node_major
    node_major="$(node -v | sed 's/^v//' | cut -d'.' -f1)"
    if [ "${node_major}" -ge 20 ]; then
      has_node="1"
    fi
  fi

  if [ "${has_node}" = "1" ]; then
    ok "Node.js $(node -v) already available."
    return
  fi

  log "Installing Node.js 20.x from mirror..."
  curl -fsSL "${NODE_SETUP_URL}" | bash -
  install_packages nodejs
  ok "Node.js installed: $(node -v)"
}

ensure_rust() {
  log "Ensuring Rust toolchain..."
  if ! command -v cargo >/dev/null 2>&1; then
    curl --proto '=https' --tlsv1.2 -sSf "${RUSTUP_INIT_SCRIPT_URL}" | sh -s -- -y
  fi

  if [ -f "${HOME}/.cargo/env" ]; then
    # shellcheck disable=SC1090
    source "${HOME}/.cargo/env"
  fi
}

ensure_go() {
  log "Ensuring Go toolchain..."
  if ! command -v go >/dev/null 2>&1; then
    wget -q "${GO_TARBALL_URL}" -O "/tmp/${GO_TARBALL}"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
    rm -f "/tmp/${GO_TARBALL}"
  fi

  export PATH="$PATH:/usr/local/go/bin"
}

ensure_openlitespeed() {
  if [ -x /usr/local/lsws/bin/lswsctrl ]; then
    ok "OpenLiteSpeed already installed."
  else
    log "Installing OpenLiteSpeed..."
    curl -fsSL "${LITESPEED_REPO_SCRIPT_URL}" | bash

    install_packages openlitespeed || fail "OpenLiteSpeed installation failed."
  fi

  # Ensure lsphp toolchain is present even when OpenLiteSpeed was preinstalled.
  install_optional_packages lsphp83 lsphp83-common lsphp83-mysql lsphp83-curl lsphp83-xml lsphp83-zip lsphp83-opcache

  systemctl enable lshttpd >/dev/null 2>&1 || true
  systemctl restart lshttpd >/dev/null 2>&1 || true
}

ensure_ols_public_listeners() {
  local ols_conf="/usr/local/lsws/conf/httpd_config.conf"
  local tls_key="/usr/local/lsws/admin/conf/webadmin.key"
  local tls_crt="/usr/local/lsws/admin/conf/webadmin.crt"

  if [ ! -f "${ols_conf}" ]; then
    warn "OpenLiteSpeed config not found: ${ols_conf}"
    return 0
  fi

  if [ ! -f "${tls_key}" ] || [ ! -f "${tls_crt}" ]; then
    log "Generating fallback TLS cert for OpenLiteSpeed listener..."
    mkdir -p /usr/local/lsws/admin/conf
    openssl req -x509 -nodes -newkey rsa:2048 \
      -keyout "${tls_key}" \
      -out "${tls_crt}" \
      -days 3650 \
      -subj "/CN=$(hostname -f 2>/dev/null || hostname)" >/dev/null 2>&1 || true
    chmod 600 "${tls_key}" >/dev/null 2>&1 || true
    chmod 644 "${tls_crt}" >/dev/null 2>&1 || true
  fi

  cp -n "${ols_conf}" "${ols_conf}.aurapanel.bak" >/dev/null 2>&1 || true

  # Ensure default public HTTP listener is on port 80.
  sed -i '/^[[:space:]]*listener[[:space:]]\+Default{/,/^[[:space:]]*}/{s/^[[:space:]]*address[[:space:]]\+.*/    address                  *:80/;s/^[[:space:]]*secure[[:space:]]\+.*/    secure                   0/}' "${ols_conf}"

  # Ensure a public HTTPS listener exists.
  if ! grep -Eq '^[[:space:]]*listener[[:space:]]+AuraPanelSSL[[:space:]]*\{' "${ols_conf}"; then
    cat <<EOF >> "${ols_conf}"

listener AuraPanelSSL{
    address                  *:443
    secure                   1
    keyFile                  ${tls_key}
    certFile                 ${tls_crt}
    map                      Example *
}
EOF
    ok "OpenLiteSpeed HTTPS listener AuraPanelSSL ensured on 443."
  fi

  # Template vhosts should be attached to both HTTP/HTTPS listeners.
  sed -i 's/^\([[:space:]]*listeners[[:space:]]\+\)Default[[:space:]]*$/\1Default,AuraPanelSSL/' "${ols_conf}"

  if ! /usr/local/lsws/bin/lswsctrl restart >/dev/null 2>&1; then
    warn "OpenLiteSpeed restart failed after listener update. Check ${ols_conf}."
  else
    ok "OpenLiteSpeed listeners synchronized (80/443)."
  fi
}

ensure_openlitespeed_admin_php() {
  local admin_php="/usr/local/lsws/admin/fcgi-bin/admin_php"
  local target_lsphp="/usr/local/lsws/lsphp83/bin/lsphp"

  mkdir -p /usr/local/lsws/admin/fcgi-bin
  if [ ! -x "${target_lsphp}" ]; then
    warn "OpenLiteSpeed admin PHP binary missing, trying to reinstall lsphp83..."
    install_optional_packages lsphp83 lsphp83-common lsphp83-mysql || true
  fi

  if [ -x "${target_lsphp}" ]; then
    ln -snf ../../lsphp83/bin/lsphp "${admin_php}"
  fi
}

ensure_lsphp_database_drivers() {
  log "Ensuring lsphp83 database drivers (mysql, pgsql, sqlite3)..."
  install_optional_packages lsphp83-mysql lsphp83-pgsql lsphp83-sqlite3

  local ext_dir
  ext_dir="$(/usr/local/lsws/lsphp83/bin/lsphp -i 2>/dev/null | awk -F'=> ' '/^extension_dir =>/{print $2; exit}' | awk '{print $1}')"
  if [ -n "${ext_dir}" ]; then
    local missing=()
    for so in pdo_mysql.so pdo_pgsql.so pdo_sqlite.so; do
      if [ ! -f "${ext_dir}/${so}" ]; then
        missing+=("${so}")
      fi
    done
    if [ "${#missing[@]}" -gt 0 ]; then
      warn "Some lsphp83 PDO drivers are missing: ${missing[*]}"
    else
      ok "lsphp83 PDO drivers are present (mysql, pgsql, sqlite)."
    fi
  else
    warn "Could not detect lsphp83 extension_dir for PDO driver verification."
  fi
}

ensure_ioncube_loader() {
  if [ "${AURAPANEL_INSTALL_IONCUBE:-1}" != "1" ]; then
    warn "ionCube loader install skipped (AURAPANEL_INSTALL_IONCUBE!=1)."
    return
  fi

  local lsphp_bin="/usr/local/lsws/lsphp83/bin/lsphp"
  local ioncube_url="https://downloads.ioncube.com/loader_downloads/ioncube_loaders_lin_x86-64.tar.gz"
  local ext_dir ini_file tmpdir archive src_loader dst_loader

  if [ ! -x "${lsphp_bin}" ]; then
    warn "lsphp83 binary not found; ionCube install skipped."
    return
  fi

  if "${lsphp_bin}" -i 2>/dev/null | grep -qi "with the ionCube PHP Loader"; then
    ok "ionCube loader already active for lsphp83."
    return
  fi

  ext_dir="$("${lsphp_bin}" -i 2>/dev/null | awk -F'=> ' '/^extension_dir =>/{print $2; exit}' | awk '{print $1}')"
  if [ -z "${ext_dir}" ]; then
    warn "Unable to detect lsphp83 extension_dir; ionCube install skipped."
    return
  fi

  ini_file="/usr/local/lsws/lsphp83/etc/php/8.3/mods-available/00-ioncube.ini"
  tmpdir="$(mktemp -d /tmp/ioncube.XXXXXX)"
  archive="${tmpdir}/ioncube_loaders_lin_x86-64.tar.gz"
  src_loader="${tmpdir}/ioncube/ioncube_loader_lin_8.3.so"
  dst_loader="${ext_dir}/ioncube_loader_lin_8.3.so"

  if ! download_file "${ioncube_url}" "${archive}"; then
    warn "ionCube archive download failed; continuing without ionCube."
    rm -rf "${tmpdir}"
    return
  fi

  if ! tar -xzf "${archive}" -C "${tmpdir}"; then
    warn "ionCube archive extract failed; continuing without ionCube."
    rm -rf "${tmpdir}"
    return
  fi

  if [ ! -f "${src_loader}" ]; then
    warn "ionCube loader for PHP 8.3 not found in archive."
    rm -rf "${tmpdir}"
    return
  fi

  mkdir -p "${ext_dir}" "$(dirname "${ini_file}")"
  install -m 755 "${src_loader}" "${dst_loader}"
  cat <<EOF > "${ini_file}"
; ionCube Loader
zend_extension=${dst_loader}
EOF
  chmod 644 "${ini_file}"

  if systemctl restart lshttpd >/dev/null 2>&1; then
    if "${lsphp_bin}" -i 2>/dev/null | grep -qi "with the ionCube PHP Loader"; then
      ok "ionCube loader installed and activated for lsphp83."
    else
      warn "ionCube files installed, but loader activation could not be verified."
    fi
  else
    warn "lshttpd restart failed after ionCube install. Check OpenLiteSpeed logs."
  fi

  rm -rf "${tmpdir}"
}

ensure_certbot() {
  if command -v certbot >/dev/null 2>&1; then
    ok "Certbot already installed: $(certbot --version 2>/dev/null | head -n1)"
  else
    log "Installing Certbot..."
    install_packages certbot || fail "Certbot installation failed."
  fi

  if [ "${PKG_MGR}" = "apt" ]; then
    install_optional_packages python3-certbot-dns-cloudflare python3-certbot-dns-rfc2136
  else
    install_optional_packages python3-certbot-dns-cloudflare python3-certbot-dns-rfc2136 certbot-dns-cloudflare certbot-dns-rfc2136
  fi

  if ! command -v certbot >/dev/null 2>&1; then
    fail "Certbot binary is missing after installation."
  fi
}

configure_ols_admin_credentials() {
  local ols_user="admin"
  local ols_pass=""
  local ols_conf_dir="/usr/local/lsws/admin/conf"
  local ols_htpasswd="${ols_conf_dir}/htpasswd"
  local ols_admin_php="/usr/local/lsws/admin/fcgi-bin/admin_php"
  local ols_htpasswd_php="/usr/local/lsws/admin/misc/htpasswd.php"
  local applied="0"
  local hashed_pass=""

  if [ -f "${OLS_ADMIN_STATE_FILE}" ]; then
    ols_user="$(read_env_value "${OLS_ADMIN_STATE_FILE}" "AURAPANEL_OLS_ADMIN_USER")"
    ols_pass="$(read_env_value "${OLS_ADMIN_STATE_FILE}" "AURAPANEL_OLS_ADMIN_PASSWORD")"
  fi

  ols_user="${ols_user:-admin}"

  if [ -z "${ols_pass}" ]; then
    ols_pass="$(LC_ALL=C tr -dc 'A-Za-z0-9@#%+=._-' < /dev/urandom | head -c 22 || true)"
    if [ -z "${ols_pass}" ]; then
      ols_pass="$(openssl rand -hex 16 | tr -d '\n')"
    fi
  fi

  mkdir -p "${GATEWAY_ENV_DIR}" "${ols_conf_dir}"
  cat <<EOF > "${OLS_ADMIN_STATE_FILE}"
AURAPANEL_OLS_ADMIN_USER=${ols_user}
AURAPANEL_OLS_ADMIN_PASSWORD=${ols_pass}
EOF
  chmod 600 "${OLS_ADMIN_STATE_FILE}"

  if [ -x /usr/local/lsws/admin/misc/admpass.sh ]; then
    if printf '%s\n%s\n%s\n' "${ols_user}" "${ols_pass}" "${ols_pass}" | /usr/local/lsws/admin/misc/admpass.sh >/dev/null 2>&1; then
      ok "OpenLiteSpeed admin password initialized."
      applied="1"
    else
      warn "admpass.sh failed, falling back to htpasswd sync."
    fi
  fi

  if [ "${applied}" != "1" ] && [ -x "${ols_admin_php}" ] && [ -f "${ols_htpasswd_php}" ]; then
    hashed_pass="$("${ols_admin_php}" -c /usr/local/lsws/admin/conf/php.ini -q "${ols_htpasswd_php}" "${ols_pass}" 2>/dev/null || true)"
    if [ -n "${hashed_pass}" ]; then
      printf '%s:%s\n' "${ols_user}" "${hashed_pass}" > "${ols_htpasswd}"
      chmod 600 "${ols_htpasswd}"
      applied="1"
    fi
  fi

  if [ "${applied}" != "1" ]; then
    fail "OpenLiteSpeed admin credentials could not be applied."
  fi

  systemctl restart lshttpd >/dev/null 2>&1 || true
}

write_access_summary() {
  local panel_port panel_user panel_pass ols_user ols_pass
  panel_port="$(gateway_port)"
  panel_user="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_EMAIL")"
  panel_pass="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD")"
  ols_user="$(read_env_value "${OLS_ADMIN_STATE_FILE}" "AURAPANEL_OLS_ADMIN_USER")"
  ols_pass="$(read_env_value "${OLS_ADMIN_STATE_FILE}" "AURAPANEL_OLS_ADMIN_PASSWORD")"

  panel_user="${panel_user:-admin@server.com}"
  if [ -z "${panel_pass}" ] && [ -f "${PROJECT_DIR}/logs/initial_password.txt" ]; then
    panel_pass="$(tr -d '\r\n' < "${PROJECT_DIR}/logs/initial_password.txt")"
  fi
  ols_user="${ols_user:-admin}"

  if [ ! -f "${CREDENTIALS_SUMMARY_FILE}" ]; then
    cat <<EOF > "${CREDENTIALS_SUMMARY_FILE}"
AuraPanel Initial Access
=======================
Panel URL: http://YOUR_SERVER_IP:${panel_port}
Panel Login: ${panel_user}
Panel Password: ${panel_pass:-<not available>}

OpenLiteSpeed WebAdmin URL: https://YOUR_SERVER_IP:7080
OpenLiteSpeed Username: ${ols_user}
OpenLiteSpeed Password: ${ols_pass:-<not available>}

${ONE_TIME_PASSWORD_NOTE}
EOF
    chmod 600 "${CREDENTIALS_SUMMARY_FILE}"
    ok "Access summary written to ${CREDENTIALS_SUMMARY_FILE}"
  fi

  ok "AuraPanel Login: ${panel_user}"
  ok "AuraPanel Password: ${panel_pass:-<not available>}"
  ok "OpenLiteSpeed Username: ${ols_user}"
  ok "OpenLiteSpeed Password: ${ols_pass:-<not available>}"
  warn "${ONE_TIME_PASSWORD_NOTE}"
}

ensure_minio_binaries() {
  if ! command -v minio >/dev/null 2>&1; then
    log "Installing MinIO binary..."
    wget -q "${MINIO_BIN_URL}" -O /usr/local/bin/minio
    chmod +x /usr/local/bin/minio
  fi

  if ! command -v mc >/dev/null 2>&1; then
    log "Installing MinIO client (mc)..."
    wget -q "${MINIO_MC_URL}" -O /usr/local/bin/mc
    chmod +x /usr/local/bin/mc
  fi
}

write_core_env_defaults() {
  mkdir -p "${GATEWAY_ENV_DIR}" "${PROJECT_DIR}/logs"
  chmod 700 "${GATEWAY_ENV_DIR}"
  local shared_jwt_secret=""

  if [ ! -f "${GATEWAY_ENV_FILE}" ]; then
    local admin_pass jwt_secret
    admin_pass="$(openssl rand -base64 18 | tr -d '\n')"
    jwt_secret="$(openssl rand -hex 32 | tr -d '\n')"
    shared_jwt_secret="${jwt_secret}"

    cat <<EOF > "${GATEWAY_ENV_FILE}"
AURAPANEL_ADMIN_EMAIL=admin@server.com
AURAPANEL_ADMIN_PASSWORD=${admin_pass}
AURAPANEL_JWT_SECRET=${jwt_secret}
AURAPANEL_JWT_ISSUER=aurapanel-gateway
AURAPANEL_JWT_AUDIENCE=aurapanel-ui
AURAPANEL_ALLOWED_ORIGINS=http://127.0.0.1:${PANEL_PORT_DEFAULT},http://localhost:${PANEL_PORT_DEFAULT}
AURAPANEL_CORE_URL=http://127.0.0.1:8000
AURAPANEL_GATEWAY_ONLY=1
AURAPANEL_GATEWAY_ADDR=:${PANEL_PORT_DEFAULT}
AURAPANEL_PANEL_DIST=/opt/aurapanel/frontend/dist
EOF

    chmod 600 "${GATEWAY_ENV_FILE}"
    echo "${admin_pass}" > "${PROJECT_DIR}/logs/initial_password.txt"
    chmod 600 "${PROJECT_DIR}/logs/initial_password.txt"
    ok "Initial admin password written to ${PROJECT_DIR}/logs/initial_password.txt"
  fi

  if [ -z "${shared_jwt_secret}" ]; then
    shared_jwt_secret="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_JWT_SECRET")"
  fi

  if [ ! -f "${CORE_ENV_FILE}" ]; then
    local restic_pass minio_access minio_secret
    restic_pass="$(openssl rand -hex 24 | tr -d '\n')"
    minio_access="backup$(openssl rand -hex 3 | tr -d '\n')"
    minio_secret="$(openssl rand -hex 24 | tr -d '\n')"

    cat <<EOF > "${CORE_ENV_FILE}"
AURAPANEL_RUNTIME_MODE=production
AURAPANEL_SECURITY_POLICY=fail-closed
AURAPANEL_GATEWAY_ONLY=1
AURAPANEL_JWT_SECRET=${shared_jwt_secret}
AURAPANEL_CORE_BIND_ADDR=127.0.0.1:8000
AURAPANEL_FEDERATION_MODE=active-passive
AURAPANEL_FEDERATION_PRIMARY=1
AURAPANEL_BACKUP_TARGET=internal-minio
AURAPANEL_BACKUP_MINIO_ENDPOINT=http://127.0.0.1:9000
AURAPANEL_BACKUP_MINIO_BUCKET=aurapanel-backups
AURAPANEL_BACKUP_MINIO_ACCESS_KEY=${minio_access}
AURAPANEL_BACKUP_MINIO_SECRET_KEY=${minio_secret}
AURAPANEL_BACKUP_RESTIC_PASSWORD=${restic_pass}
AURAPANEL_MAIL_BACKEND=vmail
AURAPANEL_WEBMAIL_BASE_URL=http://127.0.0.1/webmail
AURAPANEL_MAIL_VMAIL_UID=5000
AURAPANEL_MAIL_VMAIL_GID=5000
AURAPANEL_MAIL_VMAIL_BASE=/var/mail/vhosts
EOF

    chmod 600 "${CORE_ENV_FILE}"
  fi

  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_GATEWAY_ONLY" "1"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_CORE_URL" "http://127.0.0.1:8000"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_GATEWAY_ADDR" ":${PANEL_PORT_DEFAULT}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_PANEL_DIST" "${PROJECT_DIR}/frontend/dist"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_ALLOWED_ORIGINS" "http://127.0.0.1:${PANEL_PORT_DEFAULT},http://localhost:${PANEL_PORT_DEFAULT}"

  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_RUNTIME_MODE" "production"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_SECURITY_POLICY" "fail-closed"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_GATEWAY_ONLY" "1"
  if [ -n "${shared_jwt_secret}" ]; then
    upsert_env "${CORE_ENV_FILE}" "AURAPANEL_JWT_SECRET" "${shared_jwt_secret}"
  fi
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_CORE_BIND_ADDR" "127.0.0.1:8000"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_FEDERATION_MODE" "active-passive"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_FEDERATION_PRIMARY" "1"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_BACKUP_TARGET" "internal-minio"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_BACKUP_MINIO_ENDPOINT" "http://127.0.0.1:9000"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_BACKUP_MINIO_BUCKET" "aurapanel-backups"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_MAIL_BACKEND" "vmail"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_WEBMAIL_BASE_URL" "http://127.0.0.1/webmail"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_MAIL_VMAIL_UID" "5000"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_MAIL_VMAIL_GID" "5000"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_MAIL_VMAIL_BASE" "/var/mail/vhosts"

  chmod 600 "${GATEWAY_ENV_FILE}" "${CORE_ENV_FILE}"
}

configure_minio_service() {
  ensure_minio_binaries

  # shellcheck disable=SC1090
  source "${CORE_ENV_FILE}"

  id -u minio-user >/dev/null 2>&1 || useradd --system --home /var/lib/minio --shell /sbin/nologin minio-user
  mkdir -p /var/lib/minio /etc/minio
  chown -R minio-user:minio-user /var/lib/minio /etc/minio

  cat <<EOF > "${MINIO_ENV_FILE}"
MINIO_ROOT_USER=${AURAPANEL_BACKUP_MINIO_ACCESS_KEY}
MINIO_ROOT_PASSWORD=${AURAPANEL_BACKUP_MINIO_SECRET_KEY}
MINIO_VOLUMES=/var/lib/minio
MINIO_OPTS=--address 127.0.0.1:9000 --console-address 127.0.0.1:9001
EOF
  chmod 600 "${MINIO_ENV_FILE}"

  cat <<'EOF' > /etc/systemd/system/minio.service
[Unit]
Description=MinIO
After=network-online.target
Wants=network-online.target

[Service]
User=minio-user
Group=minio-user
EnvironmentFile=-/etc/default/minio
ExecStart=/usr/local/bin/minio server $MINIO_VOLUMES $MINIO_OPTS
Restart=always
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable minio
  systemctl restart minio

  for _ in $(seq 1 20); do
    if curl -fsS http://127.0.0.1:9000/minio/health/live >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done

  if command -v mc >/dev/null 2>&1; then
    mc alias set local http://127.0.0.1:9000 "${AURAPANEL_BACKUP_MINIO_ACCESS_KEY}" "${AURAPANEL_BACKUP_MINIO_SECRET_KEY}" >/dev/null 2>&1 || true
    mc mb --ignore-existing "local/${AURAPANEL_BACKUP_MINIO_BUCKET}" >/dev/null 2>&1 || true
  fi
}

enable_stack_services() {
  local services=(mariadb postgresql redis-server redis docker fail2ban pdns pure-ftpd postfix dovecot)

  for svc in "${services[@]}"; do
    if systemctl list-unit-files | grep -qE "^${svc}\\.service"; then
      systemctl enable "${svc}" >/dev/null 2>&1 || true
      systemctl restart "${svc}" >/dev/null 2>&1 || true
    fi
  done
}

sync_project() {
  log "Preparing project directory at ${PROJECT_DIR}..."
  mkdir -p "${PROJECT_DIR}"

  if [ -d "$(pwd)/core" ] && [ -d "$(pwd)/api-gateway" ] && [ -d "$(pwd)/frontend" ]; then
    log "Copying current workspace into ${PROJECT_DIR}..."
    rsync -a --delete \
      --exclude '.git' \
      --exclude 'core/target' \
      --exclude 'frontend/node_modules' \
      --exclude 'api-gateway/apigw' \
      "$(pwd)/" "${PROJECT_DIR}/"
  else
    if [ "${AURAPANEL_INSTALL_SOURCE:-archive}" = "archive" ]; then
      local archive_file="/tmp/aurapanel-source-latest.tar.gz"
      local checksum_file="${archive_file}.sha256"

      log "Downloading AuraPanel source archive from ${SOURCE_ARCHIVE_URL}..."
      if download_file "${SOURCE_ARCHIVE_URL}" "${archive_file}"; then
        if download_file "${SOURCE_ARCHIVE_SHA256_URL}" "${checksum_file}"; then
          local expected_sha actual_sha
          if [ ! -s "${checksum_file}" ]; then
            warn "Source archive checksum file is empty; falling back to git."
            expected_sha=""
          else
            expected_sha="$(awk '{print $1}' "${checksum_file}" | head -n1)"
          fi
          actual_sha="$(sha256sum "${archive_file}" | awk '{print $1}')"
          if [ -z "${expected_sha}" ] || [ "${expected_sha}" != "${actual_sha}" ]; then
            warn "Source archive checksum mismatch; falling back to git."
          else
            rm -rf "${PROJECT_DIR}"
            mkdir -p "${PROJECT_DIR}"
            tar -xzf "${archive_file}" -C "${PROJECT_DIR}" --strip-components=1
            ok "Source archive extracted successfully."
            return
          fi
        else
          warn "Source archive checksum file not available; falling back to git."
        fi
      else
        warn "Source archive download failed; falling back to git."
      fi
    fi

    if [ "${AURAPANEL_ALLOW_GIT_FALLBACK}" != "1" ]; then
      fail "Source archive is unavailable and git fallback is disabled (AURAPANEL_ALLOW_GIT_FALLBACK=0)."
    fi

    if [ ! -d "${PROJECT_DIR}/.git" ]; then
      log "Cloning repository from ${REPO_URL}..."
      rm -rf "${PROJECT_DIR}"
      git clone "${REPO_URL}" "${PROJECT_DIR}"
    else
      log "Updating existing repository..."
      git -C "${PROJECT_DIR}" fetch --all
      git -C "${PROJECT_DIR}" pull --ff-only
    fi
  fi
}

build_components() {
  log "Building Rust core..."
  cd "${PROJECT_DIR}/core"
  cargo build --release

  log "Building Go API gateway..."
  cd "${PROJECT_DIR}/api-gateway"
  /usr/local/go/bin/go mod tidy
  /usr/local/go/bin/go build -o apigw .

  log "Building Vue frontend (production dist)..."
  cd "${PROJECT_DIR}/frontend"
  npm ci
  npm run build
}

configure_systemd_services() {
  log "Configuring systemd services..."

  cat <<EOF > /etc/systemd/system/aurapanel-core.service
[Unit]
Description=AuraPanel Core (Rust)
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${PROJECT_DIR}/core
ExecStart=${PROJECT_DIR}/core/target/release/aurapanel-core
Restart=on-failure
Environment=RUST_LOG=info
EnvironmentFile=-${CORE_ENV_FILE}

[Install]
WantedBy=multi-user.target
EOF

  cat <<EOF > /etc/systemd/system/aurapanel-api.service
[Unit]
Description=AuraPanel API Gateway (Go + Panel Static)
After=network.target aurapanel-core.service
Requires=aurapanel-core.service

[Service]
Type=simple
User=root
WorkingDirectory=${PROJECT_DIR}/api-gateway
ExecStart=${PROJECT_DIR}/api-gateway/apigw
Restart=on-failure
EnvironmentFile=-${GATEWAY_ENV_FILE}

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable aurapanel-core aurapanel-api
  systemctl restart aurapanel-core aurapanel-api
}

smoke_check() {
  log "Running post-install smoke checks..."
  local panel_port
  panel_port="$(gateway_port)"

  systemctl is-active --quiet aurapanel-core || fail "aurapanel-core is not active"
  systemctl is-active --quiet aurapanel-api || fail "aurapanel-api is not active"
  systemctl is-active --quiet lshttpd || fail "lshttpd is not active"
  systemctl is-active --quiet minio || fail "minio is not active"
  if systemctl list-unit-files | grep -q '^pure-ftpd\.service'; then
    systemctl is-active --quiet pure-ftpd || fail "pure-ftpd is not active"
  fi

  curl -fsS http://127.0.0.1:8000/api/v1/health >/dev/null || fail "Core health check failed"
  curl -fsS "http://127.0.0.1:${panel_port}/api/health" >/dev/null || fail "Gateway health check failed"
  curl -fsS "http://127.0.0.1:${panel_port}/" >/dev/null || fail "Panel static endpoint failed"
  curl -fsS http://127.0.0.1:9000/minio/health/live >/dev/null || fail "MinIO health check failed"
  curl -fsS http://127.0.0.1/webmail/ >/dev/null 2>&1 || warn "Roundcube endpoint check skipped/failed (non-fatal)."

  if command -v ss >/dev/null 2>&1; then
    if ! ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)80$'; then
      warn "Port 80 listener was not detected. HTTP-01 SSL validation may fail."
    fi
    if ! ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)443$'; then
      warn "Port 443 listener was not detected. Public HTTPS traffic may fail."
    fi
  fi

  ok "Smoke checks passed."
}

main() {
  echo -e "${BLUE}=================================================${NC}"
  echo -e "${GREEN} AuraPanel - Production Installation ${NC}"
  echo -e "${BLUE}=================================================${NC}"

  log "Installing system prerequisites..."
  if [ "${PKG_MGR}" = "apt" ]; then
    apt-get update -y
    install_packages curl wget git rsync build-essential cmake pkg-config libssl-dev gcc ufw ca-certificates openssl jq unzip tar
    install_packages software-properties-common gnupg lsb-release
  else
    dnf update -y
    dnf groupinstall -y "Development Tools"
    install_packages curl wget git rsync cmake openssl-devel openssl gcc firewalld ca-certificates jq unzip tar
    install_packages dnf-plugins-core
  fi

  install_optional_packages restic mariadb-server postgresql redis-server redis docker docker.io fail2ban powerdns pdns pure-ftpd postfix dovecot-core dovecot-imapd

  ensure_rust
  ensure_go
  ensure_node20
  ensure_openlitespeed
  ensure_ols_public_listeners
  ensure_openlitespeed_admin_php
  ensure_lsphp_database_drivers
  ensure_ioncube_loader
  ensure_certbot
  configure_ols_admin_credentials
  configure_pureftpd

  sync_project
  write_core_env_defaults
  configure_minio_service
  configure_roundcube
  configure_mail_stack_vmail
  build_components
  configure_systemd_services
  enable_stack_services
  configure_standard_firewall "$(gateway_port)"
  smoke_check

  local panel_port
  panel_port="$(gateway_port)"
  ok "AuraPanel deployment is complete."
  ok "Panel URL: http://YOUR_SERVER_IP:${panel_port}"
  ok "API Health: http://YOUR_SERVER_IP:${panel_port}/api/health"
  ok "Core Health (internal): http://127.0.0.1:8000/api/v1/health"
  ok "OpenLiteSpeed Web: http://YOUR_SERVER_IP (80/443)"
  ok "OpenLiteSpeed Admin: https://YOUR_SERVER_IP:7080"
  write_access_summary
}

main "$@"
