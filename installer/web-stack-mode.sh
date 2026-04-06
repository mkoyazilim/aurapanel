#!/usr/bin/env bash
set -euo pipefail

GATEWAY_ENV_FILE="/etc/aurapanel/aurapanel.env"
SERVICE_ENV_FILE="/etc/aurapanel/aurapanel-service.env"
OLS_HTTPD_CONF="/usr/local/lsws/conf/httpd_config.conf"
OLS_CTRL="/usr/local/lsws/bin/lswsctrl"
OLS_LOCK_DIR="/tmp/aurapanel-ols-config.lock.d"
OLS_LOCK_TIMEOUT_SEC="45"
OLS_LOCK_STALE_SEC="900"
NGINX_CONF="/etc/nginx/nginx.conf"
NGINX_EDGE_HTTP_CONF="/etc/nginx/conf.d/aurapanel_edge_http.conf"
NGINX_EDGE_STREAM_DIR="/etc/nginx/stream-conf.d"
NGINX_EDGE_STREAM_CONF="${NGINX_EDGE_STREAM_DIR}/aurapanel_edge_stream.conf"
STREAM_MARKER_BEGIN="# AURAPANEL STREAM BEGIN"
STREAM_MARKER_END="# AURAPANEL STREAM END"

MODE=""
APPLY_CHANGES="0"
AUTO_INSTALL_NGINX="0"

log() {
  echo "[web-stack-mode] $*"
}

warn() {
  echo "[web-stack-mode][warn] $*" >&2
}

fail() {
  echo "[web-stack-mode][error] $*" >&2
  exit 1
}

acquire_ols_lock() {
  local start now elapsed lock_mtime lock_age
  start="$(date +%s)"
  while true; do
    if mkdir "${OLS_LOCK_DIR}" >/dev/null 2>&1; then
      printf '%s\n' "$$" > "${OLS_LOCK_DIR}/owner" 2>/dev/null || true
      return 0
    fi

    if [ -d "${OLS_LOCK_DIR}" ]; then
      lock_mtime="$(stat -c %Y "${OLS_LOCK_DIR}" 2>/dev/null || echo 0)"
      now="$(date +%s)"
      lock_age="$(( now - lock_mtime ))"
      if [ "${lock_mtime}" -gt 0 ] && [ "${lock_age}" -gt "${OLS_LOCK_STALE_SEC}" ]; then
        warn "stale OLS lock detected; removing ${OLS_LOCK_DIR}."
        rm -rf "${OLS_LOCK_DIR}" >/dev/null 2>&1 || true
        sleep 1
        continue
      fi
    fi

    now="$(date +%s)"
    elapsed="$(( now - start ))"
    if [ "${elapsed}" -ge "${OLS_LOCK_TIMEOUT_SEC}" ]; then
      fail "timed out while waiting for OpenLiteSpeed config lock (${OLS_LOCK_DIR})."
    fi
    sleep 1
  done
}

release_ols_lock() {
  if [ -d "${OLS_LOCK_DIR}" ]; then
    rm -rf "${OLS_LOCK_DIR}" >/dev/null 2>&1 || true
  fi
}

normalize_mode() {
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

read_env_value() {
  local file="$1"
  local key="$2"
  if [ ! -f "${file}" ]; then
    return 0
  fi
  grep -E "^${key}=" "${file}" 2>/dev/null | tail -n1 | cut -d'=' -f2- || true
}

upsert_env() {
  local file="$1"
  local key="$2"
  local value="$3"
  mkdir -p "$(dirname "${file}")"
  touch "${file}"
  if grep -qE "^${key}=" "${file}" 2>/dev/null; then
    sed -i "s|^${key}=.*|${key}=${value}|g" "${file}"
  else
    printf '%s=%s\n' "${key}" "${value}" >> "${file}"
  fi
}

detect_pkg_mgr() {
  if command -v apt-get >/dev/null 2>&1; then
    echo "apt"
    return
  fi
  if command -v dnf >/dev/null 2>&1; then
    echo "dnf"
    return
  fi
  if command -v yum >/dev/null 2>&1; then
    echo "yum"
    return
  fi
  echo ""
}

install_nginx_if_needed() {
  local pkg_mgr
  pkg_mgr="$(detect_pkg_mgr)"
  if command -v nginx >/dev/null 2>&1; then
    return 0
  fi
  [ "${AUTO_INSTALL_NGINX}" = "1" ] || fail "nginx is not installed (use --auto-install-nginx)."

  case "${pkg_mgr}" in
    apt)
      DEBIAN_FRONTEND=noninteractive apt-get update -y
      DEBIAN_FRONTEND=noninteractive apt-get install -y nginx libnginx-mod-stream || DEBIAN_FRONTEND=noninteractive apt-get install -y nginx
      ;;
    dnf)
      dnf install -y nginx
      ;;
    yum)
      yum install -y nginx
      ;;
    *)
      fail "cannot install nginx automatically on this host (unsupported package manager)."
      ;;
  esac

  command -v nginx >/dev/null 2>&1 || fail "nginx installation failed."
}

ensure_ols_listener_addresses() {
  local mode="$1"
  local default_addr ssl_addr
  local default_secure="0"
  local ssl_secure="1"

  [ -f "${OLS_HTTPD_CONF}" ] || fail "OpenLiteSpeed config not found: ${OLS_HTTPD_CONF}"

  if [ "${mode}" = "nginx-edge" ]; then
    default_addr="127.0.0.1:8088"
    ssl_addr="127.0.0.1:8443"
  else
    default_addr="*:80"
    ssl_addr="*:443"
  fi

  sed -i "/^[[:space:]]*listener[[:space:]]\+Default[[:space:]]*{/,/^[[:space:]]*}/{s/^[[:space:]]*address[[:space:]]\+.*/    address                  ${default_addr}/;s/^[[:space:]]*secure[[:space:]]\+.*/    secure                   ${default_secure}/}" "${OLS_HTTPD_CONF}"
  sed -i "/^[[:space:]]*listener[[:space:]]\+AuraPanelSSL[[:space:]]*{/,/^[[:space:]]*}/{s/^[[:space:]]*address[[:space:]]\+.*/    address                  ${ssl_addr}/;s/^[[:space:]]*secure[[:space:]]\+.*/    secure                   ${ssl_secure}/}" "${OLS_HTTPD_CONF}"
}

restart_openlitespeed() {
  if [ -x "${OLS_CTRL}" ]; then
    "${OLS_CTRL}" restart >/dev/null 2>&1
    return $?
  fi
  warn "OpenLiteSpeed control binary not found: ${OLS_CTRL}"
  return 1
}

replace_or_append_stream_block() {
  local tmp
  tmp="$(mktemp /tmp/aurapanel-nginx-stream.XXXXXX)"
  awk '
    BEGIN {
      marker_begin = "'"${STREAM_MARKER_BEGIN}"'"
      marker_end = "'"${STREAM_MARKER_END}"'"
      in_marker = 0
      stream_seen = 0
      stream_include_seen = 0
      in_stream = 0
      stream_depth = 0
    }
    {
      line = $0

      if (line == marker_begin) {
        in_marker = 1
        next
      }
      if (line == marker_end) {
        in_marker = 0
        next
      }
      if (in_marker) {
        next
      }

      if (!stream_seen && line ~ /^[[:space:]]*stream[[:space:]]*\{/) {
        stream_seen = 1
        in_stream = 1
        stream_depth = 1
        print line
        next
      }

      if (in_stream) {
        if (line ~ /\/etc\/nginx\/stream-conf.d\/\*\.conf;/) {
          stream_include_seen = 1
        }
        open_count = gsub(/\{/, "{", line)
        close_count = gsub(/\}/, "}", line)
        stream_depth += open_count - close_count
        if (stream_depth <= 0) {
          if (!stream_include_seen) {
            print "    include /etc/nginx/stream-conf.d/*.conf;"
          }
          in_stream = 0
          stream_include_seen = 0
          print line
          next
        }
        print line
        next
      }

      print line
    }
    END {
      if (!stream_seen) {
        print ""
        print "'"${STREAM_MARKER_BEGIN}"'"
        print "stream {"
        print "    include /etc/nginx/stream-conf.d/*.conf;"
        print "}"
        print "'"${STREAM_MARKER_END}"'"
      }
    }
  ' "${NGINX_CONF}" > "${tmp}"
  install -m 644 "${tmp}" "${NGINX_CONF}"
  rm -f "${tmp}"
}

write_nginx_edge_http_conf() {
  cat <<'EOF' > "${NGINX_EDGE_HTTP_CONF}"
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;

    client_max_body_size 128m;
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;

    location / {
        proxy_pass http://127.0.0.1:8088;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Host $host;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
    }
}
EOF
  chmod 644 "${NGINX_EDGE_HTTP_CONF}" >/dev/null 2>&1 || true
}

write_nginx_edge_stream_conf() {
  mkdir -p "${NGINX_EDGE_STREAM_DIR}"
  cat <<'EOF' > "${NGINX_EDGE_STREAM_CONF}"
server {
    listen 443;
    listen [::]:443;
    proxy_pass 127.0.0.1:8443;
    proxy_connect_timeout 5s;
    proxy_timeout 3600s;
    ssl_preread on;
}
EOF
  chmod 644 "${NGINX_EDGE_STREAM_CONF}" >/dev/null 2>&1 || true
}

remove_nginx_edge_confs() {
  rm -f "${NGINX_EDGE_HTTP_CONF}" "${NGINX_EDGE_STREAM_CONF}"
}

reload_or_restart_nginx() {
  nginx -t >/dev/null 2>&1 || return 1
  if command -v systemctl >/dev/null 2>&1; then
    systemctl enable nginx >/dev/null 2>&1 || true
    systemctl restart nginx >/dev/null 2>&1 || return 1
  fi
  return 0
}

stop_disable_nginx_if_present() {
  if ! command -v nginx >/dev/null 2>&1; then
    return 0
  fi
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop nginx >/dev/null 2>&1 || true
    systemctl disable nginx >/dev/null 2>&1 || true
  fi
}

persist_mode_env() {
  local mode="$1"
  local edge_enabled="0"
  local backend_addr="*:80"
  if [ "${mode}" = "nginx-edge" ]; then
    edge_enabled="1"
    backend_addr="127.0.0.1:8088"
  fi

  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_WEB_STACK_MODE" "${mode}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_NGINX_EDGE_ENABLED" "${edge_enabled}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_OLS_BACKEND_ADDR" "${backend_addr}"
  chmod 600 "${SERVICE_ENV_FILE}" >/dev/null 2>&1 || true
}

resolve_mode() {
  local requested normalized from_env
  requested="${MODE}"
  normalized="$(normalize_mode "${requested}")"
  if [ -n "${normalized}" ]; then
    echo "${normalized}"
    return
  fi
  from_env="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_WEB_STACK_MODE")"
  normalized="$(normalize_mode "${from_env}")"
  if [ -n "${normalized}" ]; then
    echo "${normalized}"
    return
  fi
  echo "ols-only"
}

apply_mode() {
  local mode="$1"
  local ols_backup nginx_backup edge_http_backup edge_stream_backup
  local edge_http_exists="0"
  local edge_stream_exists="0"

  ols_backup="$(mktemp /tmp/aurapanel-ols-httpd.XXXXXX)"
  cp -f "${OLS_HTTPD_CONF}" "${ols_backup}"

  nginx_backup=""
  if [ -f "${NGINX_CONF}" ]; then
    nginx_backup="$(mktemp /tmp/aurapanel-nginx-conf.XXXXXX)"
    cp -f "${NGINX_CONF}" "${nginx_backup}"
  fi

  edge_http_backup="$(mktemp /tmp/aurapanel-nginx-edge-http.XXXXXX)"
  if [ -f "${NGINX_EDGE_HTTP_CONF}" ]; then
    edge_http_exists="1"
    cp -f "${NGINX_EDGE_HTTP_CONF}" "${edge_http_backup}"
  fi

  edge_stream_backup="$(mktemp /tmp/aurapanel-nginx-edge-stream.XXXXXX)"
  if [ -f "${NGINX_EDGE_STREAM_CONF}" ]; then
    edge_stream_exists="1"
    cp -f "${NGINX_EDGE_STREAM_CONF}" "${edge_stream_backup}"
  fi

  rollback() {
    warn "rolling back web stack changes."
    cp -f "${ols_backup}" "${OLS_HTTPD_CONF}" >/dev/null 2>&1 || true
    if [ -n "${nginx_backup}" ] && [ -f "${nginx_backup}" ]; then
      cp -f "${nginx_backup}" "${NGINX_CONF}" >/dev/null 2>&1 || true
    fi
    if [ "${edge_http_exists}" = "1" ]; then
      cp -f "${edge_http_backup}" "${NGINX_EDGE_HTTP_CONF}" >/dev/null 2>&1 || true
    else
      rm -f "${NGINX_EDGE_HTTP_CONF}" >/dev/null 2>&1 || true
    fi
    if [ "${edge_stream_exists}" = "1" ]; then
      cp -f "${edge_stream_backup}" "${NGINX_EDGE_STREAM_CONF}" >/dev/null 2>&1 || true
    else
      rm -f "${NGINX_EDGE_STREAM_CONF}" >/dev/null 2>&1 || true
    fi
    restart_openlitespeed || true
    if command -v nginx >/dev/null 2>&1; then
      nginx -t >/dev/null 2>&1 && systemctl restart nginx >/dev/null 2>&1 || true
    fi
  }

  acquire_ols_lock
  trap 'release_ols_lock' EXIT

  if [ "${mode}" = "nginx-edge" ]; then
    install_nginx_if_needed
    [ -f "${NGINX_CONF}" ] || fail "nginx config not found: ${NGINX_CONF}"

    replace_or_append_stream_block
    write_nginx_edge_http_conf
    write_nginx_edge_stream_conf

    ensure_ols_listener_addresses "nginx-edge"
    restart_openlitespeed || { rollback; fail "failed to restart OpenLiteSpeed in nginx-edge mode."; }

    if ! reload_or_restart_nginx; then
      rollback
      fail "failed to start nginx-edge mode."
    fi
  else
    ensure_ols_listener_addresses "ols-only"
    restart_openlitespeed || { rollback; fail "failed to restart OpenLiteSpeed in ols-only mode."; }

    remove_nginx_edge_confs
    if command -v nginx >/dev/null 2>&1; then
      if nginx -t >/dev/null 2>&1; then
        systemctl restart nginx >/dev/null 2>&1 || true
      fi
      stop_disable_nginx_if_present
    fi
  fi

  persist_mode_env "${mode}"
  release_ols_lock
  trap - EXIT
  rm -f "${ols_backup}" "${nginx_backup}" "${edge_http_backup}" "${edge_stream_backup}" >/dev/null 2>&1 || true
}

usage() {
  cat <<EOF
Usage: web-stack-mode.sh [--mode ols-only|nginx-edge] [--apply] [--auto-install-nginx]

Without --apply, prints resolved target mode.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --mode)
      [ $# -ge 2 ] || fail "--mode requires a value"
      MODE="$2"
      shift 2
      ;;
    --apply)
      APPLY_CHANGES="1"
      shift
      ;;
    --auto-install-nginx)
      AUTO_INSTALL_NGINX="1"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

TARGET_MODE="$(resolve_mode)"
[ -n "${TARGET_MODE}" ] || fail "invalid web stack mode"

if [ "${APPLY_CHANGES}" != "1" ]; then
  echo "${TARGET_MODE}"
  exit 0
fi

log "Applying web stack mode: ${TARGET_MODE}"
apply_mode "${TARGET_MODE}"
log "Web stack mode active: ${TARGET_MODE}"
