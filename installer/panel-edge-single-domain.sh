#!/usr/bin/env bash
set -euo pipefail

GATEWAY_ENV_FILE="/etc/aurapanel/aurapanel.env"
SERVICE_ENV_FILE="/etc/aurapanel/aurapanel-service.env"
VHOST_CONF="/usr/local/lsws/conf/vhosts/Example/vhconf.conf"
NGINX_SNIPPET="/etc/nginx/snippets/aurapanel_panel_single_domain.conf"

log() {
  echo "[panel-edge-single-domain] $*"
}

warn() {
  echo "[panel-edge-single-domain][warn] $*" >&2
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

gateway_port() {
  local raw port
  raw="$(read_env_value "${GATEWAY_ENV_FILE}" "AURAPANEL_GATEWAY_ADDR")"
  raw="$(printf '%s' "${raw}" | tr -d '[:space:]')"
  if [ -z "${raw}" ]; then
    raw=":8090"
  fi
  if [[ "${raw}" =~ :([0-9]{2,5})$ ]]; then
    port="${BASH_REMATCH[1]}"
  elif [[ "${raw}" =~ ^([0-9]{2,5})$ ]]; then
    port="${BASH_REMATCH[1]}"
  else
    port="8090"
  fi
  printf '%s' "${port}"
}

replace_global_rewrite_block() {
  local conf="$1"
  local rewrite_target="$2"
  local tmp
  tmp="$(mktemp /tmp/aurapanel-panel-edge-rewrite.XXXXXX)"
  awk -v target="${rewrite_target}" '
    BEGIN {
      skip = 0
      depth = 0
      replaced = 0
      block = "rewrite {\n" \
              "  enable 1\n" \
              "  logLevel 0\n" \
              "  RewriteCond %{REQUEST_URI} !^/webmail/\n" \
              "  RewriteRule ^(.*)$ http://" target "/$1 [P,L]\n" \
              "}"
    }
    !skip && $0 ~ /^rewrite[[:space:]]*\{$/ {
      print block
      skip = 1
      depth = 1
      replaced = 1
      next
    }
    skip {
      openCount = gsub(/\{/, "{")
      closeCount = gsub(/\}/, "}")
      depth += openCount - closeCount
      if (depth <= 0) {
        skip = 0
      }
      next
    }
    { print }
    END {
      if (!replaced) {
        print ""
        print block
      }
    }
  ' "${conf}" > "${tmp}"
  install -m 640 "${tmp}" "${conf}"
  rm -f "${tmp}"
}

ensure_ols_panel_gateway() {
  local proxy_addr="$1"

  if [ ! -f "${VHOST_CONF}" ]; then
    warn "OpenLiteSpeed Example vhost not found: ${VHOST_CONF}"
    return 0
  fi

  replace_global_rewrite_block "${VHOST_CONF}" "${proxy_addr}"

  local tmp
  tmp="$(mktemp /tmp/aurapanel-panel-edge-vhconf.XXXXXX)"
  awk '
    $0=="# AURAPANEL PANEL EDGE EXTPROC BEGIN" {skip=1; next}
    $0=="# AURAPANEL PANEL EDGE EXTPROC END" {skip=0; next}
    !skip {print}
  ' "${VHOST_CONF}" > "${tmp}"

  cat <<EOF >> "${tmp}"

# AURAPANEL PANEL EDGE EXTPROC BEGIN
extprocessor aurapanel_gateway {
  type                    proxy
  address                 ${proxy_addr}
  maxConns                1000
  initTimeout             60
  retryTimeout            0
  respBuffer              0
}
# AURAPANEL PANEL EDGE EXTPROC END
EOF

  install -m 640 "${tmp}" "${VHOST_CONF}"
  rm -f "${tmp}"
}

configure_nginx_panel_snippet() {
  local proxy_addr="$1"
  if ! command -v nginx >/dev/null 2>&1; then
    return 0
  fi

  mkdir -p /etc/nginx/snippets
  cat <<EOF > "${NGINX_SNIPPET}"
# Include inside your panel server block for single-domain gateway routing.
location / {
    proxy_pass http://${proxy_addr};
    proxy_http_version 1.1;
    proxy_set_header Host \$host;
    proxy_set_header X-Forwarded-Host \$host;
    proxy_set_header X-Forwarded-Proto \$scheme;
    proxy_set_header X-Forwarded-For \$remote_addr;
    proxy_set_header Upgrade \$http_upgrade;
    proxy_set_header Connection "upgrade";
}
EOF
  chmod 640 "${NGINX_SNIPPET}"
  log "nginx detected. Wrote ${NGINX_SNIPPET} (manual include required)."
}

restart_services() {
  if [ -x /usr/local/lsws/bin/lswsctrl ]; then
    /usr/local/lsws/bin/lswsctrl restart >/dev/null 2>&1 || {
      warn "OpenLiteSpeed restart failed."
      return 1
    }
  fi

  if command -v systemctl >/dev/null 2>&1; then
    systemctl restart aurapanel-api >/dev/null 2>&1 || true
    systemctl restart aurapanel-service >/dev/null 2>&1 || true
  fi
}

main() {
  local port proxy_addr
  port="$(gateway_port)"
  proxy_addr="127.0.0.1:${port}"

  ensure_ols_panel_gateway "${proxy_addr}"
  configure_nginx_panel_snippet "${proxy_addr}"

  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN" "1"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE" "0"
  chmod 600 "${SERVICE_ENV_FILE}" >/dev/null 2>&1 || true

  restart_services
  log "Single-domain panel edge enabled. Upstream=${proxy_addr}"
}

main "$@"
