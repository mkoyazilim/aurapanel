#!/usr/bin/env bash
set -euo pipefail

GATEWAY_ENV_DIR="/etc/aurapanel"
SERVICE_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-service.env"
DBTOOLS_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-dbtools.env"
DBTOOLS_CONF_DIR="${GATEWAY_ENV_DIR}/db-tools"
DBTOOLS_RUNTIME_ALLOWLIST_FILE_DEFAULT="${DBTOOLS_CONF_DIR}/runtime-allowlist.txt"
VHOST_CONF="/usr/local/lsws/conf/vhosts/Example/vhconf.conf"
MODSEC_INCLUDE="/usr/local/lsws/conf/owasp/modsec_includes.conf"
MODSEC_CUSTOM="/usr/local/lsws/conf/owasp/modsec_dbtools.conf"
DBTOOLS_USERDB="/usr/local/lsws/conf/vhosts/Example/htpasswd-dbtools"
DBTOOLS_GROUPDB="/usr/local/lsws/conf/vhosts/Example/htgroup-dbtools"
CREDENTIALS_SUMMARY_FILE="/root/aurapanel_credentials.txt"
PHPMYADMIN_DIR="/usr/local/lsws/Example/html/phpmyadmin"
PGADMIN_PROXY_PATH="/pgadmin4/"
PGADMIN_CONTAINER_NAME="aurapanel-pgadmin"
PGADMIN_DATA_DIR_DEFAULT="/opt/aurapanel/pgadmin"
PGADMIN_PROXY_PORT_DEFAULT="5055"

DBTOOLS_PGADMIN_DEFAULT_EMAIL=""
DBTOOLS_PGADMIN_DEFAULT_PASSWORD=""
DBTOOLS_PGADMIN_PROXY_PORT="${PGADMIN_PROXY_PORT_DEFAULT}"
DBTOOLS_PGADMIN_DATA_DIR="${PGADMIN_DATA_DIR_DEFAULT}"

log() {
  echo "[db-tools-hardening] $*"
}

warn() {
  echo "[db-tools-hardening][warn] $*" >&2
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

trim_csv_spaces() {
  local value="$1"
  value="$(printf '%s' "${value}" | tr -d '[:space:]')"
  value="${value#,}"
  value="${value%,}"
  printf '%s' "${value}"
}

dedupe_csv() {
  local value="$1"
  if [ -z "${value}" ]; then
    return 0
  fi
  printf '%s' "${value}" | tr ',' '\n' | sed '/^$/d' | awk '!seen[$0]++' | paste -sd, -
}

merge_allowlist() {
  local base="$1"
  local extra="$2"
  if [ -z "${base}" ]; then
    printf '%s' "${extra}"
    return 0
  fi
  if [ -z "${extra}" ]; then
    printf '%s' "${base}"
    return 0
  fi
  printf '%s,%s' "${base}" "${extra}"
}

panel_edge_single_domain_enabled() {
  case "$(printf '%s' "${DBTOOLS_PANEL_EDGE_SINGLE_DOMAIN:-0}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
  esac
  return 1
}

safe_password() {
  openssl rand -base64 24 | tr -d '\n' | tr '/+' 'AB'
}

default_pgadmin_email() {
  local host
  host="$(hostname -f 2>/dev/null || hostname || true)"
  host="$(printf '%s' "${host}" | tr -d '[:space:]')"
  if [ -z "${host}" ] || ! printf '%s' "${host}" | grep -Eq '\.'; then
    host="aurapanel.info"
  fi
  printf 'admin@%s' "${host}"
}

web_owner_user() {
  if id -u nobody >/dev/null 2>&1; then
    printf 'nobody'
    return 0
  fi
  if id -u www-data >/dev/null 2>&1; then
    printf 'www-data'
    return 0
  fi
  printf 'root'
}

web_owner_group() {
  if getent group nogroup >/dev/null 2>&1; then
    printf 'nogroup'
    return 0
  fi
  if getent group nobody >/dev/null 2>&1; then
    printf 'nobody'
    return 0
  fi
  if getent group www-data >/dev/null 2>&1; then
    printf 'www-data'
    return 0
  fi
  printf 'root'
}

ensure_dbtools_credentials() {
  local env_user env_pass
  local svc_user svc_pass svc_ips svc_rate svc_runtime_file svc_reload svc_panel_edge
  local svc_pgadmin_email svc_pgadmin_pass svc_pgadmin_port svc_pgadmin_data_dir

  env_user="$(read_env_value "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER")"
  env_pass="$(read_env_value "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS")"
  svc_user="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER")"
  svc_pass="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS")"
  svc_ips="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS")"
  svc_rate="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN")"
  svc_runtime_file="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RUNTIME_ALLOWLIST_FILE")"
  svc_reload="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE")"
  svc_panel_edge="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN")"
  svc_pgadmin_email="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_DEFAULT_EMAIL")"
  svc_pgadmin_pass="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_DEFAULT_PASSWORD")"
  svc_pgadmin_port="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_PROXY_PORT")"
  svc_pgadmin_data_dir="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_DATA_DIR")"

  DBTOOLS_AUTH_USER="${AURAPANEL_DBTOOLS_AUTH_USER:-${env_user:-${svc_user:-dbtools}}}"
  DBTOOLS_AUTH_PASS="${AURAPANEL_DBTOOLS_AUTH_PASS:-${env_pass:-${svc_pass:-}}}"
  if [ -z "${DBTOOLS_AUTH_PASS}" ]; then
    DBTOOLS_AUTH_PASS="$(safe_password)"
  fi

  DBTOOLS_ALLOWED_IPS="${AURAPANEL_DBTOOLS_ALLOWED_IPS:-${svc_ips:-}}"
  DBTOOLS_ALLOWED_IPS="$(trim_csv_spaces "${DBTOOLS_ALLOWED_IPS}")"
  DBTOOLS_ALLOWED_IPS="$(merge_allowlist "127.0.0.1,::1" "${DBTOOLS_ALLOWED_IPS}")"
  DBTOOLS_ALLOWED_IPS="$(trim_csv_spaces "${DBTOOLS_ALLOWED_IPS}")"
  DBTOOLS_ALLOWED_IPS="$(dedupe_csv "${DBTOOLS_ALLOWED_IPS}")"

  DBTOOLS_RATE_LIMIT_PER_MIN="${AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN:-${svc_rate:-120}}"
  if ! [[ "${DBTOOLS_RATE_LIMIT_PER_MIN}" =~ ^[0-9]+$ ]]; then
    DBTOOLS_RATE_LIMIT_PER_MIN="120"
  fi
  if [ "${DBTOOLS_RATE_LIMIT_PER_MIN}" -lt 30 ]; then
    DBTOOLS_RATE_LIMIT_PER_MIN="30"
  fi
  if [ "${DBTOOLS_RATE_LIMIT_PER_MIN}" -gt 1000 ]; then
    DBTOOLS_RATE_LIMIT_PER_MIN="1000"
  fi

  DBTOOLS_RUNTIME_ALLOWLIST_FILE="${AURAPANEL_DBTOOLS_RUNTIME_ALLOWLIST_FILE:-${svc_runtime_file:-${DBTOOLS_RUNTIME_ALLOWLIST_FILE_DEFAULT}}}"
  DBTOOLS_RUNTIME_ALLOWLIST_FILE="$(printf '%s' "${DBTOOLS_RUNTIME_ALLOWLIST_FILE}" | tr -d '\r')"
  [ -n "${DBTOOLS_RUNTIME_ALLOWLIST_FILE}" ] || DBTOOLS_RUNTIME_ALLOWLIST_FILE="${DBTOOLS_RUNTIME_ALLOWLIST_FILE_DEFAULT}"
  DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE="${AURAPANEL_DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE:-${svc_reload:-0}}"
  DBTOOLS_PANEL_EDGE_SINGLE_DOMAIN="${AURAPANEL_PANEL_EDGE_SINGLE_DOMAIN:-${svc_panel_edge:-0}}"
  DBTOOLS_PGADMIN_DEFAULT_EMAIL="${AURAPANEL_PGADMIN_DEFAULT_EMAIL:-${svc_pgadmin_email:-$(default_pgadmin_email)}}"
  DBTOOLS_PGADMIN_DEFAULT_PASSWORD="${AURAPANEL_PGADMIN_DEFAULT_PASSWORD:-${svc_pgadmin_pass:-}}"
  DBTOOLS_PGADMIN_PROXY_PORT="${AURAPANEL_PGADMIN_PROXY_PORT:-${svc_pgadmin_port:-${PGADMIN_PROXY_PORT_DEFAULT}}}"
  DBTOOLS_PGADMIN_DATA_DIR="${AURAPANEL_PGADMIN_DATA_DIR:-${svc_pgadmin_data_dir:-${PGADMIN_DATA_DIR_DEFAULT}}}"

  if [ -z "${DBTOOLS_PGADMIN_DEFAULT_PASSWORD}" ]; then
    DBTOOLS_PGADMIN_DEFAULT_PASSWORD="$(safe_password)"
  fi
  if ! [[ "${DBTOOLS_PGADMIN_PROXY_PORT}" =~ ^[0-9]+$ ]] || [ "${DBTOOLS_PGADMIN_PROXY_PORT}" -lt 1024 ] || [ "${DBTOOLS_PGADMIN_PROXY_PORT}" -gt 65535 ]; then
    DBTOOLS_PGADMIN_PROXY_PORT="${PGADMIN_PROXY_PORT_DEFAULT}"
  fi
  DBTOOLS_PGADMIN_DATA_DIR="$(printf '%s' "${DBTOOLS_PGADMIN_DATA_DIR}" | tr -d '\r')"
  [ -n "${DBTOOLS_PGADMIN_DATA_DIR}" ] || DBTOOLS_PGADMIN_DATA_DIR="${PGADMIN_DATA_DIR_DEFAULT}"

  mkdir -p "${DBTOOLS_CONF_DIR}" "/usr/local/lsws/conf/vhosts/Example"
  upsert_env "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER" "${DBTOOLS_AUTH_USER}"
  upsert_env "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS" "${DBTOOLS_AUTH_PASS}"
  upsert_env "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS" "${DBTOOLS_ALLOWED_IPS}"
  upsert_env "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN" "${DBTOOLS_RATE_LIMIT_PER_MIN}"
  chmod 600 "${DBTOOLS_ENV_FILE}"

  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER" "${DBTOOLS_AUTH_USER}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS" "${DBTOOLS_AUTH_PASS}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS" "${DBTOOLS_ALLOWED_IPS}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN" "${DBTOOLS_RATE_LIMIT_PER_MIN}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RUNTIME_ALLOWLIST_FILE" "${DBTOOLS_RUNTIME_ALLOWLIST_FILE}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE" "${DBTOOLS_RELOAD_ON_ALLOWLIST_CHANGE}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PHPMYADMIN_BASE_URL" "/phpmyadmin/index.php"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_BASE_URL" "${PGADMIN_PROXY_PATH}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_DEFAULT_EMAIL" "${DBTOOLS_PGADMIN_DEFAULT_EMAIL}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_DEFAULT_PASSWORD" "${DBTOOLS_PGADMIN_DEFAULT_PASSWORD}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_PROXY_PORT" "${DBTOOLS_PGADMIN_PROXY_PORT}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_DATA_DIR" "${DBTOOLS_PGADMIN_DATA_DIR}"
  chmod 600 "${SERVICE_ENV_FILE}"

  mkdir -p "$(dirname "${DBTOOLS_RUNTIME_ALLOWLIST_FILE}")"
  chmod 755 "${DBTOOLS_CONF_DIR}" "$(dirname "${DBTOOLS_RUNTIME_ALLOWLIST_FILE}")" >/dev/null 2>&1 || true
  chmod o+x "${GATEWAY_ENV_DIR}" >/dev/null 2>&1 || true
  touch "${DBTOOLS_RUNTIME_ALLOWLIST_FILE}"
  chmod 644 "${DBTOOLS_RUNTIME_ALLOWLIST_FILE}"

  local hashed
  hashed="$(openssl passwd -apr1 "${DBTOOLS_AUTH_PASS}")"
  printf '%s:%s\n' "${DBTOOLS_AUTH_USER}" "${hashed}" > "${DBTOOLS_USERDB}"
  printf 'dbtools: %s\n' "${DBTOOLS_AUTH_USER}" > "${DBTOOLS_GROUPDB}"
  chmod 640 "${DBTOOLS_USERDB}" "${DBTOOLS_GROUPDB}"

  local realm_owner="root"
  local realm_group="root"
  if id -u lsadm >/dev/null 2>&1; then
    realm_owner="lsadm"
  fi
  if getent group nogroup >/dev/null 2>&1; then
    realm_group="nogroup"
  elif getent group nobody >/dev/null 2>&1; then
    realm_group="nobody"
  elif id -gn "${realm_owner}" >/dev/null 2>&1; then
    realm_group="$(id -gn "${realm_owner}")"
  fi
  chown "${realm_owner}:${realm_group}" "${DBTOOLS_USERDB}" "${DBTOOLS_GROUPDB}" >/dev/null 2>&1 || true
}

install_phpmyadmin_runtime() {
  local tmp_dir archive src_dir pma_url owner group secret

  pma_url="${AURAPANEL_PHPMYADMIN_DOWNLOAD_URL:-https://www.phpmyadmin.net/downloads/phpMyAdmin-latest-all-languages.tar.gz}"
  owner="$(web_owner_user)"
  group="$(web_owner_group)"
  mkdir -p "${PHPMYADMIN_DIR}"

  tmp_dir="$(mktemp -d /tmp/aurapanel-pma.XXXXXX)"
  archive="${tmp_dir}/phpmyadmin.tar.gz"

  if ! curl -fsSL "${pma_url}" -o "${archive}"; then
    rm -rf "${tmp_dir}"
    warn "phpMyAdmin archive download failed (${pma_url})."
    return 1
  fi
  if ! tar -xzf "${archive}" -C "${tmp_dir}"; then
    rm -rf "${tmp_dir}"
    warn "phpMyAdmin archive extract failed."
    return 1
  fi
  src_dir="$(find "${tmp_dir}" -maxdepth 1 -type d -name 'phpMyAdmin-*' | head -n1)"
  if [ -z "${src_dir}" ] || [ ! -d "${src_dir}" ]; then
    rm -rf "${tmp_dir}"
    warn "phpMyAdmin archive content not found."
    return 1
  fi

  rsync -a --delete --exclude='config.inc.php' "${src_dir}/" "${PHPMYADMIN_DIR}/"
  if [ ! -f "${PHPMYADMIN_DIR}/config.inc.php" ] && [ -f "${PHPMYADMIN_DIR}/config.sample.inc.php" ]; then
    cp "${PHPMYADMIN_DIR}/config.sample.inc.php" "${PHPMYADMIN_DIR}/config.inc.php"
  fi
  if [ -f "${PHPMYADMIN_DIR}/config.inc.php" ] && grep -q "\$cfg\['blowfish_secret'\] = ''" "${PHPMYADMIN_DIR}/config.inc.php"; then
    secret="$(safe_password)"
    sed -i "s/\$cfg\['blowfish_secret'\] = '';/\$cfg['blowfish_secret'] = '${secret}';/" "${PHPMYADMIN_DIR}/config.inc.php"
  fi

  mkdir -p "${PHPMYADMIN_DIR}/tmp"
  chown -R "${owner}:${group}" "${PHPMYADMIN_DIR}" >/dev/null 2>&1 || true
  find "${PHPMYADMIN_DIR}" -type d -exec chmod 755 {} \;
  find "${PHPMYADMIN_DIR}" -type f -exec chmod 644 {} \;
  chmod 750 "${PHPMYADMIN_DIR}/tmp" >/dev/null 2>&1 || true

  rm -rf "${tmp_dir}"
  log "phpMyAdmin installed to ${PHPMYADMIN_DIR}."
  return 0
}

install_pgadmin_runtime() {
  local image email password port data_dir code

  if ! command -v docker >/dev/null 2>&1; then
    warn "docker not found, pgAdmin runtime install skipped."
    return 1
  fi
  image="${AURAPANEL_PGADMIN_IMAGE:-dpage/pgadmin4:9}"
  email="${DBTOOLS_PGADMIN_DEFAULT_EMAIL}"
  password="${DBTOOLS_PGADMIN_DEFAULT_PASSWORD}"
  port="${DBTOOLS_PGADMIN_PROXY_PORT}"
  data_dir="${DBTOOLS_PGADMIN_DATA_DIR}"

  systemctl enable docker >/dev/null 2>&1 || true
  systemctl start docker >/dev/null 2>&1 || true

  mkdir -p "${data_dir}"
  chown -R 5050:5050 "${data_dir}" >/dev/null 2>&1 || true

  docker rm -f "${PGADMIN_CONTAINER_NAME}" >/dev/null 2>&1 || true
  if ! docker run -d \
    --name "${PGADMIN_CONTAINER_NAME}" \
    --restart unless-stopped \
    -p "127.0.0.1:${port}:80" \
    -e "PGADMIN_DEFAULT_EMAIL=${email}" \
    -e "PGADMIN_DEFAULT_PASSWORD=${password}" \
    -e "SCRIPT_NAME=${PGADMIN_PROXY_PATH%/}" \
    -v "${data_dir}:/var/lib/pgadmin" \
    "${image}" >/dev/null; then
    warn "pgAdmin container launch failed."
    return 1
  fi

  code=""
  for _ in $(seq 1 40); do
    code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${port}${PGADMIN_PROXY_PATH}" || true)"
    case "${code}" in
      2*|3*) break ;;
    esac
    sleep 1
  done
  case "${code}" in
    2*|3*)
      log "pgAdmin container is ready on 127.0.0.1:${port}${PGADMIN_PROXY_PATH}."
      return 0
      ;;
    *)
      warn "pgAdmin health probe failed (HTTP ${code:-000})."
      return 1
      ;;
  esac
}

ensure_dbtools_placeholder_dirs() {
  local pma_dir="${PHPMYADMIN_DIR}"
  local pg_dir="/usr/local/lsws/Example/html/pgadmin4"

  mkdir -p "${pma_dir}" "${pg_dir}"
  if [ ! -f "${pma_dir}/index.php" ] && [ ! -f "${pma_dir}/index.html" ]; then
    cat <<'EOF' > "${pma_dir}/index.html"
<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>phpMyAdmin Placeholder</title></head>
  <body>
    <h1>phpMyAdmin route is protected.</h1>
    <p>Install phpMyAdmin files into this directory to activate the UI.</p>
  </body>
</html>
EOF
    chmod 644 "${pma_dir}/index.html"
  fi

  if [ ! -f "${pg_dir}/index.py" ] && [ ! -f "${pg_dir}/index.html" ]; then
    cat <<'EOF' > "${pg_dir}/index.html"
<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>pgAdmin Placeholder</title></head>
  <body>
    <h1>pgAdmin route is protected.</h1>
    <p>Connect pgAdmin here or reverse proxy its service under this path.</p>
  </body>
</html>
EOF
    chmod 644 "${pg_dir}/index.html"
  fi

  chmod 755 "${pma_dir}" "${pg_dir}"
}

configure_ols_dbtools_context() {
  if [ ! -f "${VHOST_CONF}" ]; then
    warn "OpenLiteSpeed Example vhost not found at ${VHOST_CONF}, skipping context hardening."
    return 0
  fi

  local tmp_conf
  tmp_conf="$(mktemp /tmp/aurapanel-dbtools-vhconf.XXXXXX)"
  awk '
    $0=="# AURAPANEL DB TOOLS BEGIN" {skip=1; next}
    $0=="# AURAPANEL DB TOOLS END" {skip=0; next}
    !skip {print}
  ' "${VHOST_CONF}" > "${tmp_conf}"

  cat <<EOF >> "${tmp_conf}"

# AURAPANEL DB TOOLS BEGIN
context /phpmyadmin/{
  allowBrowse 1
  location ${PHPMYADMIN_DIR}/
  rewrite  {
    enable 0
  }
}

extprocessor aurapanel_pgadmin {
  type                    proxy
  address                 127.0.0.1:${DBTOOLS_PGADMIN_PROXY_PORT}
  maxConns                200
  initTimeout             60
  retryTimeout            0
  respBuffer              0
}

context /pgadmin4/{
  type proxy
  handler aurapanel_pgadmin
  addDefaultCharset off
  rewrite  {
    enable 0
  }
}
# AURAPANEL DB TOOLS END
EOF

  install -m 640 "${tmp_conf}" "${VHOST_CONF}"
  rm -f "${tmp_conf}"
}

ensure_dbtools_rewrite_exclusions() {
  local tmp_conf
  if [ ! -f "${VHOST_CONF}" ]; then
    return 0
  fi
  if ! grep -q "RewriteCond %{REQUEST_URI} !^/webmail/" "${VHOST_CONF}" 2>/dev/null; then
    return 0
  fi
  if grep -q "RewriteCond %{REQUEST_URI} !^/phpmyadmin/" "${VHOST_CONF}" 2>/dev/null; then
    return 0
  fi

  tmp_conf="$(mktemp /tmp/aurapanel-dbtools-rewrite.XXXXXX)"
  awk '
    {
      print
      if ($0 ~ /RewriteCond %\{REQUEST_URI\} !\^\/webmail\//) {
        print "  RewriteCond %{REQUEST_URI} !^/phpmyadmin/"
        print "  RewriteCond %{REQUEST_URI} !^/pgadmin4/"
      }
    }
  ' "${VHOST_CONF}" > "${tmp_conf}"
  install -m 640 "${tmp_conf}" "${VHOST_CONF}"
  rm -f "${tmp_conf}"
}

write_modsecurity_dbtools_rules() {
  mkdir -p "$(dirname "${MODSEC_CUSTOM}")"

  if panel_edge_single_domain_enabled; then
    cat <<EOF > "${MODSEC_CUSTOM}"
# AuraPanel DB tools hardening rules (single-domain edge mode)
# Access control is enforced by gateway token-gate on proxied routes.
SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005100,phase:1,pass,nolog,ctl:ruleRemoveById=920350"
EOF
    chmod 640 "${MODSEC_CUSTOM}"
    return 0
  fi

  cat <<EOF > "${MODSEC_CUSTOM}"
# AuraPanel DB tools hardening rules
SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005100,phase:1,pass,nolog,ctl:ruleRemoveById=920350,initcol:ip=%{REMOTE_ADDR}"

SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005101,phase:1,pass,nolog,setvar:tx.dbtools_allow=0"
SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005105,phase:1,pass,nolog,chain"
SecRule REMOTE_ADDR "@ipMatch ${DBTOOLS_ALLOWED_IPS}" "setvar:tx.dbtools_allow=1"
SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005106,phase:1,pass,nolog,chain"
SecRule REMOTE_ADDR "@ipMatchFromFile ${DBTOOLS_RUNTIME_ALLOWLIST_FILE}" "setvar:tx.dbtools_allow=1"
SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005107,phase:1,deny,status:403,log,msg:'AuraPanel DB tools blocked by IP allowlist',chain"
SecRule TX:DBTOOLS_ALLOW "@eq 0"

SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005102,phase:1,deny,status:405,log,msg:'AuraPanel DB tools method not allowed',chain"
SecRule REQUEST_METHOD "!^(GET|POST|HEAD)$"

SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005103,phase:1,pass,nolog,setvar:ip.dbtools_counter=+1,expirevar:ip.dbtools_counter=60"

SecRule IP:dbtools_counter "@gt ${DBTOOLS_RATE_LIMIT_PER_MIN}" \
  "id:1005104,phase:1,deny,status:429,log,msg:'AuraPanel DB tools rate limit exceeded'"
EOF
  chmod 640 "${MODSEC_CUSTOM}"
}

ensure_modsecurity_include() {
  if [ ! -f "${MODSEC_INCLUDE}" ]; then
    warn "ModSecurity include file not found at ${MODSEC_INCLUDE}, skipping WAF include wiring."
    return 0
  fi

  if ! grep -Fq "include modsec_dbtools.conf" "${MODSEC_INCLUDE}" 2>/dev/null; then
    printf '\ninclude modsec_dbtools.conf\n' >> "${MODSEC_INCLUDE}"
  fi
}

configure_nginx_dbtools_snippet() {
  if ! command -v nginx >/dev/null 2>&1; then
    return 0
  fi

  mkdir -p /etc/nginx/snippets
  local allow_lines=""
  local item=""
  IFS=',' read -r -a _ips <<< "${DBTOOLS_ALLOWED_IPS}"
  for item in "${_ips[@]}"; do
    item="$(trim_csv_spaces "${item}")"
    [ -n "${item}" ] || continue
    allow_lines="${allow_lines}    allow ${item};"$'\n'
  done

  cat <<EOF > /etc/nginx/snippets/aurapanel_dbtools_hardening.conf
# Include this snippet inside a server block if nginx is used in front of DB tools.
location ~* ^/(phpmyadmin|pgadmin4)(/|$) {
    auth_basic "AuraPanel DB Tools";
    auth_basic_user_file ${DBTOOLS_USERDB};
${allow_lines}    deny all;
}
EOF
  chmod 640 /etc/nginx/snippets/aurapanel_dbtools_hardening.conf
  log "nginx detected. Wrote /etc/nginx/snippets/aurapanel_dbtools_hardening.conf (manual include required)."
}

update_credentials_summary() {
  local file="${CREDENTIALS_SUMMARY_FILE}"
  [ -f "${file}" ] || return 0

  local tmp_file
  tmp_file="$(mktemp /tmp/aurapanel-dbtools-summary.XXXXXX)"
  awk '
    $0=="# AURAPANEL DB TOOLS BEGIN" {skip=1; next}
    $0=="# AURAPANEL DB TOOLS END" {skip=0; next}
    !skip {print}
  ' "${file}" > "${tmp_file}"
  cat <<EOF >> "${tmp_file}"

# AURAPANEL DB TOOLS BEGIN
DB Tools Basic Auth Username: ${DBTOOLS_AUTH_USER}
DB Tools Basic Auth Password: ${DBTOOLS_AUTH_PASS}
DB Tools Allowed IPs: ${DBTOOLS_ALLOWED_IPS}
DB Tools Rate Limit (/min): ${DBTOOLS_RATE_LIMIT_PER_MIN}
pgAdmin URL Path: ${PGADMIN_PROXY_PATH}
pgAdmin Login Email: ${DBTOOLS_PGADMIN_DEFAULT_EMAIL}
pgAdmin Login Password: ${DBTOOLS_PGADMIN_DEFAULT_PASSWORD}
# AURAPANEL DB TOOLS END
EOF
  install -m 600 "${tmp_file}" "${file}"
  rm -f "${tmp_file}"
}

restart_services() {
  if [ -x /usr/local/lsws/bin/lswsctrl ]; then
    /usr/local/lsws/bin/lswsctrl restart >/dev/null 2>&1 || {
      warn "OpenLiteSpeed restart failed after DB tools hardening."
      return 1
    }
  fi
  if command -v systemctl >/dev/null 2>&1; then
    systemctl restart aurapanel-service >/dev/null 2>&1 || true
  fi
}

main() {
  ensure_dbtools_credentials
  install_phpmyadmin_runtime || warn "phpMyAdmin runtime install failed, leaving placeholder content."
  install_pgadmin_runtime || warn "pgAdmin runtime install failed, leaving route protected but unavailable."
  ensure_dbtools_placeholder_dirs
  configure_ols_dbtools_context
  ensure_dbtools_rewrite_exclusions
  write_modsecurity_dbtools_rules
  ensure_modsecurity_include
  configure_nginx_dbtools_snippet
  update_credentials_summary
  restart_services

  log "Hardening completed."
  log "User: ${DBTOOLS_AUTH_USER}"
  log "Allowed IPs: ${DBTOOLS_ALLOWED_IPS}"
  log "Rate limit (/min): ${DBTOOLS_RATE_LIMIT_PER_MIN}"
}

main "$@"
