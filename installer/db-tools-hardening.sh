#!/usr/bin/env bash
set -euo pipefail

GATEWAY_ENV_DIR="/etc/aurapanel"
SERVICE_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-service.env"
DBTOOLS_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-dbtools.env"
DBTOOLS_CONF_DIR="${GATEWAY_ENV_DIR}/db-tools"
VHOST_CONF="/usr/local/lsws/conf/vhosts/Example/vhconf.conf"
MODSEC_INCLUDE="/usr/local/lsws/conf/owasp/modsec_includes.conf"
MODSEC_CUSTOM="/usr/local/lsws/conf/owasp/modsec_dbtools.conf"
DBTOOLS_USERDB="/usr/local/lsws/conf/vhosts/Example/htpasswd-dbtools"
DBTOOLS_GROUPDB="/usr/local/lsws/conf/vhosts/Example/htgroup-dbtools"
CREDENTIALS_SUMMARY_FILE="/root/aurapanel_credentials.txt"

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

ensure_dbtools_credentials() {
  local env_user env_pass
  local svc_user svc_pass svc_ips svc_rate

  env_user="$(read_env_value "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER")"
  env_pass="$(read_env_value "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS")"
  svc_user="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_USER")"
  svc_pass="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_AUTH_PASS")"
  svc_ips="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS")"
  svc_rate="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_RATE_LIMIT_PER_MIN")"

  DBTOOLS_AUTH_USER="${AURAPANEL_DBTOOLS_AUTH_USER:-${env_user:-${svc_user:-dbtools}}}"
  DBTOOLS_AUTH_PASS="${AURAPANEL_DBTOOLS_AUTH_PASS:-${env_pass:-${svc_pass:-}}}"
  if [ -z "${DBTOOLS_AUTH_PASS}" ]; then
    DBTOOLS_AUTH_PASS="$(openssl rand -base64 24 | tr -d '\n' | tr '/+' 'AB')"
  fi

  DBTOOLS_ALLOWED_IPS="${AURAPANEL_DBTOOLS_ALLOWED_IPS:-${svc_ips:-}}"
  DBTOOLS_ALLOWED_IPS="$(trim_csv_spaces "${DBTOOLS_ALLOWED_IPS}")"
  DBTOOLS_ALLOWED_IPS="$(merge_allowlist "127.0.0.1,::1" "${DBTOOLS_ALLOWED_IPS}")"
  DBTOOLS_ALLOWED_IPS="$(trim_csv_spaces "${DBTOOLS_ALLOWED_IPS}")"

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
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PHPMYADMIN_BASE_URL" "/phpmyadmin/index.php"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_PGADMIN_BASE_URL" "/pgadmin4/"
  chmod 600 "${SERVICE_ENV_FILE}"

  local hashed
  hashed="$(openssl passwd -apr1 "${DBTOOLS_AUTH_PASS}")"
  printf '%s:%s\n' "${DBTOOLS_AUTH_USER}" "${hashed}" > "${DBTOOLS_USERDB}"
  printf 'dbtools: %s\n' "${DBTOOLS_AUTH_USER}" > "${DBTOOLS_GROUPDB}"
  chmod 640 "${DBTOOLS_USERDB}" "${DBTOOLS_GROUPDB}"
}

ensure_dbtools_placeholder_dirs() {
  local pma_dir="/usr/local/lsws/Example/html/phpmyadmin"
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
  allowBrowse 0
  location /usr/local/lsws/Example/html/phpmyadmin/
  authName AuraPanelDBTools
  realm AuraPanelDBTools
  required user ${DBTOOLS_AUTH_USER}
  accessControl {
    deny *
    allow ${DBTOOLS_ALLOWED_IPS}
  }
}

context /pgadmin4/{
  allowBrowse 0
  location /usr/local/lsws/Example/html/pgadmin4/
  authName AuraPanelDBTools
  realm AuraPanelDBTools
  required user ${DBTOOLS_AUTH_USER}
  accessControl {
    deny *
    allow ${DBTOOLS_ALLOWED_IPS}
  }
}

realm AuraPanelDBTools {
  userDB {
    cacheTimeout 60
    maxCacheSize 200
    location ${DBTOOLS_USERDB}
  }

  groupDB {
    cacheTimeout 60
    maxCacheSize 200
    location ${DBTOOLS_GROUPDB}
  }
}
# AURAPANEL DB TOOLS END
EOF

  install -m 640 "${tmp_conf}" "${VHOST_CONF}"
  rm -f "${tmp_conf}"
}

write_modsecurity_dbtools_rules() {
  mkdir -p "$(dirname "${MODSEC_CUSTOM}")"

  cat <<EOF > "${MODSEC_CUSTOM}"
# AuraPanel DB tools hardening rules
SecAction "id:1005100,phase:1,pass,nolog,initcol:ip=%{REMOTE_ADDR}"

SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005101,phase:1,deny,status:403,log,msg:'AuraPanel DB tools blocked by IP allowlist',chain"
SecRule REMOTE_ADDR "!@ipMatch ${DBTOOLS_ALLOWED_IPS}"

SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005102,phase:1,deny,status:405,log,msg:'AuraPanel DB tools method not allowed',chain"
SecRule REQUEST_METHOD "!^(GET|POST|HEAD)$"

SecRule REQUEST_URI "@rx ^/(phpmyadmin|pgadmin4)(/|$)" \
  "id:1005103,phase:1,pass,nolog,setvar:ip.dbtools_counter=+1,expirevar:ip.dbtools_counter=60"

SecRule IP:DBTOOLS_COUNTER "@gt ${DBTOOLS_RATE_LIMIT_PER_MIN}" \
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
  ensure_dbtools_placeholder_dirs
  configure_ols_dbtools_context
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
