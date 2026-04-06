#!/usr/bin/env bash
# AuraPanel Production Installation Script
# Supported OS: Ubuntu 22.04/24.04, Debian 12+, AlmaLinux 8/9, Rocky Linux 8/9
# Usage:
# curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
# Optional (if mirror is protected): export AURAPANEL_DOWNLOAD_AUTH="user:pass"

set -euo pipefail

GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

PROJECT_DIR="/opt/aurapanel"
GATEWAY_ENV_DIR="/etc/aurapanel"
GATEWAY_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel.env"
SERVICE_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-service.env"
OLS_ADMIN_STATE_FILE="${GATEWAY_ENV_DIR}/aurapanel-ols-admin.env"
MINIO_ENV_FILE="/etc/default/minio"
CREDENTIALS_SUMMARY_FILE="/root/aurapanel_credentials.txt"
REPO_URL="https://github.com/mkoyazilim/aurapanel.git"
AURAPANEL_DOWNLOAD_BASE_URL="${AURAPANEL_DOWNLOAD_BASE_URL:-https://downloads.aurapanel.info/mirror}"
RELEASE_BASE_URL="${AURAPANEL_RELEASE_BASE_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/releases}"

SOURCE_ARCHIVE_URL="${AURAPANEL_SOURCE_ARCHIVE_URL:-${RELEASE_BASE_URL}/aurapanel-source-latest.tar.gz}"
SOURCE_ARCHIVE_SHA256_URL="${AURAPANEL_SOURCE_ARCHIVE_SHA256_URL:-${SOURCE_ARCHIVE_URL}.sha256}"
AURAPANEL_ALLOW_GIT_FALLBACK="${AURAPANEL_ALLOW_GIT_FALLBACK:-0}"
AURAPANEL_INSTALL_SOURCE="${AURAPANEL_INSTALL_SOURCE:-archive}"
POWERDNS_REPO_KEY_URL="${AURAPANEL_POWERDNS_REPO_KEY_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/pdns/FD380FBB-pub.asc}"
POWERDNS_REPO_CHANNEL="${AURAPANEL_POWERDNS_REPO_CHANNEL:-auth-50}"

NODE_SETUP_URL="${AURAPANEL_NODE_SETUP_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/node/setup_20.x}"
GO_VERSION="${AURAPANEL_GO_VERSION:-1.22.1}"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_TARBALL_URL="${AURAPANEL_GO_TARBALL_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/go/${GO_TARBALL}}"
LITESPEED_REPO_SCRIPT_URL="${AURAPANEL_LITESPEED_REPO_SCRIPT_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/litespeed/repo.sh}"
OLS_MANIFEST_URL="${AURAPANEL_OLS_MANIFEST_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/litespeed/openlitespeed-manifest.env}"
AURAPANEL_OLS_USE_MIRROR_PACKAGE="${AURAPANEL_OLS_USE_MIRROR_PACKAGE:-1}"
MINIO_BIN_URL="${AURAPANEL_MINIO_BIN_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/minio/minio}"
MINIO_MC_URL="${AURAPANEL_MINIO_MC_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/minio/mc}"
ROUNDCUBE_VERSION="${AURAPANEL_ROUNDCUBE_VERSION:-1.6.11}"
ROUNDCUBE_ARCHIVE_URL="${AURAPANEL_ROUNDCUBE_ARCHIVE_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/roundcube/roundcubemail-${ROUNDCUBE_VERSION}-complete.tar.gz}"
OWASP_CRS_VERSION="${AURAPANEL_OWASP_CRS_VERSION:-v4.2.0}"
OWASP_CRS_ARCHIVE_URL="${AURAPANEL_OWASP_CRS_ARCHIVE_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/owasp/${OWASP_CRS_VERSION}.zip}"
WP_CLI_PHAR_URL="${AURAPANEL_WP_CLI_PHAR_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/wpcli/wp-cli.phar}"
IONCUBE_ARCHIVE_URL="${AURAPANEL_IONCUBE_ARCHIVE_URL:-${AURAPANEL_DOWNLOAD_BASE_URL}/deps/ioncube/ioncube_loaders_lin_x86-64.tar.gz}"
AURAPANEL_FFMPEG_INSTALL="${AURAPANEL_FFMPEG_INSTALL:-1}"
AURAPANEL_FFMPEG_ACTIVE="${AURAPANEL_FFMPEG_ACTIVE:-1}"
AURAPANEL_IMAGEMAGICK_INSTALL="${AURAPANEL_IMAGEMAGICK_INSTALL:-1}"
AURAPANEL_IMAGEMAGICK_ACTIVE="${AURAPANEL_IMAGEMAGICK_ACTIVE:-1}"
AURAPANEL_LIBREOFFICE_INSTALL="${AURAPANEL_LIBREOFFICE_INSTALL:-1}"
AURAPANEL_LIBREOFFICE_ACTIVE="${AURAPANEL_LIBREOFFICE_ACTIVE:-1}"
PANEL_PORT_DEFAULT="8090"
ONE_TIME_PASSWORD_NOTE="NOTE: Passwords are generated only once. Please save them now or change them immediately."
PDNS_POLICY_RC_D_PATH="/usr/sbin/policy-rc.d"
PDNS_POLICY_RC_D_BACKUP="/usr/sbin/policy-rc.d.aurapanel-backup"
PDNS_POLICY_RC_D_MARKER="AURAPANEL_PDNS_AUTOSTART_GUARD"
PDNS_POLICY_RC_D_INSTALLED="0"

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

normalize_yes_no() {
  local value="${1:-}"
  local fallback="${2:-0}"
  case "${value,,}" in
    1|y|yes|true|on) echo "1" ;;
    0|n|no|false|off) echo "0" ;;
    "") echo "${fallback}" ;;
    *) echo "${fallback}" ;;
  esac
}

prompt_yes_no() {
  local prompt="$1"
  local default_value="${2:-1}"
  local default_label response normalized

  default_value="$(normalize_yes_no "${default_value}" "1")"
  if [ "${default_value}" = "1" ]; then
    default_label="Yes"
  else
    default_label="No"
  fi

  if [ "${AURAPANEL_INTERACTIVE_PROMPTS:-1}" != "1" ] || [ ! -t 0 ]; then
    echo "${default_value}"
    return 0
  fi

  read -r -p "${prompt} (Yes/No) [${default_label}]: " response || true
  normalized="$(normalize_yes_no "${response}" "${default_value}")"
  echo "${normalized}"
}

cleanup_runtime_guards() {
  if [ "${PDNS_POLICY_RC_D_INSTALLED}" != "1" ]; then
    return 0
  fi

  if [ -f "${PDNS_POLICY_RC_D_BACKUP}" ]; then
    mv -f "${PDNS_POLICY_RC_D_BACKUP}" "${PDNS_POLICY_RC_D_PATH}"
  elif [ -f "${PDNS_POLICY_RC_D_PATH}" ] && grep -q "${PDNS_POLICY_RC_D_MARKER}" "${PDNS_POLICY_RC_D_PATH}" 2>/dev/null; then
    rm -f "${PDNS_POLICY_RC_D_PATH}"
  fi

  PDNS_POLICY_RC_D_INSTALLED="0"
}

trap cleanup_runtime_guards EXIT

package_manager_busy() {
  local lock

  if [ "${PKG_MGR}" = "apt" ]; then
    for lock in \
      /var/lib/dpkg/lock-frontend \
      /var/lib/dpkg/lock \
      /var/lib/apt/lists/lock \
      /var/cache/apt/archives/lock; do
      if [ -e "${lock}" ] && command -v fuser >/dev/null 2>&1; then
        if fuser "${lock}" >/dev/null 2>&1; then
          return 0
        fi
      fi
    done
  fi

  if [ "${PKG_MGR}" = "dnf" ]; then
    for lock in \
      /var/run/dnf.pid \
      /run/dnf.pid \
      /var/cache/dnf/metadata_lock.pid \
      /var/lib/dnf/rpmdb_lock.pid \
      /var/lib/rpm/.rpm.lock; do
      if [ -e "${lock}" ] && command -v fuser >/dev/null 2>&1; then
        if fuser "${lock}" >/dev/null 2>&1; then
          return 0
        fi
      fi
    done
  fi

  return 1
}

wait_for_package_manager() {
  local timeout step elapsed
  timeout="${AURAPANEL_PACKAGE_LOCK_TIMEOUT:-900}"
  step=5
  elapsed=0

  while package_manager_busy; do
    if [ "${elapsed}" -eq 0 ]; then
      warn "Package manager is busy, waiting for existing operations to finish..."
    fi

    if [ "${elapsed}" -ge "${timeout}" ]; then
      fail "Package manager remained busy for ${timeout}s. Re-run the installer after package activity completes."
    fi

    sleep "${step}"
    elapsed=$((elapsed + step))
  done
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
    wait_for_package_manager
    DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
  else
    wait_for_package_manager
    dnf install -y "$@"
  fi
}

package_available() {
  local pkg="$1"
  if [ -z "${pkg}" ]; then
    return 1
  fi

  if [ "${PKG_MGR}" = "apt" ]; then
    apt-cache show "${pkg}" >/dev/null 2>&1
  else
    dnf list --available "${pkg}" >/dev/null 2>&1 || rpm -q "${pkg}" >/dev/null 2>&1
  fi
}

install_optional_packages() {
  local pkg
  local missing=()

  for pkg in "$@"; do
    if ! package_available "${pkg}"; then
      missing+=("${pkg}")
      continue
    fi
    if ! install_packages "${pkg}"; then
      warn "Optional package '${pkg}' could not be installed."
    fi
  done

  if [ "${#missing[@]}" -gt 0 ]; then
    log "Optional packages unavailable in current repositories, skipped: ${missing[*]}"
  fi
}

configure_media_tool_preferences() {
  AURAPANEL_FFMPEG_INSTALL="$(normalize_yes_no "${AURAPANEL_FFMPEG_INSTALL}" "1")"
  AURAPANEL_FFMPEG_ACTIVE="$(normalize_yes_no "${AURAPANEL_FFMPEG_ACTIVE}" "${AURAPANEL_FFMPEG_INSTALL}")"
  AURAPANEL_IMAGEMAGICK_INSTALL="$(normalize_yes_no "${AURAPANEL_IMAGEMAGICK_INSTALL}" "1")"
  AURAPANEL_IMAGEMAGICK_ACTIVE="$(normalize_yes_no "${AURAPANEL_IMAGEMAGICK_ACTIVE}" "${AURAPANEL_IMAGEMAGICK_INSTALL}")"
  AURAPANEL_LIBREOFFICE_INSTALL="$(normalize_yes_no "${AURAPANEL_LIBREOFFICE_INSTALL}" "1")"
  AURAPANEL_LIBREOFFICE_ACTIVE="$(normalize_yes_no "${AURAPANEL_LIBREOFFICE_ACTIVE}" "${AURAPANEL_LIBREOFFICE_INSTALL}")"

  AURAPANEL_FFMPEG_INSTALL="$(prompt_yes_no "FFmpeg yüklensin mi?" "${AURAPANEL_FFMPEG_INSTALL}")"
  if [ "${AURAPANEL_FFMPEG_INSTALL}" = "1" ]; then
    AURAPANEL_FFMPEG_ACTIVE="$(prompt_yes_no "FFmpeg aktif doğrulansın mı?" "${AURAPANEL_FFMPEG_ACTIVE}")"
  else
    AURAPANEL_FFMPEG_ACTIVE="0"
  fi

  AURAPANEL_IMAGEMAGICK_INSTALL="$(prompt_yes_no "ImageMagick yüklensin mi?" "${AURAPANEL_IMAGEMAGICK_INSTALL}")"
  if [ "${AURAPANEL_IMAGEMAGICK_INSTALL}" = "1" ]; then
    AURAPANEL_IMAGEMAGICK_ACTIVE="$(prompt_yes_no "ImageMagick aktif doğrulansın mı?" "${AURAPANEL_IMAGEMAGICK_ACTIVE}")"
  else
    AURAPANEL_IMAGEMAGICK_ACTIVE="0"
  fi

  AURAPANEL_LIBREOFFICE_INSTALL="$(prompt_yes_no "LibreOffice yüklensin mi?" "${AURAPANEL_LIBREOFFICE_INSTALL}")"
  if [ "${AURAPANEL_LIBREOFFICE_INSTALL}" = "1" ]; then
    AURAPANEL_LIBREOFFICE_ACTIVE="$(prompt_yes_no "LibreOffice aktif doğrulansın mı?" "${AURAPANEL_LIBREOFFICE_ACTIVE}")"
  else
    AURAPANEL_LIBREOFFICE_ACTIVE="0"
  fi
}

ensure_magick_command() {
  if command -v magick >/dev/null 2>&1; then
    return 0
  fi

  if ! command -v convert >/dev/null 2>&1; then
    return 1
  fi

  cat <<'EOF' > /usr/local/bin/magick
#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -gt 0 ]; then
  subcmd="$1"
  shift
  case "${subcmd}" in
    convert|identify|mogrify|composite|compare|montage|import|display|animate|stream|conjure)
      if command -v "${subcmd}" >/dev/null 2>&1; then
        exec "${subcmd}" "$@"
      fi
      ;;
    *)
      set -- "${subcmd}" "$@"
      ;;
  esac
fi

if command -v convert >/dev/null 2>&1; then
  exec convert "$@"
fi

echo "ImageMagick command is unavailable." >&2
exit 127
EOF
  chmod 755 /usr/local/bin/magick
}

install_media_tools() {
  local -a im_packages=()

  if [ "${AURAPANEL_FFMPEG_INSTALL}" = "1" ]; then
    log "Installing FFmpeg..."
    install_optional_packages ffmpeg
    if [ "${AURAPANEL_FFMPEG_ACTIVE}" = "1" ]; then
      command -v ffmpeg >/dev/null 2>&1 || fail "FFmpeg install requested but ffmpeg binary not found."
      ffmpeg -version >/dev/null 2>&1 || fail "FFmpeg install requested but binary verification failed."
      ok "FFmpeg is installed and active."
    fi
  else
    warn "FFmpeg installation skipped by configuration."
  fi

  if [ "${AURAPANEL_IMAGEMAGICK_INSTALL}" = "1" ]; then
    log "Installing ImageMagick..."
    if [ "${PKG_MGR}" = "apt" ]; then
      im_packages=(imagemagick)
    else
      im_packages=(ImageMagick imagemagick)
    fi
    install_optional_packages "${im_packages[@]}"
    if [ "${AURAPANEL_IMAGEMAGICK_ACTIVE}" = "1" ]; then
      ensure_magick_command || fail "ImageMagick install requested but neither magick nor convert command is available."
      magick -version >/dev/null 2>&1 || fail "ImageMagick install requested but magick verification failed."
      ok "ImageMagick is installed and active."
    fi
  else
    warn "ImageMagick installation skipped by configuration."
  fi

  if [ "${AURAPANEL_LIBREOFFICE_INSTALL}" = "1" ]; then
    log "Installing LibreOffice..."
    install_optional_packages libreoffice
    if [ "${AURAPANEL_LIBREOFFICE_ACTIVE}" = "1" ]; then
      if command -v libreoffice >/dev/null 2>&1; then
        libreoffice --version >/dev/null 2>&1 || fail "LibreOffice install requested but libreoffice verification failed."
      elif command -v soffice >/dev/null 2>&1; then
        soffice --version >/dev/null 2>&1 || fail "LibreOffice install requested but soffice verification failed."
      else
        fail "LibreOffice install requested but libreoffice/soffice binary not found."
      fi
      ok "LibreOffice is installed and active."
    fi
  else
    warn "LibreOffice installation skipped by configuration."
  fi
}

systemd_unit_exists() {
  local unit="$1"
  local load_state=""
  if [ -z "${unit}" ]; then
    return 1
  fi

  if systemctl list-unit-files --full --all "${unit}" 2>/dev/null | awk '{print $1}' | grep -Fxq "${unit}"; then
    return 0
  fi

  load_state="$(systemctl show -p LoadState --value "${unit}" 2>/dev/null | tr -d '\r')"
  if [ -n "${load_state}" ] && [ "${load_state}" != "not-found" ]; then
    return 0
  fi

  systemctl cat "${unit}" >/dev/null 2>&1
}

install_pdns_policy_guard() {
  if [ "${PKG_MGR}" != "apt" ]; then
    return 0
  fi

  if [ -f "${PDNS_POLICY_RC_D_PATH}" ] && ! grep -q "${PDNS_POLICY_RC_D_MARKER}" "${PDNS_POLICY_RC_D_PATH}" 2>/dev/null; then
    cp "${PDNS_POLICY_RC_D_PATH}" "${PDNS_POLICY_RC_D_BACKUP}"
  fi

  cat <<EOF > "${PDNS_POLICY_RC_D_PATH}"
#!/usr/bin/env bash
# ${PDNS_POLICY_RC_D_MARKER}
if [ "\${1:-}" = "pdns" ] || [ "\${1:-}" = "pdns-server" ]; then
  exit 101
fi
exit 0
EOF
  chmod 755 "${PDNS_POLICY_RC_D_PATH}"
  PDNS_POLICY_RC_D_INSTALLED="1"
}

powerdns_primary_ip() {
  local public_ip=""

  public_ip="${AURAPANEL_PUBLIC_IP:-}"
  if [ -z "${public_ip}" ]; then
    public_ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i == "src") {print $(i+1); exit}}')"
  fi
  if [ -z "${public_ip}" ]; then
    public_ip="$(hostname -I 2>/dev/null | awk '{for (i=1; i<=NF; i++) if ($i !~ /^127\./) {print $i; exit}}')"
  fi

  printf '%s' "${public_ip}"
}

write_powerdns_runtime_config() {
  local db_path="$1"
  local public_ip="$2"
  local main_conf="/etc/powerdns/pdns.conf"
  local include_conf="/etc/powerdns/pdns.d/aurapanel-gsqlite3.conf"

  mkdir -p /etc/powerdns/pdns.d /var/lib/powerdns

  cat <<EOF > "${main_conf}"
# Managed by AuraPanel installer.
include-dir=/etc/powerdns/pdns.d
setuid=pdns
setgid=pdns
launch=gsqlite3
gsqlite3-database=${db_path}
local-port=53
local-address=${public_ip}
api=no
webserver=no
security-poll-suffix=
EOF

  cat <<EOF > "${include_conf}"
# Managed by AuraPanel installer.
launch=gsqlite3
gsqlite3-database=${db_path}
api=no
local-port=53
local-address=${public_ip}
EOF
}

setup_powerdns_repo() {
  if [ "${PKG_MGR}" != "apt" ]; then
    return 0
  fi

  if apt-cache show pdns-server >/dev/null 2>&1; then
    return 0
  fi

  local repo_domain repo_file repo_line keyring_path distro_channel codename
  codename="${VERSION_CODENAME:-}"
  if [ -z "${codename}" ] && command -v lsb_release >/dev/null 2>&1; then
    codename="$(lsb_release -cs 2>/dev/null || true)"
  fi
  if [ -z "${codename}" ]; then
    warn "Could not determine distro codename for PowerDNS repository setup."
    return 0
  fi

  case "${OS_ID}" in
    ubuntu)
      repo_domain="ubuntu"
      ;;
    debian)
      repo_domain="debian"
      ;;
    *)
      return 0
      ;;
  esac

  distro_channel="${codename}-${POWERDNS_REPO_CHANNEL}"
  repo_file="/etc/apt/sources.list.d/pdns.list"
  keyring_path="/etc/apt/keyrings/pdns-${POWERDNS_REPO_CHANNEL}.asc"
  repo_line="deb [signed-by=${keyring_path}] http://repo.powerdns.com/${repo_domain} ${distro_channel} main"

  log "Ensuring PowerDNS authoritative repository..."
  mkdir -p /etc/apt/keyrings
  download_file "${POWERDNS_REPO_KEY_URL}" "${keyring_path}" || {
    warn "PowerDNS repo key download failed."
    return 0
  }
  printf '%s\n' "${repo_line}" > "${repo_file}"
  wait_for_package_manager
  apt-get update -y >/dev/null 2>&1 || warn "apt-get update after PowerDNS repo setup failed."
}

configure_powerdns() {
  if ! command -v pdns_server >/dev/null 2>&1; then
    warn "PowerDNS server binary not found; DNS daemon bootstrap skipped."
    return 0
  fi

  local schema_file db_path public_ip
  db_path="/var/lib/powerdns/aurapanel.sqlite3"

  mkdir -p /etc/powerdns/pdns.d /var/lib/powerdns
  if [ ! -f "${db_path}" ]; then
    schema_file=""
    for candidate in \
      /usr/share/pdns-backend-sqlite3/schema/schema.sqlite3.sql \
      /usr/share/doc/pdns-backend-sqlite3/schema.sqlite3.sql \
      /usr/share/pdns-backend-sqlite3/schema.sqlite3.sql; do
      if [ -f "${candidate}" ]; then
        schema_file="${candidate}"
        break
      fi
    done
  if [ -n "${schema_file}" ] && command -v sqlite3 >/dev/null 2>&1; then
      sqlite3 "${db_path}" < "${schema_file}" >/dev/null 2>&1 || warn "PowerDNS SQLite schema bootstrap failed."
    else
      warn "PowerDNS SQLite schema file not found; service may require manual backend initialization."
    fi
  fi
  if id -u pdns >/dev/null 2>&1; then
    chown -R pdns:pdns /var/lib/powerdns >/dev/null 2>&1 || true
    chmod 660 "${db_path}" >/dev/null 2>&1 || true
  fi

  public_ip="$(powerdns_primary_ip)"
  if [ -z "${public_ip}" ]; then
    warn "Unable to determine a non-loopback IPv4 address for PowerDNS; DNS daemon bootstrap skipped."
    return 0
  fi

  rm -f /etc/powerdns/pdns.d/bind.conf /etc/powerdns/pdns.d/gsqlite3.conf >/dev/null 2>&1 || true
  write_powerdns_runtime_config "${db_path}" "${public_ip}"

  if systemctl list-unit-files | grep -q '^pdns\.service'; then
    systemctl enable pdns >/dev/null 2>&1 || true
    if systemctl restart pdns >/dev/null 2>&1; then
      ok "PowerDNS configured on ${public_ip}:53."
    else
      warn "PowerDNS restart failed after AuraPanel configuration."
      journalctl -u pdns -n 20 --no-pager >/dev/null 2>&1 || true
    fi
  fi
}

download_file() {
  local url="$1"
  local output="$2"
  local auth="${AURAPANEL_DOWNLOAD_AUTH:-}"
  local user=""
  local pass=""

  if command -v curl >/dev/null 2>&1; then
    if [ -n "${auth}" ]; then
      curl -fsSL -u "${auth}" "$url" -o "$output" || return 1
    else
      curl -fsSL "$url" -o "$output" || return 1
    fi
    return 0
  fi

  if command -v wget >/dev/null 2>&1; then
    if [ -n "${auth}" ] && [[ "${auth}" == *:* ]]; then
      user="${auth%%:*}"
      pass="${auth#*:}"
      wget -q --user="${user}" --password="${pass}" "$url" -O "$output" || return 1
    else
      wget -q "$url" -O "$output" || return 1
    fi
    return 0
  fi

  return 1
}

manifest_read_value() {
  local file="$1"
  local key="$2"
  if [ ! -f "${file}" ] || [ -z "${key}" ]; then
    return 1
  fi
  grep -E "^${key}=" "${file}" | head -n1 | cut -d'=' -f2- | tr -d '\r'
}

ols_manifest_key_prefix() {
  local arch codename major
  arch="$(uname -m)"
  if [ "${PKG_MGR}" = "apt" ]; then
    codename="${VERSION_CODENAME:-}"
    if [ -z "${codename}" ] && command -v lsb_release >/dev/null 2>&1; then
      codename="$(lsb_release -cs 2>/dev/null || true)"
    fi
    case "${arch}" in
      x86_64) arch="amd64" ;;
      aarch64) arch="arm64" ;;
    esac
    if [ -z "${codename}" ] || [ -z "${arch}" ]; then
      return 1
    fi
    codename="$(printf '%s' "${codename}" | tr '[:lower:]-' '[:upper:]_')"
    arch="$(printf '%s' "${arch}" | tr '[:lower:]-' '[:upper:]_')"
    printf 'AURAPANEL_OLS_DEB_%s_%s' "${codename}" "${arch}"
    return 0
  fi

  major="$(printf '%s' "${VERSION_ID:-}" | cut -d. -f1)"
  if [ -z "${major}" ]; then
    return 1
  fi
  arch="$(printf '%s' "${arch}" | tr '[:lower:]-' '[:upper:]_')"
  printf 'AURAPANEL_OLS_RPM_EL%s_%s' "${major}" "${arch}"
  return 0
}

install_openlitespeed_from_mirror_manifest() {
  local manifest tmp_pkg key_prefix pkg_url pkg_sha pkg_version computed_sha
  manifest="/tmp/aurapanel-openlitespeed-manifest.env"
  tmp_pkg="/tmp/aurapanel-openlitespeed.pkg"
  rm -f "${manifest}" "${tmp_pkg}" >/dev/null 2>&1 || true

  if ! download_file "${OLS_MANIFEST_URL}" "${manifest}"; then
    warn "Mirror OLS manifest could not be downloaded (${OLS_MANIFEST_URL})."
    return 1
  fi

  key_prefix="$(ols_manifest_key_prefix || true)"
  if [ -z "${key_prefix}" ]; then
    warn "Could not resolve OLS manifest key prefix for this OS."
    return 1
  fi

  pkg_url="$(manifest_read_value "${manifest}" "${key_prefix}_URL" || true)"
  pkg_sha="$(manifest_read_value "${manifest}" "${key_prefix}_SHA256" || true)"
  pkg_version="$(manifest_read_value "${manifest}" "${key_prefix}_VERSION" || true)"
  if [ -z "${pkg_url}" ]; then
    warn "No pinned OLS package URL found in manifest for ${key_prefix}."
    return 1
  fi

  log "Installing OpenLiteSpeed from mirror package (${pkg_version:-unknown})..."
  if ! download_file "${pkg_url}" "${tmp_pkg}"; then
    warn "Pinned OLS package download failed: ${pkg_url}"
    return 1
  fi

  if [ -n "${pkg_sha}" ]; then
    computed_sha="$(sha256sum "${tmp_pkg}" | awk '{print $1}')"
    if [ "${computed_sha}" != "${pkg_sha}" ]; then
      warn "Pinned OLS package checksum mismatch (expected ${pkg_sha}, got ${computed_sha})."
      return 1
    fi
  fi

  if [ "${PKG_MGR}" = "apt" ]; then
    wait_for_package_manager
    DEBIAN_FRONTEND=noninteractive apt-get install -y "${tmp_pkg}" || return 1
  else
    wait_for_package_manager
    dnf install -y "${tmp_pkg}" || return 1
  fi
  ok "OpenLiteSpeed installed from mirror package (${pkg_version:-unknown})."
  rm -f "${manifest}" "${tmp_pkg}" >/dev/null 2>&1 || true
  return 0
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

delete_env() {
  local file="$1"
  local key="$2"

  if [ -f "${file}" ]; then
    sed -i "/^${key}=/d" "${file}"
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

normalize_web_stack_mode() {
  local raw
  raw="$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
  case "${raw}" in
    "nginx-edge"|"nginx_edge"|"nginxedge")
      echo "nginx-edge"
      ;;
    "ols-only"|"ols_only"|"olsonly"|"")
      echo "ols-only"
      ;;
    *)
      echo ""
      ;;
  esac
}

web_stack_mode() {
  local requested existing normalized
  requested="$(normalize_web_stack_mode "${AURAPANEL_WEB_STACK_MODE:-}")"
  if [ -n "${requested}" ]; then
    echo "${requested}"
    return
  fi
  existing="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_WEB_STACK_MODE")"
  normalized="$(normalize_web_stack_mode "${existing}")"
  if [ -n "${normalized}" ]; then
    echo "${normalized}"
    return
  fi
  echo "ols-only"
}

generate_safe_password() {
  local length="${1:-24}"
  local generated=""

  generated="$(LC_ALL=C tr -dc 'A-Za-z0-9@#%+=._-' < /dev/urandom | head -c "${length}" || true)"
  if [ -z "${generated}" ]; then
    generated="$(openssl rand -hex 16 | tr -d '\n')"
  fi

  printf '%s' "${generated}"
}

panel_admin_email() {
  local admin_email
  admin_email="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_EMAIL")"
  admin_email="${admin_email:-admin@server.com}"
  printf '%s' "${admin_email}"
}

panel_admin_password() {
  local admin_pass
  admin_pass="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD")"

  if [ -z "${admin_pass}" ] && [ -f "${PROJECT_DIR}/logs/initial_password.txt" ]; then
    admin_pass="$(tr -d '\r\n' < "${PROJECT_DIR}/logs/initial_password.txt")"
  fi

  printf '%s' "${admin_pass}"
}

sync_panel_admin_credentials() {
  local admin_email admin_pass initial_password_file
  initial_password_file="${PROJECT_DIR}/logs/initial_password.txt"

  mkdir -p "${GATEWAY_ENV_DIR}" "${PROJECT_DIR}/logs"
  touch "${GATEWAY_ENV_FILE}"

  admin_email="$(panel_admin_email)"
  admin_pass="$(panel_admin_password)"

  if [ -z "${admin_pass}" ]; then
    admin_pass="$(generate_safe_password 24)"
    ok "Generated AuraPanel initial admin password."
  fi

  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_EMAIL" "${admin_email}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD" "${admin_pass}"
  delete_env "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD_BCRYPT"

  printf '%s\n' "${admin_pass}" > "${initial_password_file}"
  chmod 600 "${initial_password_file}"

  if [ -f "${SERVICE_ENV_FILE}" ]; then
    upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_ADMIN_EMAIL" "${admin_email}"
    upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD" "${admin_pass}"
    delete_env "${SERVICE_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD_BCRYPT"
    chmod 600 "${SERVICE_ENV_FILE}"
  fi

  chmod 600 "${GATEWAY_ENV_FILE}"
}

ols_admin_user() {
  local ols_user
  ols_user="$(read_env_value "${OLS_ADMIN_STATE_FILE}" "AURAPANEL_OLS_ADMIN_USER")"
  ols_user="${ols_user:-admin}"
  printf '%s' "${ols_user}"
}

ols_admin_password() {
  local ols_pass
  ols_pass="$(read_env_value "${OLS_ADMIN_STATE_FILE}" "AURAPANEL_OLS_ADMIN_PASSWORD")"
  printf '%s' "${ols_pass}"
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

detect_ssh_port() {
  local detected_port=""
  local config_file

  for config_file in /etc/ssh/sshd_config /etc/ssh/sshd_config.d/*.conf; do
    [ -f "${config_file}" ] || continue
    detected_port="$(awk '
      /^[[:space:]]*#/ {next}
      /^[[:space:]]*[Pp][Oo][Rr][Tt][[:space:]]+/ {
        print $2
        exit
      }
    ' "${config_file}" 2>/dev/null | tr -d '\r\n')"
    if [ -n "${detected_port}" ] && [[ "${detected_port}" =~ ^[0-9]+$ ]]; then
      echo "${detected_port}"
      return 0
    fi
  done

  echo "22"
}

harden_ssh_systemd_unit() {
  local ssh_unit=""
  local dropin_dir=""

  if systemd_unit_exists "ssh.service"; then
    ssh_unit="ssh.service"
  elif systemd_unit_exists "sshd.service"; then
    ssh_unit="sshd.service"
  else
    warn "No ssh.service/sshd.service unit found for KillMode hardening."
    return 0
  fi

  dropin_dir="/etc/systemd/system/${ssh_unit}.d"
  mkdir -p "${dropin_dir}"
  cat <<'EOF' > "${dropin_dir}/99-aurapanel-killmode.conf"
[Service]
KillMode=control-group
EOF
  systemctl daemon-reload >/dev/null 2>&1 || true
}

configure_standard_firewall() {
  local panel_port="$1"
  local ssh_port
  local touched="0"
  local ufw_active="0"
  local firewalld_active="0"
  local firewalld_changed="0"
  ssh_port="$(detect_ssh_port)"

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
    "${ssh_port}/tcp|SSH"
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

ensure_firewall_manager_active() {
  local ssh_port
  ssh_port="$(detect_ssh_port)"

  if [ "${PKG_MGR}" = "apt" ]; then
    if command -v ufw >/dev/null 2>&1; then
      if ! ufw status 2>/dev/null | grep -qi "Status: active"; then
        log "Activating ufw baseline policy..."
        ufw --force reset >/dev/null 2>&1 || true
        ufw default deny incoming >/dev/null 2>&1 || true
        ufw default allow outgoing >/dev/null 2>&1 || true
        ufw allow "${ssh_port}/tcp" >/dev/null 2>&1 || true
        if [ "${ssh_port}" != "22" ]; then
          ufw deny 22/tcp >/dev/null 2>&1 || true
        fi
        ufw --force enable >/dev/null 2>&1 || true
      fi
    fi
    return
  fi

  if command -v firewall-cmd >/dev/null 2>&1; then
    systemctl enable firewalld >/dev/null 2>&1 || true
    systemctl start firewalld >/dev/null 2>&1 || true
  fi
}

configure_pureftpd() {
  if ! command -v pure-pw >/dev/null 2>&1; then
    warn "pure-pw binary is missing. PureFTPd may not be installed on this distro."
    return
  fi

  log "Configuring PureFTPd defaults..."
  mkdir -p /etc/pure-ftpd/conf /etc/pure-ftpd/auth /etc/pure-ftpd /etc/ssl/private

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
  # Panel FTP users are mapped to www-data (uid 33) by default.
  # Keep MinUID low enough, otherwise Pure-FTPd rejects login with:
  # "account disabled (uid < ...)" / 530 errors.
  echo "33" > /etc/pure-ftpd/conf/MinUID

  # Ensure auth backends are actually enabled for Debian/Ubuntu layouts.
  ln -sfn ../conf/PureDB /etc/pure-ftpd/auth/60puredb
  ln -sfn ../conf/UnixAuthentication /etc/pure-ftpd/auth/65unix
  ln -sfn ../conf/PAMAuthentication /etc/pure-ftpd/auth/70pam

  touch /etc/pure-ftpd/pureftpd.passwd
  pure-pw mkdb /etc/pure-ftpd/pureftpd.pdb -f /etc/pure-ftpd/pureftpd.passwd >/dev/null 2>&1 || true

  systemctl daemon-reload >/dev/null 2>&1 || true
  if systemd_unit_exists "pure-ftpd.service"; then
    systemctl enable pure-ftpd >/dev/null 2>&1 || true
    systemctl restart pure-ftpd >/dev/null 2>&1 || true
    ok "PureFTPd service enabled and restarted."
  else
    warn "pure-ftpd systemd service not found. Check distro package naming."
  fi
}

configure_htaccess_watcher() {
  local watcher_service="/etc/systemd/system/aurapanel-htaccess-watcher.service"
  local watcher_script="/usr/local/bin/aurapanel-htaccess-watcher.sh"
  local fallback_script="/usr/local/bin/aurapanel-htaccess-fallback.sh"
  local fallback_cron="/etc/cron.d/aurapanel-htaccess-fallback"
  local state_dir="/var/lib/aurapanel"

  mkdir -p "${state_dir}" /usr/local/bin
  touch "${state_dir}/htaccess.last_reload" >/dev/null 2>&1 || true

  cat <<'EOF' > "${watcher_script}"
#!/usr/bin/env bash
set -euo pipefail

state_dir="/var/lib/aurapanel"
stamp_file="${state_dir}/htaccess.last_reload"
cooldown="${AURAPANEL_HTACCESS_RELOAD_COOLDOWN:-3}"

case "${cooldown}" in
  ''|*[!0-9]*)
    cooldown=3
    ;;
esac

mkdir -p "${state_dir}"
touch "${stamp_file}" >/dev/null 2>&1 || true

last_reload=0
inotifywait -m -r -e close_write,create,delete,move --format "%w%f" /home | while read -r changed; do
  case "${changed}" in
    */.htaccess)
      now="$(date +%s)"
      if [ $(( now - last_reload )) -lt "${cooldown}" ]; then
        continue
      fi
      if /usr/local/lsws/bin/lswsctrl reload >/dev/null 2>&1; then
        last_reload="${now}"
        touch "${stamp_file}" >/dev/null 2>&1 || true
      fi
      ;;
  esac
done
EOF
  chmod 750 "${watcher_script}" >/dev/null 2>&1 || true

  cat <<'EOF' > "${fallback_script}"
#!/usr/bin/env bash
set -euo pipefail

state_dir="/var/lib/aurapanel"
stamp_file="${state_dir}/htaccess.last_reload"
mkdir -p "${state_dir}"
touch "${stamp_file}" >/dev/null 2>&1 || true

if command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet aurapanel-htaccess-watcher 2>/dev/null; then
  exit 0
fi

latest_raw="$(find /home -type f -name '.htaccess' -printf '%T@\n' 2>/dev/null | sort -nr | head -n1 || true)"
if [ -z "${latest_raw}" ]; then
  exit 0
fi

latest_epoch="${latest_raw%%.*}"
if [ -z "${latest_epoch}" ]; then
  latest_epoch=0
fi
last_epoch="$(stat -c %Y "${stamp_file}" 2>/dev/null || echo 0)"

if [ "${latest_epoch}" -le "${last_epoch}" ]; then
  exit 0
fi

if /usr/local/lsws/bin/lswsctrl reload >/dev/null 2>&1; then
  touch "${stamp_file}" >/dev/null 2>&1 || true
fi
EOF
  chmod 750 "${fallback_script}" >/dev/null 2>&1 || true

  cat <<EOF > "${fallback_cron}"
*/5 * * * * root ${fallback_script} >/dev/null 2>&1
EOF
  chmod 644 "${fallback_cron}" >/dev/null 2>&1 || true

  install_optional_packages cron cronie
  if systemd_unit_exists "cron.service"; then
    systemctl enable cron >/dev/null 2>&1 || true
    systemctl restart cron >/dev/null 2>&1 || true
  elif systemd_unit_exists "crond.service"; then
    systemctl enable crond >/dev/null 2>&1 || true
    systemctl restart crond >/dev/null 2>&1 || true
  else
    warn "cron service not found. .htaccess fallback cron may not run."
  fi

  if ! command -v inotifywait >/dev/null 2>&1; then
    warn "inotifywait is missing. realtime .htaccess watcher skipped; cron fallback enabled."
    systemctl disable aurapanel-htaccess-watcher >/dev/null 2>&1 || true
    systemctl stop aurapanel-htaccess-watcher >/dev/null 2>&1 || true
    rm -f "${watcher_service}" >/dev/null 2>&1 || true
    systemctl daemon-reload >/dev/null 2>&1 || true
    ok ".htaccess cron fallback enabled."
    return 0
  fi

  cat <<EOF > "${watcher_service}"
[Unit]
Description=AuraPanel .htaccess watcher for OpenLiteSpeed
After=lshttpd.service
Requires=lshttpd.service

[Service]
Type=simple
ExecStart=${watcher_script}
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload >/dev/null 2>&1 || true
  systemctl enable aurapanel-htaccess-watcher >/dev/null 2>&1 || true
  systemctl restart aurapanel-htaccess-watcher >/dev/null 2>&1 || true
  ok ".htaccess watcher enabled (debounced) with cron fallback."
}

configure_roundcube() {
  if [ "${AURAPANEL_INSTALL_ROUNDCUBE:-1}" != "1" ]; then
    warn "Roundcube install skipped (AURAPANEL_INSTALL_ROUNDCUBE!=1)."
    return
  fi

  local webmail_dir="/usr/local/lsws/Example/html/webmail"
  local webmail_owner="nobody"
  local webmail_group="nobody"
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

  mkdir -p "${webmail_dir}/config" "${webmail_dir}/logs" "${webmail_dir}/temp"
  cat <<EOF > "${webmail_dir}/config/config.inc.php"
<?php
\$config['db_dsnw'] = 'mysql://${rc_db_user}:${rc_db_pass}@localhost/${rc_db_name}';
\$config['default_host'] = '127.0.0.1';
\$config['default_port'] = 993;
\$config['imap_conn_options'] = ['ssl' => ['verify_peer' => false, 'verify_peer_name' => false]];
\$config['smtp_server'] = 'ssl://127.0.0.1';
\$config['smtp_port'] = 465;
\$config['smtp_user'] = '%u';
\$config['smtp_pass'] = '%p';
\$config['smtp_conn_options'] = ['ssl' => ['verify_peer' => false, 'verify_peer_name' => false]];
\$config['product_name'] = 'AuraPanel Webmail';
\$config['des_key'] = '$(openssl rand -hex 16 | tr -d '\n')';
\$config['plugins'] = ['archive', 'zipdownload', 'markasjunk', 'aurapanel_sso'];
EOF

  mkdir -p "${webmail_dir}/plugins/aurapanel_sso"
  cat <<'EOF' > "${webmail_dir}/plugins/aurapanel_sso/aurapanel_sso.php"
<?php
class aurapanel_sso extends rcube_plugin {
    public $task = 'login';
    function init() {
        $this->add_hook('authenticate', array($this, 'authenticate'));
    }
    function authenticate($args) {
        if (!empty($_GET['_autologin_token'])) {
            $token = $_GET['_autologin_token'];
            $address = $_GET['_user'];
            
            $url = "http://127.0.0.1:8081/api/v1/mail/webmail/sso/verify?token=" . urlencode($token);
            $ch = curl_init($url);
            curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
            curl_setopt($ch, CURLOPT_TIMEOUT, 5);
            $response = curl_exec($ch);
            $http_code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
            curl_close($ch);
            
            if ($http_code === 200) {
                $data = json_decode($response, true);
                if (!empty($data['master_pass']) && strtolower($data['address']) === strtolower($address)) {
                    $args['user'] = $address . '*aurapanel-master';
                    $args['pass'] = $data['master_pass'];
                    $args['cookiecheck'] = false;
                    $args['valid'] = true;
                }
            }
        }
        return $args;
    }
}
EOF

  if id -gn nobody >/dev/null 2>&1; then
    webmail_group="$(id -gn nobody)"
  fi

  chown -R "${webmail_owner}:${webmail_group}" "${webmail_dir}" >/dev/null 2>&1 || true
  chmod 750 "${webmail_dir}/logs" "${webmail_dir}/temp" >/dev/null 2>&1 || true
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_MAIL_BACKEND" "vmail"

  rm -rf "${tmpdir}"
  ok "Roundcube setup completed at ${webmail_dir}"
}

configure_ols_webmail_route() {
  local example_vh="/usr/local/lsws/conf/vhosts/Example/vhconf.conf"

  if [ ! -f "${example_vh}" ]; then
    warn "Example vhost config not found. Roundcube route bootstrap skipped."
    return 0
  fi

  sed -i 's/indexFiles index.html/indexFiles index.php, index.html/' "${example_vh}" >/dev/null 2>&1 || true

  if ! grep -q '# AURAPANEL WEBMAIL BEGIN' "${example_vh}" 2>/dev/null; then
    cat <<'EOF' >> "${example_vh}"

# AURAPANEL WEBMAIL BEGIN
context /webmail/{
  allowBrowse 1
  location /usr/local/lsws/Example/html/webmail/
}
# AURAPANEL WEBMAIL END
EOF
  fi

  /usr/local/lsws/bin/lswsctrl restart >/dev/null 2>&1 || true
  ok "OpenLiteSpeed webmail route ensured."
}

configure_db_tools_hardening() {
  local hardening_script="${PROJECT_DIR}/installer/db-tools-hardening.sh"
  if [ ! -f "${hardening_script}" ]; then
    fail "DB tools hardening script not found: ${hardening_script}"
  fi

  chmod +x "${hardening_script}" >/dev/null 2>&1 || true
  if "${hardening_script}"; then
    ok "DB tools hardening policy applied."
  else
    fail "DB tools hardening script failed."
  fi
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

  touch /etc/dovecot/users /etc/dovecot/master-users /etc/postfix/vmailbox /etc/postfix/vmailbox_domains /etc/postfix/virtual /etc/postfix/virtual_regexp
  chmod 640 /etc/dovecot/users /etc/dovecot/master-users /etc/postfix/vmailbox /etc/postfix/vmailbox_domains /etc/postfix/virtual /etc/postfix/virtual_regexp >/dev/null 2>&1 || true
  if getent group dovecot >/dev/null 2>&1; then
    chgrp dovecot /etc/dovecot/users /etc/dovecot/master-users >/dev/null 2>&1 || true
  fi

  if [ ! -s /etc/dovecot/master-users ]; then
    local dovecot_master_pass
    dovecot_master_pass="$(generate_safe_password 24)"
    echo "aurapanel-master:{PLAIN}${dovecot_master_pass}" > /etc/dovecot/master-users
    upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_MAIL_MASTER_PASS" "${dovecot_master_pass}"
  fi

  if [ -f /etc/dovecot/conf.d/10-auth.conf ]; then
    sed -i 's/^!include auth-system\.conf\.ext/#!include auth-system.conf.ext/' /etc/dovecot/conf.d/10-auth.conf >/dev/null 2>&1 || true
    if ! grep -q 'auth-passwdfile\.conf\.ext' /etc/dovecot/conf.d/10-auth.conf 2>/dev/null; then
      printf '\n!include auth-passwdfile.conf.ext\n' >> /etc/dovecot/conf.d/10-auth.conf
    fi
  fi

  cat <<'EOF' >/etc/dovecot/conf.d/90-aurapanel-auth-socket.conf
service auth {
  unix_listener /var/spool/postfix/private/auth {
    mode = 0660
    user = postfix
    group = postfix
  }
}
EOF

  if command -v postmap >/dev/null 2>&1; then
    postmap /etc/postfix/vmailbox_domains >/dev/null 2>&1 || true
    postmap /etc/postfix/vmailbox >/dev/null 2>&1 || true
    postmap /etc/postfix/virtual >/dev/null 2>&1 || true
    chmod 644 /etc/postfix/vmailbox_domains.db /etc/postfix/vmailbox.db /etc/postfix/virtual.db >/dev/null 2>&1 || true
    if getent group postfix >/dev/null 2>&1; then
      chgrp postfix /etc/postfix/vmailbox_domains.db /etc/postfix/vmailbox.db /etc/postfix/virtual.db >/dev/null 2>&1 || true
    fi
  fi

  cat <<EOF >/etc/dovecot/conf.d/90-aurapanel-vmail.conf
auth_master_user_separator = *

passdb {
  driver = passwd-file
  args = /etc/dovecot/master-users
  master = yes
  pass = yes
}

passdb {
  driver = passwd-file
  args = scheme=SHA512-CRYPT username_format=%u /etc/dovecot/users
}

userdb {
  driver = static
  args = uid=${vmail_uid} gid=${vmail_gid} home=${vmail_base}/%d/%n allow_all_users=yes
}

mail_location = maildir:${vmail_base}/%d/%n/Maildir

plugin {
  quota = maildir:User quota
  quota_rule = *:storage=1024M
}
EOF

  if [ -f /etc/postfix/master.cf ]; then
    if ! grep -q '^submission inet' /etc/postfix/master.cf 2>/dev/null; then
      cat <<'EOF' >> /etc/postfix/master.cf

submission inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/submission
  -o smtpd_tls_security_level=encrypt
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING
EOF
    fi
    if ! grep -q '^smtps[[:space:]]\+inet' /etc/postfix/master.cf 2>/dev/null; then
      cat <<'EOF' >> /etc/postfix/master.cf

smtps     inet n       -       y       -       -       smtpd
  -o syslog_name=postfix/smtps
  -o smtpd_tls_wrappermode=yes
  -o smtpd_sasl_auth_enable=yes
  -o smtpd_recipient_restrictions=permit_sasl_authenticated,reject
  -o milter_macro_daemon_name=ORIGINATING
EOF
    fi
  fi

  if command -v postconf >/dev/null 2>&1; then
    postconf -e "virtual_mailbox_base = ${vmail_base}" >/dev/null 2>&1 || true
    postconf -e "virtual_mailbox_domains = hash:/etc/postfix/vmailbox_domains" >/dev/null 2>&1 || true
    postconf -e "virtual_mailbox_maps = hash:/etc/postfix/vmailbox" >/dev/null 2>&1 || true
    postconf -e "virtual_alias_maps = hash:/etc/postfix/virtual,regexp:/etc/postfix/virtual_regexp" >/dev/null 2>&1 || true
    postconf -e "virtual_minimum_uid = ${vmail_uid}" >/dev/null 2>&1 || true
    postconf -e "virtual_uid_maps = static:${vmail_uid}" >/dev/null 2>&1 || true
    postconf -e "virtual_gid_maps = static:${vmail_gid}" >/dev/null 2>&1 || true
    postconf -e "smtpd_sasl_type = dovecot" >/dev/null 2>&1 || true
    postconf -e "smtpd_sasl_path = private/auth" >/dev/null 2>&1 || true
    postconf -e "smtpd_sasl_auth_enable = yes" >/dev/null 2>&1 || true
    postconf -e "smtpd_tls_security_level = may" >/dev/null 2>&1 || true
    postconf -e "smtpd_tls_auth_only = no" >/dev/null 2>&1 || true
    postconf -e "smtpd_recipient_restrictions = permit_mynetworks,permit_sasl_authenticated,reject_unauth_destination" >/dev/null 2>&1 || true
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
  wait_for_package_manager
  curl -fsSL "${NODE_SETUP_URL}" | bash -
  install_packages nodejs
  ok "Node.js installed: $(node -v)"
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
  local installed_from_mirror="0"
  local use_pinned="${AURAPANEL_OLS_USE_MIRROR_PACKAGE:-1}"
  if [ -x /usr/local/lsws/bin/lswsctrl ]; then
    ok "OpenLiteSpeed already installed."
  else
    log "Installing OpenLiteSpeed..."
    case "$(printf '%s' "${use_pinned}" | tr '[:upper:]' '[:lower:]')" in
      1|true|yes|on)
      if install_openlitespeed_from_mirror_manifest; then
        installed_from_mirror="1"
      else
        warn "Pinned mirror package install failed, falling back to repository install."
      fi
      ;;
    esac
    if [ "${installed_from_mirror}" != "1" ]; then
      wait_for_package_manager
      curl -fsSL "${LITESPEED_REPO_SCRIPT_URL}" | bash
      install_packages openlitespeed || fail "OpenLiteSpeed installation failed."
    fi
  fi

  # Ensure lsphp toolchain is present even when OpenLiteSpeed was preinstalled.
  install_optional_packages lsphp83 lsphp83-common lsphp83-mysql lsphp83-curl lsphp83-xml lsphp83-zip lsphp83-opcache lsphp83-intl

  systemctl enable lshttpd >/dev/null 2>&1 || true
  systemctl restart lshttpd >/dev/null 2>&1 || true
}

ensure_wp_cli() {
  if ! command -v wp >/dev/null 2>&1; then
    log "Installing WP-CLI..."
    download_file "${WP_CLI_PHAR_URL}" "wp-cli.phar" || fail "WP-CLI download failed."
    chmod +x wp-cli.phar
    mv wp-cli.phar /usr/local/bin/wp
  else
    log "WP-CLI is already installed."
  fi
}

ensure_docker_runtime() {
  if command -v docker >/dev/null 2>&1; then
    ok "Docker runtime already available."
    return
  fi

  log "Installing Docker runtime (required for pgAdmin)..."
  if [ "${PKG_MGR}" = "apt" ]; then
    install_packages docker.io || fail "Docker installation failed."
  else
    install_packages docker || install_packages docker-ce || fail "Docker installation failed."
  fi

  command -v docker >/dev/null 2>&1 || fail "Docker binary missing after install."
  systemctl enable docker >/dev/null 2>&1 || true
  systemctl start docker >/dev/null 2>&1 || true
}

ensure_nginx_for_edge_mode() {
  local mode
  mode="$(web_stack_mode)"
  if [ "${mode}" != "nginx-edge" ]; then
    return
  fi

  if command -v nginx >/dev/null 2>&1; then
    ok "nginx already installed for edge mode."
    return
  fi

  log "Installing nginx for optional nginx-edge web stack mode..."
  if [ "${PKG_MGR}" = "apt" ]; then
    install_packages nginx libnginx-mod-stream || install_packages nginx
  elif [ "${PKG_MGR}" = "dnf" ]; then
    install_packages nginx
  else
    install_packages nginx
  fi

  command -v nginx >/dev/null 2>&1 || fail "nginx installation failed for nginx-edge mode."
}

ols_runtime_group() {
  if getent group nogroup >/dev/null 2>&1; then
    echo "nogroup"
    return
  fi
  if getent group nobody >/dev/null 2>&1; then
    echo "nobody"
    return
  fi
  echo "root"
}

ensure_ols_httpd_config_ownership() {
  local ols_conf="/usr/local/lsws/conf/httpd_config.conf"
  local runtime_group

  [ -f "${ols_conf}" ] || return 0
  runtime_group="$(ols_runtime_group)"
  chown root:"${runtime_group}" "${ols_conf}" >/dev/null 2>&1 || true
  chmod 640 "${ols_conf}" >/dev/null 2>&1 || true
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
  ensure_ols_httpd_config_ownership

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
  log "Ensuring lsphp83 database drivers (mysql, pgsql, sqlite3) and intl..."
  install_optional_packages lsphp83-mysql lsphp83-pgsql lsphp83-sqlite3 lsphp83-intl

  local intl_ini="/usr/local/lsws/lsphp83/etc/php/8.3/mods-available/intl.ini"
  mkdir -p "$(dirname "${intl_ini}")"
  if [ ! -f "${intl_ini}" ]; then
    cat <<'EOF' > "${intl_ini}"
extension=intl.so
EOF
  fi
  chmod 644 "${intl_ini}" >/dev/null 2>&1 || true

  local ext_dir
  ext_dir="$({ /usr/local/lsws/lsphp83/bin/lsphp -i 2>/dev/null | awk -F'=> ' '/^extension_dir =>/{print $2; exit}' | awk '{print $1}'; } || true)"
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

    if [ ! -f "${ext_dir}/intl.so" ]; then
      warn "lsphp83 intl.so is missing from extension_dir (${ext_dir})."
    else
      ok "lsphp83 intl extension binary detected."
    fi
  else
    warn "Could not detect lsphp83 extension_dir for PDO driver verification."
  fi
}

ensure_ols_modsecurity() {
  if [ -f /usr/local/lsws/modules/mod_security.so ]; then
    ok "OpenLiteSpeed ModSecurity module already present."
    return
  fi

  log "Installing OpenLiteSpeed ModSecurity module..."
  install_packages ols-modsecurity || fail "ols-modsecurity installation failed."

  if [ ! -f /usr/local/lsws/modules/mod_security.so ]; then
    fail "mod_security.so is missing after ols-modsecurity install."
  fi

  ok "OpenLiteSpeed ModSecurity module installed."
}

configure_ols_modsecurity_crs() {
  local owasp_root="/usr/local/lsws/conf/owasp"
  local owasp_dir="${owasp_root}/owasp-modsecurity-crs"
  local archive_path="/tmp/aurapanel-owasp-crs.zip"

  log "Configuring OWASP CRS for OpenLiteSpeed..."
  mkdir -p "${owasp_root}" /tmp/modsecurity /usr/local/lsws/logs

  if [ ! -d "${owasp_dir}" ]; then
    rm -f "${archive_path}"
    download_file "${OWASP_CRS_ARCHIVE_URL}" "${archive_path}" || fail "OWASP CRS archive download failed."
    unzip -qq -o "${archive_path}" -d "${owasp_root}" || fail "OWASP CRS archive extract failed."
    rm -rf "${owasp_dir}"
    mv "${owasp_root}"/coreruleset-* "${owasp_dir}" || fail "OWASP CRS directory rename failed."
  fi

  if [ -f "${owasp_dir}/crs-setup.conf.example" ] && [ ! -f "${owasp_dir}/crs-setup.conf" ]; then
    cp "${owasp_dir}/crs-setup.conf.example" "${owasp_dir}/crs-setup.conf"
  fi
  if [ -f "${owasp_dir}/rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf.example" ] && [ ! -f "${owasp_dir}/rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf" ]; then
    cp "${owasp_dir}/rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf.example" "${owasp_dir}/rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf"
  fi
  if [ -f "${owasp_dir}/rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf.example" ] && [ ! -f "${owasp_dir}/rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf" ]; then
    cp "${owasp_dir}/rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf.example" "${owasp_dir}/rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf"
  fi

  cat <<'EOF' > "${owasp_root}/modsecurity.conf"
SecRuleEngine On
SecRequestBodyAccess On
SecResponseBodyAccess Off
SecAuditEngine RelevantOnly
SecAuditLog /usr/local/lsws/logs/modsec_audit.log
SecDebugLog /usr/local/lsws/logs/modsec_debug.log
SecDebugLogLevel 0
SecTmpDir /tmp
SecDataDir /tmp/modsecurity
EOF

  cat <<'EOF' > "${owasp_root}/modsec_includes.conf"
include modsecurity.conf
include owasp-modsecurity-crs/crs-setup.conf
include owasp-modsecurity-crs/rules/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf
include owasp-modsecurity-crs/rules/REQUEST-901-INITIALIZATION.conf
include owasp-modsecurity-crs/rules/REQUEST-905-COMMON-EXCEPTIONS.conf
include owasp-modsecurity-crs/rules/REQUEST-911-METHOD-ENFORCEMENT.conf
include owasp-modsecurity-crs/rules/REQUEST-913-SCANNER-DETECTION.conf
include owasp-modsecurity-crs/rules/REQUEST-920-PROTOCOL-ENFORCEMENT.conf
include owasp-modsecurity-crs/rules/REQUEST-921-PROTOCOL-ATTACK.conf
include owasp-modsecurity-crs/rules/REQUEST-922-MULTIPART-ATTACK.conf
include owasp-modsecurity-crs/rules/REQUEST-930-APPLICATION-ATTACK-LFI.conf
include owasp-modsecurity-crs/rules/REQUEST-931-APPLICATION-ATTACK-RFI.conf
include owasp-modsecurity-crs/rules/REQUEST-932-APPLICATION-ATTACK-RCE.conf
include owasp-modsecurity-crs/rules/REQUEST-933-APPLICATION-ATTACK-PHP.conf
include owasp-modsecurity-crs/rules/REQUEST-934-APPLICATION-ATTACK-GENERIC.conf
include owasp-modsecurity-crs/rules/REQUEST-941-APPLICATION-ATTACK-XSS.conf
include owasp-modsecurity-crs/rules/REQUEST-942-APPLICATION-ATTACK-SQLI.conf
include owasp-modsecurity-crs/rules/REQUEST-943-APPLICATION-ATTACK-SESSION-FIXATION.conf
include owasp-modsecurity-crs/rules/REQUEST-944-APPLICATION-ATTACK-JAVA.conf
include owasp-modsecurity-crs/rules/REQUEST-949-BLOCKING-EVALUATION.conf
include owasp-modsecurity-crs/rules/RESPONSE-950-DATA-LEAKAGES.conf
include owasp-modsecurity-crs/rules/RESPONSE-951-DATA-LEAKAGES-SQL.conf
include owasp-modsecurity-crs/rules/RESPONSE-952-DATA-LEAKAGES-JAVA.conf
include owasp-modsecurity-crs/rules/RESPONSE-953-DATA-LEAKAGES-PHP.conf
include owasp-modsecurity-crs/rules/RESPONSE-954-DATA-LEAKAGES-IIS.conf
include owasp-modsecurity-crs/rules/RESPONSE-955-WEB-SHELLS.conf
include owasp-modsecurity-crs/rules/RESPONSE-959-BLOCKING-EVALUATION.conf
include owasp-modsecurity-crs/rules/RESPONSE-980-CORRELATION.conf
include owasp-modsecurity-crs/rules/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf
EOF

  chmod 644 "${owasp_root}/modsecurity.conf" "${owasp_root}/modsec_includes.conf"
  chmod -R 755 "${owasp_dir}" >/dev/null 2>&1 || true
}

enable_ols_modsecurity() {
  local ols_conf="/usr/local/lsws/conf/httpd_config.conf"
  local tmp_conf

  if [ ! -f "${ols_conf}" ]; then
    warn "OpenLiteSpeed config not found: ${ols_conf}"
    return 0
  fi

  tmp_conf="$(mktemp /tmp/aurapanel-modsec-httpd.XXXXXX)"
  awk '
    $0=="# AURAPANEL MODSEC BEGIN" {skip=1; next}
    $0=="# AURAPANEL MODSEC END" {skip=0; next}
    !skip {print}
  ' "${ols_conf}" > "${tmp_conf}"

  cat <<'EOF' >> "${tmp_conf}"

# AURAPANEL MODSEC BEGIN
module mod_security {
    modsecurity  on
    modsecurity_rules `
    SecRuleEngine On
    `
    modsecurity_rules_file         /usr/local/lsws/conf/owasp/modsec_includes.conf
    ls_enabled              1
}
# AURAPANEL MODSEC END
EOF

  install -m 640 "${tmp_conf}" "${ols_conf}"
  rm -f "${tmp_conf}"
  ensure_ols_httpd_config_ownership

  if ! /usr/local/lsws/bin/lswsctrl restart >/dev/null 2>&1; then
    fail "OpenLiteSpeed restart failed after ModSecurity enable."
  fi

  ok "OpenLiteSpeed ModSecurity + OWASP CRS enabled."
}

ensure_ioncube_loader() {
  if [ "${AURAPANEL_INSTALL_IONCUBE:-1}" != "1" ]; then
    warn "ionCube loader install skipped (AURAPANEL_INSTALL_IONCUBE!=1)."
    return
  fi

  local lsphp_bin="/usr/local/lsws/lsphp83/bin/lsphp"
  local ioncube_url="${IONCUBE_ARCHIVE_URL}"
  local ext_dir ini_file loaded_ini scan_dir tmpdir archive src_loader dst_loader

  lsphp_ioncube_active() {
    "${lsphp_bin}" -v 2>/dev/null | grep -qi "ionCube PHP Loader" && return 0
    "${lsphp_bin}" -m 2>/dev/null | grep -qi '^ionCube Loader$' && return 0
    "${lsphp_bin}" -i 2>/dev/null | grep -qi "ionCube PHP Loader" && return 0
    return 1
  }

  if [ ! -x "${lsphp_bin}" ]; then
    warn "lsphp83 binary not found; ionCube install skipped."
    return
  fi

  if lsphp_ioncube_active; then
    ok "ionCube loader already active for lsphp83."
    return
  fi

  ext_dir="$({ "${lsphp_bin}" -i 2>/dev/null | awk -F'=> ' '/^extension_dir =>/{print $2; exit}' | awk '{print $1}'; } || true)"
  if [ -z "${ext_dir}" ]; then
    warn "Unable to detect lsphp83 extension_dir; ionCube install skipped."
    return
  fi

  ini_file="/usr/local/lsws/lsphp83/etc/php/8.3/mods-available/00-ioncube.ini"
  loaded_ini="$({ "${lsphp_bin}" -i 2>/dev/null | awk -F'=> ' '/^Loaded Configuration File =>/{print $2; exit}' | awk '{print $1}'; } || true)"
  scan_dir="$({ "${lsphp_bin}" -i 2>/dev/null | awk -F'=> ' '/^Scan this dir for additional \.ini files =>/{print $2; exit}' | awk '{print $1}'; } || true)"
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

  if [ -n "${scan_dir}" ] && [ "${scan_dir}" != "(none)" ]; then
    mkdir -p "${scan_dir}"
    cat <<EOF > "${scan_dir}/00-ioncube.ini"
; ionCube Loader
zend_extension=${dst_loader}
EOF
    chmod 644 "${scan_dir}/00-ioncube.ini"
  elif [ -n "${loaded_ini}" ] && [ "${loaded_ini}" != "(none)" ] && [ -f "${loaded_ini}" ]; then
    if ! grep -q "ioncube_loader_lin_8\.3\.so" "${loaded_ini}" 2>/dev/null; then
      printf '\n; AuraPanel ionCube Loader\nzend_extension=%s\n' "${dst_loader}" >> "${loaded_ini}"
    fi
  fi

  if systemctl restart lshttpd >/dev/null 2>&1; then
    if lsphp_ioncube_active; then
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
    ols_pass="$(generate_safe_password 22)"
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
  panel_user="$(panel_admin_email)"
  panel_pass="$(panel_admin_password)"
  ols_user="$(ols_admin_user)"
  ols_pass="$(ols_admin_password)"

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

write_service_env_defaults() {
  mkdir -p "${GATEWAY_ENV_DIR}" "${PROJECT_DIR}/logs"
  chmod 700 "${GATEWAY_ENV_DIR}"
  local shared_jwt_secret=""
  local shared_proxy_token=""
  local web_stack_mode_value web_stack_edge_enabled web_stack_backend_addr
  local legacy_gateway_core_url_key="AURAPANEL_""CORE_URL"
  local legacy_service_bind_key="AURAPANEL_""CORE_BIND_ADDR"

  web_stack_mode_value="$(web_stack_mode)"
  if [ "${web_stack_mode_value}" = "nginx-edge" ]; then
    web_stack_edge_enabled="1"
    web_stack_backend_addr="127.0.0.1:8088"
  else
    web_stack_edge_enabled="0"
    web_stack_backend_addr="*:80"
  fi

  if [ ! -f "${GATEWAY_ENV_FILE}" ]; then
    local admin_pass jwt_secret proxy_token
    admin_pass="$(generate_safe_password 24)"
    jwt_secret="$(openssl rand -hex 32 | tr -d '\n')"
    proxy_token="$(openssl rand -hex 32 | tr -d '\n')"
    shared_jwt_secret="${jwt_secret}"
    shared_proxy_token="${proxy_token}"

    cat <<EOF > "${GATEWAY_ENV_FILE}"
AURAPANEL_ADMIN_EMAIL=admin@server.com
AURAPANEL_ADMIN_PASSWORD=${admin_pass}
AURAPANEL_JWT_SECRET=${jwt_secret}
AURAPANEL_JWT_ISSUER=aurapanel-gateway
AURAPANEL_JWT_AUDIENCE=aurapanel-ui
AURAPANEL_ALLOWED_ORIGINS=http://127.0.0.1:${PANEL_PORT_DEFAULT},http://localhost:${PANEL_PORT_DEFAULT}
AURAPANEL_SERVICE_URL=http://127.0.0.1:8081
AURAPANEL_GATEWAY_ONLY=1
AURAPANEL_INTERNAL_PROXY_TOKEN=${proxy_token}
AURAPANEL_GATEWAY_ADDR=:${PANEL_PORT_DEFAULT}
AURAPANEL_PANEL_DIST=/opt/aurapanel/frontend/dist
AURAPANEL_DDOS_ENABLED=0
AURAPANEL_DDOS_PROFILE=off
AURAPANEL_DDOS_GLOBAL_RPS=120
AURAPANEL_DDOS_GLOBAL_BURST=240
AURAPANEL_DDOS_AUTH_RPS=20
AURAPANEL_DDOS_AUTH_BURST=40
EOF

    chmod 600 "${GATEWAY_ENV_FILE}"
  fi

  if [ -z "${shared_jwt_secret}" ]; then
    shared_jwt_secret="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_JWT_SECRET")"
  fi
  if [ -z "${shared_proxy_token}" ]; then
    shared_proxy_token="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_INTERNAL_PROXY_TOKEN")"
  fi
  if [ -z "${shared_proxy_token}" ]; then
    shared_proxy_token="$(openssl rand -hex 32 | tr -d '\n')"
  fi

  if [ ! -f "${SERVICE_ENV_FILE}" ]; then
    local restic_pass minio_access minio_secret
    restic_pass="$(openssl rand -hex 24 | tr -d '\n')"
    minio_access="backup$(openssl rand -hex 3 | tr -d '\n')"
    minio_secret="$(openssl rand -hex 24 | tr -d '\n')"

    cat <<EOF > "${SERVICE_ENV_FILE}"
AURAPANEL_ADMIN_EMAIL=$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_EMAIL")
AURAPANEL_ADMIN_PASSWORD=$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD")
AURAPANEL_RUNTIME_MODE=production
AURAPANEL_SECURITY_POLICY=fail-closed
AURAPANEL_GATEWAY_ONLY=1
AURAPANEL_JWT_SECRET=${shared_jwt_secret}
AURAPANEL_INTERNAL_PROXY_TOKEN=${shared_proxy_token}
AURAPANEL_SERVICE_ADDR=127.0.0.1:8081
AURAPANEL_ALLOW_REMOTE_SERVICE=0
AURAPANEL_TERMINAL_ENABLED=0
AURAPANEL_FEDERATION_MODE=active-passive
AURAPANEL_FEDERATION_PRIMARY=1
AURAPANEL_BACKUP_TARGET=internal-minio
AURAPANEL_BACKUP_MINIO_ENDPOINT=http://127.0.0.1:9000
AURAPANEL_BACKUP_MINIO_BUCKET=aurapanel-backups
AURAPANEL_BACKUP_MINIO_ACCESS_KEY=${minio_access}
AURAPANEL_BACKUP_MINIO_SECRET_KEY=${minio_secret}
AURAPANEL_BACKUP_RESTIC_PASSWORD=${restic_pass}
AURAPANEL_MAIL_BACKEND=vmail
AURAPANEL_MAIL_VMAIL_UID=5000
AURAPANEL_MAIL_VMAIL_GID=5000
AURAPANEL_MAIL_VMAIL_BASE=/var/mail/vhosts
AURAPANEL_FFMPEG_INSTALL=${AURAPANEL_FFMPEG_INSTALL}
AURAPANEL_FFMPEG_ACTIVE=${AURAPANEL_FFMPEG_ACTIVE}
AURAPANEL_IMAGEMAGICK_INSTALL=${AURAPANEL_IMAGEMAGICK_INSTALL}
AURAPANEL_IMAGEMAGICK_ACTIVE=${AURAPANEL_IMAGEMAGICK_ACTIVE}
AURAPANEL_LIBREOFFICE_INSTALL=${AURAPANEL_LIBREOFFICE_INSTALL}
AURAPANEL_LIBREOFFICE_ACTIVE=${AURAPANEL_LIBREOFFICE_ACTIVE}
AURAPANEL_DBTOOLS_AUTH_USER=dbtools
AURAPANEL_DBTOOLS_AUTH_PASS=
AURAPANEL_DBTOOLS_ALLOWED_IPS=
AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN=120
AURAPANEL_DBTOOLS_RUNTIME_ALLOWLIST_FILE=/etc/aurapanel/db-tools/runtime-allowlist.txt
AURAPANEL_DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE=0
AURAPANEL_STATE_BACKEND=mariadb
AURAPANEL_STATE_DB_NAME=aurapanel
AURAPANEL_STATE_DB_USER=root
AURAPANEL_STATE_DB_PASSWORD=
AURAPANEL_STATE_DB_SOCKET=/run/mysqld/mysqld.sock
AURAPANEL_WEB_STACK_MODE=${web_stack_mode_value}
AURAPANEL_NGINX_EDGE_ENABLED=${web_stack_edge_enabled}
AURAPANEL_OLS_BACKEND_ADDR=${web_stack_backend_addr}
AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN=0
AURAPANEL_PHPMYADMIN_BASE_URL=/phpmyadmin/index.php
AURAPANEL_PGADMIN_BASE_URL=/pgadmin4/
AURAPANEL_CLOUDFLARE_EMAIL=
AURAPANEL_CLOUDFLARE_API_KEY=
AURAPANEL_CLOUDFLARE_API_TOKEN=
AURAPANEL_CLOUDFLARE_AUTO_SYNC=0
AURAPANEL_DDOS_ENABLED=0
AURAPANEL_DDOS_PROFILE=off
AURAPANEL_DDOS_GLOBAL_RPS=120
AURAPANEL_DDOS_GLOBAL_BURST=240
AURAPANEL_DDOS_AUTH_RPS=20
AURAPANEL_DDOS_AUTH_BURST=40
EOF

    chmod 600 "${SERVICE_ENV_FILE}"
  fi

  sync_panel_admin_credentials

  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_GATEWAY_ONLY" "1"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_SERVICE_URL" "http://127.0.0.1:8081"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_GATEWAY_ADDR" ":${PANEL_PORT_DEFAULT}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_PANEL_DIST" "${PROJECT_DIR}/frontend/dist"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_ALLOWED_ORIGINS" "http://127.0.0.1:${PANEL_PORT_DEFAULT},http://localhost:${PANEL_PORT_DEFAULT}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_INTERNAL_PROXY_TOKEN" "${shared_proxy_token}"
  local gateway_ddos_enabled gateway_ddos_profile gateway_ddos_global_rps gateway_ddos_global_burst gateway_ddos_auth_rps gateway_ddos_auth_burst
  gateway_ddos_enabled="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_ENABLED")"
  gateway_ddos_profile="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_PROFILE")"
  gateway_ddos_global_rps="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_RPS")"
  gateway_ddos_global_burst="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_BURST")"
  gateway_ddos_auth_rps="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_AUTH_RPS")"
  gateway_ddos_auth_burst="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_AUTH_BURST")"
  [ -n "${gateway_ddos_enabled}" ] || gateway_ddos_enabled="0"
  [ -n "${gateway_ddos_profile}" ] || gateway_ddos_profile="off"
  [ -n "${gateway_ddos_global_rps}" ] || gateway_ddos_global_rps="120"
  [ -n "${gateway_ddos_global_burst}" ] || gateway_ddos_global_burst="240"
  [ -n "${gateway_ddos_auth_rps}" ] || gateway_ddos_auth_rps="20"
  [ -n "${gateway_ddos_auth_burst}" ] || gateway_ddos_auth_burst="40"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_ENABLED" "${gateway_ddos_enabled}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_PROFILE" "${gateway_ddos_profile}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_RPS" "${gateway_ddos_global_rps}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_BURST" "${gateway_ddos_global_burst}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_AUTH_RPS" "${gateway_ddos_auth_rps}"
  upsert_env "${GATEWAY_ENV_FILE}" "AURAPANEL_DDOS_AUTH_BURST" "${gateway_ddos_auth_burst}"
  delete_env "${GATEWAY_ENV_FILE}" "${legacy_gateway_core_url_key}"

  local shared_admin_email shared_admin_password shared_admin_hash
  shared_admin_email="$(panel_admin_email)"
  shared_admin_password="$(panel_admin_password)"
  shared_admin_hash="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD_BCRYPT")"

  if [ -n "${shared_admin_email}" ]; then
    upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_ADMIN_EMAIL" "${shared_admin_email}"
  fi
  if [ -n "${shared_admin_password}" ]; then
    upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD" "${shared_admin_password}"
  fi
  if [ -n "${shared_admin_hash}" ]; then
    upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD_BCRYPT" "${shared_admin_hash}"
  else
    delete_env "${SERVICE_ENV_FILE}" "AURAPANEL_ADMIN_PASSWORD_BCRYPT"
  fi

  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_RUNTIME_MODE" "production"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_SECURITY_POLICY" "fail-closed"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_GATEWAY_ONLY" "1"
  if [ -n "${shared_jwt_secret}" ]; then
    upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_JWT_SECRET" "${shared_jwt_secret}"
  fi
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_INTERNAL_PROXY_TOKEN" "${shared_proxy_token}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_SERVICE_ADDR" "127.0.0.1:8081"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_ALLOW_REMOTE_SERVICE" "0"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_TERMINAL_ENABLED" "0"
  delete_env "${SERVICE_ENV_FILE}" "${legacy_service_bind_key}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_FEDERATION_MODE" "active-passive"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_FEDERATION_PRIMARY" "1"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_BACKUP_TARGET" "internal-minio"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_BACKUP_MINIO_ENDPOINT" "http://127.0.0.1:9000"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_BACKUP_MINIO_BUCKET" "aurapanel-backups"
  local dbtools_auth_user dbtools_auth_pass dbtools_allowed_ips dbtools_rate dbtools_runtime_file dbtools_reload panel_edge_single_domain pma_base pg_base ddos_enabled ddos_profile ddos_global_rps ddos_global_burst ddos_auth_rps ddos_auth_burst state_backend state_db_name state_db_user state_db_password state_db_socket web_stack_mode_current nginx_edge_enabled ols_backend_addr
  dbtools_auth_user="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER")"
  dbtools_auth_pass="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS")"
  dbtools_allowed_ips="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS")"
  dbtools_rate="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN")"
  dbtools_runtime_file="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RUNTIME_ALLOWLIST_FILE")"
  dbtools_reload="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE")"
  panel_edge_single_domain="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN")"
  pma_base="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PHPMYADMIN_BASE_URL")"
  pg_base="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_BASE_URL")"
  ddos_enabled="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_ENABLED")"
  ddos_profile="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_PROFILE")"
  ddos_global_rps="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_RPS")"
  ddos_global_burst="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_BURST")"
  ddos_auth_rps="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_AUTH_RPS")"
  ddos_auth_burst="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_AUTH_BURST")"
  state_backend="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_BACKEND")"
  state_db_name="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_NAME")"
  state_db_user="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_USER")"
  state_db_password="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_PASSWORD")"
  state_db_socket="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_SOCKET")"
  web_stack_mode_current="$(normalize_web_stack_mode "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_WEB_STACK_MODE")")"
  nginx_edge_enabled="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_NGINX_EDGE_ENABLED")"
  ols_backend_addr="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_OLS_BACKEND_ADDR")"
  [ -n "${dbtools_auth_user}" ] || dbtools_auth_user="dbtools"
  [ -n "${dbtools_rate}" ] || dbtools_rate="120"
  [ -n "${dbtools_runtime_file}" ] || dbtools_runtime_file="/etc/aurapanel/db-tools/runtime-allowlist.txt"
  [ -n "${dbtools_reload}" ] || dbtools_reload="0"
  [ -n "${panel_edge_single_domain}" ] || panel_edge_single_domain="0"
  [ -n "${pma_base}" ] || pma_base="/phpmyadmin/index.php"
  [ -n "${pg_base}" ] || pg_base="/pgadmin4/"
  [ -n "${ddos_enabled}" ] || ddos_enabled="0"
  [ -n "${ddos_profile}" ] || ddos_profile="off"
  [ -n "${ddos_global_rps}" ] || ddos_global_rps="120"
  [ -n "${ddos_global_burst}" ] || ddos_global_burst="240"
  [ -n "${ddos_auth_rps}" ] || ddos_auth_rps="20"
  [ -n "${ddos_auth_burst}" ] || ddos_auth_burst="40"
  [ -n "${state_backend}" ] || state_backend="mariadb"
  [ -n "${state_db_name}" ] || state_db_name="aurapanel"
  [ -n "${state_db_user}" ] || state_db_user="root"
  [ -n "${state_db_socket}" ] || state_db_socket="/run/mysqld/mysqld.sock"
  [ -n "${web_stack_mode_current}" ] || web_stack_mode_current="${web_stack_mode_value}"
  if [ "${web_stack_mode_current}" = "nginx-edge" ]; then
    [ -n "${nginx_edge_enabled}" ] || nginx_edge_enabled="1"
    [ -n "${ols_backend_addr}" ] || ols_backend_addr="127.0.0.1:8088"
  else
    [ -n "${nginx_edge_enabled}" ] || nginx_edge_enabled="0"
    [ -n "${ols_backend_addr}" ] || ols_backend_addr="*:80"
  fi

  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_MAIL_BACKEND" "vmail"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_MAIL_VMAIL_UID" "5000"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_MAIL_VMAIL_GID" "5000"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_MAIL_VMAIL_BASE" "/var/mail/vhosts"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_FFMPEG_INSTALL" "$(normalize_yes_no "${AURAPANEL_FFMPEG_INSTALL}" "1")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_FFMPEG_ACTIVE" "$(normalize_yes_no "${AURAPANEL_FFMPEG_ACTIVE}" "1")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_IMAGEMAGICK_INSTALL" "$(normalize_yes_no "${AURAPANEL_IMAGEMAGICK_INSTALL}" "1")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_IMAGEMAGICK_ACTIVE" "$(normalize_yes_no "${AURAPANEL_IMAGEMAGICK_ACTIVE}" "1")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_LIBREOFFICE_INSTALL" "$(normalize_yes_no "${AURAPANEL_LIBREOFFICE_INSTALL}" "1")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_LIBREOFFICE_ACTIVE" "$(normalize_yes_no "${AURAPANEL_LIBREOFFICE_ACTIVE}" "1")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER" "${dbtools_auth_user}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS" "${dbtools_auth_pass}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS" "${dbtools_allowed_ips}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN" "${dbtools_rate}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RUNTIME_ALLOWLIST_FILE" "${dbtools_runtime_file}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE" "${dbtools_reload}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN" "${panel_edge_single_domain}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PHPMYADMIN_BASE_URL" "${pma_base}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_BASE_URL" "${pg_base}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_EMAIL" "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_EMAIL")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_API_KEY" "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_API_KEY")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_API_TOKEN" "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_API_TOKEN")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_AUTO_SYNC" "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_CLOUDFLARE_AUTO_SYNC")"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_ENABLED" "${ddos_enabled}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_PROFILE" "${ddos_profile}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_RPS" "${ddos_global_rps}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_GLOBAL_BURST" "${ddos_global_burst}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_AUTH_RPS" "${ddos_auth_rps}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DDOS_AUTH_BURST" "${ddos_auth_burst}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_BACKEND" "${state_backend}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_NAME" "${state_db_name}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_USER" "${state_db_user}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_PASSWORD" "${state_db_password}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_STATE_DB_SOCKET" "${state_db_socket}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_WEB_STACK_MODE" "${web_stack_mode_current}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_NGINX_EDGE_ENABLED" "${nginx_edge_enabled}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_OLS_BACKEND_ADDR" "${ols_backend_addr}"

  chmod 600 "${GATEWAY_ENV_FILE}" "${SERVICE_ENV_FILE}"
}

configure_minio_service() {
  ensure_minio_binaries

  # shellcheck disable=SC1090
  source "${SERVICE_ENV_FILE}"

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
  local services=(mariadb postgresql redis-server redis docker fail2ban pdns pure-ftpd postfix dovecot aurapanel-htaccess-watcher)
  local unit=""

  if command -v nginx >/dev/null 2>&1; then
    services+=(nginx)
  fi

  for svc in "${services[@]}"; do
    unit="${svc}.service"
    if systemd_unit_exists "${unit}"; then
      systemctl enable "${svc}" >/dev/null 2>&1 || true
      systemctl restart "${svc}" >/dev/null 2>&1 || true
    fi
  done
}

sync_project() {
  log "Preparing project directory at ${PROJECT_DIR}..."
  mkdir -p "${PROJECT_DIR}"

  if [ -d "$(pwd)/panel-service" ] && [ -d "$(pwd)/api-gateway" ] && [ -d "$(pwd)/frontend" ]; then
    log "Copying current workspace into ${PROJECT_DIR}..."
    rsync -a --delete \
      --exclude '.git' \
      --exclude 'frontend/node_modules' \
      --exclude 'api-gateway/apigw' \
      --exclude 'panel-service/panel-service' \
      --exclude 'panel-service/panel-service.exe' \
      "$(pwd)/" "${PROJECT_DIR}/"
  else
    if [ "${AURAPANEL_INSTALL_SOURCE}" = "archive" ]; then
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

apply_web_stack_mode() {
  local mode script_path
  mode="$(web_stack_mode)"
  script_path="${PROJECT_DIR}/installer/web-stack-mode.sh"

  if [ ! -f "${script_path}" ]; then
    fail "web stack mode script not found: ${script_path}"
  fi

  log "Applying web stack mode: ${mode}"
  if ! bash "${script_path}" --mode "${mode}" --apply --auto-install-nginx; then
    fail "web stack mode apply failed (${mode})."
  fi
  ok "Web stack mode active: ${mode}"
}

build_components() {
  log "Building Go panel service..."
  cd "${PROJECT_DIR}/panel-service"
  /usr/local/go/bin/go mod tidy
  /usr/local/go/bin/go build -o panel-service .

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

  cat <<EOF > /etc/systemd/system/aurapanel-service.service
[Unit]
Description=AuraPanel Panel Service (Go)
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${PROJECT_DIR}/panel-service
ExecStart=${PROJECT_DIR}/panel-service/panel-service
Restart=on-failure
EnvironmentFile=-${SERVICE_ENV_FILE}

[Install]
WantedBy=multi-user.target
EOF

  cat <<EOF > /etc/systemd/system/aurapanel-api.service
[Unit]
Description=AuraPanel API Gateway (Go + Panel Static)
After=network.target aurapanel-service.service
Requires=aurapanel-service.service

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
  systemctl enable aurapanel-service aurapanel-api
  systemctl restart aurapanel-service aurapanel-api
}

smoke_check() {
  log "Running post-install smoke checks..."
  local panel_port panel_user panel_pass login_payload ols_user ols_pass ols_status stack_mode
  panel_port="$(gateway_port)"
  stack_mode="$(normalize_web_stack_mode "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_WEB_STACK_MODE")")"
  [ -n "${stack_mode}" ] || stack_mode="ols-only"

  systemctl is-active --quiet aurapanel-service || fail "aurapanel-service is not active"
  systemctl is-active --quiet aurapanel-api || fail "aurapanel-api is not active"
  systemctl is-active --quiet lshttpd || fail "lshttpd is not active"
  if [ "${stack_mode}" = "nginx-edge" ]; then
    systemctl is-active --quiet nginx || fail "nginx is not active in nginx-edge mode"
  fi
  systemctl is-active --quiet minio || fail "minio is not active"
  if systemd_unit_exists "pure-ftpd.service"; then
    systemctl is-active --quiet pure-ftpd || fail "pure-ftpd is not active"
  fi
  if systemd_unit_exists "postfix.service"; then
    systemctl is-active --quiet postfix || fail "postfix is not active"
  fi
  if systemd_unit_exists "dovecot.service"; then
    systemctl is-active --quiet dovecot || fail "dovecot is not active"
  fi
  if command -v ufw >/dev/null 2>&1; then
    ufw status 2>/dev/null | grep -qi "Status: active" || warn "ufw is installed but not active."
  fi
  if [ ! -f /usr/local/lsws/modules/mod_security.so ]; then
    warn "OpenLiteSpeed ModSecurity module is missing."
  fi
  if ! grep -q "module mod_security" /usr/local/lsws/conf/httpd_config.conf 2>/dev/null; then
    warn "OpenLiteSpeed ModSecurity block is missing from httpd_config.conf."
  fi

  curl -fsS http://127.0.0.1:8081/api/v1/health >/dev/null || fail "Panel service health check failed"
  curl -fsS "http://127.0.0.1:${panel_port}/api/health" >/dev/null || fail "Gateway health check failed"
  curl -fsS "http://127.0.0.1:${panel_port}/" >/dev/null || fail "Panel static endpoint failed"
  curl -fsS http://127.0.0.1:9000/minio/health/live >/dev/null || fail "MinIO health check failed"
  curl -fsS 'http://127.0.0.1/webmail/index.php?_task=login' >/dev/null 2>&1 || fail "Roundcube login endpoint failed"
  local pma_status pg_status
  pma_status="$(curl -s -o /dev/null -w '%{http_code}' 'http://127.0.0.1/phpmyadmin/index.php' || true)"
  case "${pma_status}" in
    2*|3*) ;;
    *) fail "phpMyAdmin endpoint failed (HTTP ${pma_status:-000})" ;;
  esac
  pg_status="$(curl -s -o /dev/null -w '%{http_code}' 'http://127.0.0.1/pgadmin4/' || true)"
  case "${pg_status}" in
    2*|3*) ;;
    *) fail "pgAdmin endpoint failed (HTTP ${pg_status:-000})" ;;
  esac

  panel_user="$(panel_admin_email)"
  panel_pass="$(panel_admin_password)"
  if [ -z "${panel_pass}" ]; then
    fail "Panel admin password is missing after install."
  fi

  login_payload="$(jq -nc --arg email "${panel_user}" --arg password "${panel_pass}" '{email:$email,password:$password}')"
  curl -fsS -H "Content-Type: application/json" -d "${login_payload}" "http://127.0.0.1:${panel_port}/api/v1/auth/login" >/dev/null || fail "Panel login verification failed (/api/v1/auth/login)"
  ok "AuraPanel login credentials verified."

  ols_user="$(ols_admin_user)"
  ols_pass="$(ols_admin_password)"
  if [ -z "${ols_pass}" ]; then
    fail "OpenLiteSpeed admin password is missing after install."
  fi

  ols_status="$(curl -k -sS -o /dev/null -w '%{http_code}' -u "${ols_user}:${ols_pass}" https://127.0.0.1:7080/ || true)"
  case "${ols_status}" in
    2*|3*)
      ok "OpenLiteSpeed WebAdmin credentials verified."
      ;;
    *)
      fail "OpenLiteSpeed WebAdmin login verification failed (HTTP ${ols_status:-unknown})."
      ;;
  esac

  if command -v ss >/dev/null 2>&1; then
    if ! ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)80$'; then
      warn "Port 80 listener was not detected. HTTP-01 SSL validation may fail."
    fi
    if ! ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)443$'; then
      warn "Port 443 listener was not detected. Public HTTPS traffic may fail."
    fi
    if [ "${stack_mode}" = "nginx-edge" ]; then
      ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '127\.0\.0\.1:8088$' || warn "OLS backend listener 127.0.0.1:8088 was not detected in nginx-edge mode."
      ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '127\.0\.0\.1:8443$' || warn "OLS backend TLS listener 127.0.0.1:8443 was not detected in nginx-edge mode."
    fi
    if systemctl list-unit-files | grep -q '^postfix\.service'; then
      ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)587$' || fail "Postfix submission listener (587) was not detected"
      ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)465$' || fail "Postfix SMTPS listener (465) was not detected"
    fi
    if systemctl list-unit-files | grep -q '^dovecot\.service'; then
      ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)143$' || fail "Dovecot IMAP listener (143) was not detected"
      ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq '(^|:)993$' || fail "Dovecot IMAPS listener (993) was not detected"
    fi
  fi

  local ffmpeg_install ffmpeg_active im_install im_active lo_install lo_active
  ffmpeg_install="$(normalize_yes_no "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_FFMPEG_INSTALL")" "1")"
  ffmpeg_active="$(normalize_yes_no "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_FFMPEG_ACTIVE")" "${ffmpeg_install}")"
  im_install="$(normalize_yes_no "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_IMAGEMAGICK_INSTALL")" "1")"
  im_active="$(normalize_yes_no "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_IMAGEMAGICK_ACTIVE")" "${im_install}")"
  lo_install="$(normalize_yes_no "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_LIBREOFFICE_INSTALL")" "1")"
  lo_active="$(normalize_yes_no "$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_LIBREOFFICE_ACTIVE")" "${lo_install}")"

  if [ "${ffmpeg_install}" = "1" ] && [ "${ffmpeg_active}" = "1" ]; then
    command -v ffmpeg >/dev/null 2>&1 || fail "FFmpeg binary missing while marked active."
    ffmpeg -version >/dev/null 2>&1 || fail "FFmpeg binary check failed."
  fi
  if [ "${im_install}" = "1" ] && [ "${im_active}" = "1" ]; then
    ensure_magick_command || fail "ImageMagick binary missing while marked active."
    magick -version >/dev/null 2>&1 || fail "ImageMagick binary check failed."
  fi
  if [ "${lo_install}" = "1" ] && [ "${lo_active}" = "1" ]; then
    if command -v libreoffice >/dev/null 2>&1; then
      libreoffice --version >/dev/null 2>&1 || fail "LibreOffice binary check failed."
    elif command -v soffice >/dev/null 2>&1; then
      soffice --version >/dev/null 2>&1 || fail "LibreOffice binary check failed."
    else
      fail "LibreOffice binary missing while marked active."
    fi
  fi

  local waf_status
  waf_status="$(curl -s -o /dev/null -w '%{http_code}' 'http://127.0.0.1/?q=%22%3E%3Cscript%3Ealert(123)%3C/script%3E' || true)"
  if [ "${waf_status}" = "403" ]; then
    ok "ModSecurity OWASP smoke test returned HTTP 403."
  else
    warn "ModSecurity OWASP smoke test did not return 403 (HTTP ${waf_status:-unknown})."
  fi

  ok "Smoke checks passed."
}

main() {
  echo -e "${BLUE}=================================================${NC}"
  echo -e "${GREEN} AuraPanel - Production Installation ${NC}"
  echo -e "${BLUE}=================================================${NC}"

  log "Installing system prerequisites..."
  if [ "${PKG_MGR}" = "apt" ]; then
    wait_for_package_manager
    apt-get update -y
    install_packages curl wget git rsync build-essential cmake pkg-config libssl-dev gcc ufw ca-certificates openssl jq unzip tar
    install_packages software-properties-common gnupg lsb-release
  else
    wait_for_package_manager
    dnf update -y
    dnf groupinstall -y "Development Tools"
    install_packages curl wget git rsync cmake openssl-devel openssl gcc firewalld ca-certificates jq unzip tar
    install_packages dnf-plugins-core
  fi

  setup_powerdns_repo
  install_pdns_policy_guard
  install_optional_packages restic mariadb-server postgresql redis-server redis docker docker.io fail2ban inotify-tools sqlite3 pdns-server pdns-backend-sqlite3 pure-ftpd postfix dovecot-core dovecot-imapd dovecot-pop3d
  cleanup_runtime_guards
  configure_media_tool_preferences
  install_media_tools

  ensure_go
  ensure_node20
  ensure_openlitespeed
  ensure_wp_cli
  ensure_ols_public_listeners
  ensure_openlitespeed_admin_php
  ensure_lsphp_database_drivers
  ensure_ols_modsecurity
  configure_ols_modsecurity_crs
  enable_ols_modsecurity
  ensure_ioncube_loader
  ensure_certbot
  configure_ols_admin_credentials
  configure_pureftpd

  sync_project
  write_service_env_defaults
  ensure_nginx_for_edge_mode
  apply_web_stack_mode
  harden_ssh_systemd_unit
  ensure_firewall_manager_active
  configure_powerdns
  configure_minio_service
  configure_roundcube
  configure_ols_webmail_route
  ensure_docker_runtime
  configure_db_tools_hardening
  configure_mail_stack_vmail
  configure_htaccess_watcher
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
  ok "Panel Service Health (internal): http://127.0.0.1:8081/api/v1/health"
  ok "OpenLiteSpeed Web: http://YOUR_SERVER_IP (80/443)"
  ok "OpenLiteSpeed Admin: https://YOUR_SERVER_IP:7080"
  write_access_summary
}

main "$@"
