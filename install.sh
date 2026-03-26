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
MINIO_ENV_FILE="/etc/default/minio"
REPO_URL="https://github.com/mkoyazilim/aurapanel.git"
PANEL_PORT_DEFAULT="8090"

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
  local port="$1"
  local rule="${port}/tcp"
  local touched="0"

  if command -v ufw >/dev/null 2>&1; then
    if ufw status 2>/dev/null | grep -qi "Status: active"; then
      if ufw allow "${rule}" >/dev/null 2>&1; then
        ok "ufw rule added for AuraPanel port ${port}/tcp"
        touched="1"
      else
        warn "ufw is active but failed to allow ${port}/tcp."
      fi
    else
      warn "ufw is installed but inactive. Skipping ufw rule automation."
    fi
  fi

  if command -v firewall-cmd >/dev/null 2>&1; then
    if firewall-cmd --state >/dev/null 2>&1; then
      if firewall-cmd --permanent --add-port="${rule}" >/dev/null 2>&1; then
        firewall-cmd --reload >/dev/null 2>&1 || true
        ok "firewalld rule added for AuraPanel port ${port}/tcp"
        touched="1"
      else
        warn "firewalld is active but failed to add ${port}/tcp."
      fi
    else
      warn "firewalld is installed but inactive. Skipping firewalld rule automation."
    fi
  fi

  if [ "${touched}" = "0" ]; then
    warn "No active firewall manager detected for automated port opening."
  fi
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

  log "Installing Node.js 20.x..."
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  install_packages nodejs
  ok "Node.js installed: $(node -v)"
}

ensure_rust() {
  log "Ensuring Rust toolchain..."
  if ! command -v cargo >/dev/null 2>&1; then
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
  fi

  if [ -f "${HOME}/.cargo/env" ]; then
    # shellcheck disable=SC1090
    source "${HOME}/.cargo/env"
  fi
}

ensure_go() {
  log "Ensuring Go toolchain..."
  if ! command -v go >/dev/null 2>&1; then
    local go_tarball="go1.22.1.linux-amd64.tar.gz"
    wget -q "https://go.dev/dl/${go_tarball}" -O "/tmp/${go_tarball}"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf "/tmp/${go_tarball}"
    rm -f "/tmp/${go_tarball}"
  fi

  export PATH="$PATH:/usr/local/go/bin"
}

ensure_openlitespeed() {
  if [ -x /usr/local/lsws/bin/lswsctrl ]; then
    ok "OpenLiteSpeed already installed."
  else
    log "Installing OpenLiteSpeed..."
    curl -fsSL https://repo.litespeed.sh | bash

    install_packages openlitespeed || fail "OpenLiteSpeed installation failed."
    install_optional_packages lsphp83 lsphp83-common lsphp83-mysql lsphp83-curl lsphp83-xml lsphp83-zip lsphp83-opcache
  fi

  systemctl enable lshttpd >/dev/null 2>&1 || true
  systemctl restart lshttpd >/dev/null 2>&1 || true
}

ensure_minio_binaries() {
  if ! command -v minio >/dev/null 2>&1; then
    log "Installing MinIO binary..."
    wget -q https://dl.min.io/server/minio/release/linux-amd64/minio -O /usr/local/bin/minio
    chmod +x /usr/local/bin/minio
  fi

  if ! command -v mc >/dev/null 2>&1; then
    log "Installing MinIO client (mc)..."
    wget -q https://dl.min.io/client/mc/release/linux-amd64/mc -O /usr/local/bin/mc
    chmod +x /usr/local/bin/mc
  fi
}

write_core_env_defaults() {
  mkdir -p "${GATEWAY_ENV_DIR}" "${PROJECT_DIR}/logs"
  chmod 700 "${GATEWAY_ENV_DIR}"

  if [ ! -f "${GATEWAY_ENV_FILE}" ]; then
    local admin_pass jwt_secret
    admin_pass="$(openssl rand -base64 18 | tr -d '\n')"
    jwt_secret="$(openssl rand -hex 32 | tr -d '\n')"

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

  if [ ! -f "${CORE_ENV_FILE}" ]; then
    local restic_pass minio_access minio_secret
    restic_pass="$(openssl rand -hex 24 | tr -d '\n')"
    minio_access="backup$(openssl rand -hex 3 | tr -d '\n')"
    minio_secret="$(openssl rand -hex 24 | tr -d '\n')"

    cat <<EOF > "${CORE_ENV_FILE}"
AURAPANEL_RUNTIME_MODE=production
AURAPANEL_SECURITY_POLICY=fail-closed
AURAPANEL_GATEWAY_ONLY=1
AURAPANEL_CORE_BIND_ADDR=127.0.0.1:8000
AURAPANEL_FEDERATION_MODE=active-passive
AURAPANEL_FEDERATION_PRIMARY=1
AURAPANEL_BACKUP_TARGET=internal-minio
AURAPANEL_BACKUP_MINIO_ENDPOINT=http://127.0.0.1:9000
AURAPANEL_BACKUP_MINIO_BUCKET=aurapanel-backups
AURAPANEL_BACKUP_MINIO_ACCESS_KEY=${minio_access}
AURAPANEL_BACKUP_MINIO_SECRET_KEY=${minio_secret}
AURAPANEL_BACKUP_RESTIC_PASSWORD=${restic_pass}
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
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_CORE_BIND_ADDR" "127.0.0.1:8000"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_FEDERATION_MODE" "active-passive"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_FEDERATION_PRIMARY" "1"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_BACKUP_TARGET" "internal-minio"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_BACKUP_MINIO_ENDPOINT" "http://127.0.0.1:9000"
  upsert_env "${CORE_ENV_FILE}" "AURAPANEL_BACKUP_MINIO_BUCKET" "aurapanel-backups"

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
  local services=(mariadb postgresql redis-server redis docker fail2ban pdns)

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

  curl -fsS http://127.0.0.1:8000/api/v1/health >/dev/null || fail "Core health check failed"
  curl -fsS "http://127.0.0.1:${panel_port}/api/health" >/dev/null || fail "Gateway health check failed"
  curl -fsS "http://127.0.0.1:${panel_port}/" >/dev/null || fail "Panel static endpoint failed"
  curl -fsS http://127.0.0.1:9000/minio/health/live >/dev/null || fail "MinIO health check failed"

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

  install_optional_packages restic mariadb-server postgresql redis-server redis docker docker.io fail2ban powerdns pdns

  ensure_rust
  ensure_go
  ensure_node20
  ensure_openlitespeed

  sync_project
  write_core_env_defaults
  configure_minio_service
  build_components
  configure_systemd_services
  enable_stack_services
  configure_panel_firewall "$(gateway_port)"
  smoke_check

  local panel_port
  panel_port="$(gateway_port)"
  ok "AuraPanel deployment is complete."
  ok "Panel URL: http://YOUR_SERVER_IP:${panel_port}"
  ok "API Health: http://YOUR_SERVER_IP:${panel_port}/api/health"
  ok "Core Health (internal): http://127.0.0.1:8000/api/v1/health"
  ok "OpenLiteSpeed Web: http://YOUR_SERVER_IP (80/443)"
  ok "OpenLiteSpeed Admin: https://YOUR_SERVER_IP:7080"
}

main "$@"
