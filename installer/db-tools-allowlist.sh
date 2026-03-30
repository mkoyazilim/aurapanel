#!/usr/bin/env bash
set -euo pipefail

GATEWAY_ENV_DIR="/etc/aurapanel"
SERVICE_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-service.env"
DBTOOLS_ENV_FILE="${GATEWAY_ENV_DIR}/aurapanel-dbtools.env"
HARDENING_SCRIPT_DEFAULT="/opt/aurapanel/installer/db-tools-hardening.sh"

usage() {
  cat <<'EOF'
Usage:
  db-tools-allowlist.sh list
  db-tools-allowlist.sh set <ip_or_cidr_csv>
  db-tools-allowlist.sh add <ip_or_cidr_csv>
  db-tools-allowlist.sh remove <ip_or_cidr_csv>
  db-tools-allowlist.sh apply

Notes:
  - 127.0.0.1 and ::1 are always kept in allowlist.
  - Supported values: IPv4, IPv6, IPv4 CIDR, IPv6 CIDR.
  - set/add/remove automatically re-apply db-tools hardening policy.
EOF
}

log() {
  echo "[db-tools-allowlist] $*"
}

warn() {
  echo "[db-tools-allowlist][warn] $*" >&2
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

ipv4_token_valid() {
  local token="$1"
  local ip cidr octets octet
  ip="${token%%/*}"
  cidr=""
  if [ "${token}" != "${ip}" ]; then
    cidr="${token##*/}"
    if ! [[ "${cidr}" =~ ^[0-9]+$ ]] || [ "${cidr}" -lt 0 ] || [ "${cidr}" -gt 32 ]; then
      return 1
    fi
  fi
  if [[ ! "${ip}" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
    return 1
  fi
  IFS='.' read -r -a octets <<< "${ip}"
  for octet in "${octets[@]}"; do
    if [ "${octet}" -lt 0 ] || [ "${octet}" -gt 255 ]; then
      return 1
    fi
  done
  return 0
}

ipv6_token_valid() {
  local token="$1"
  local ip cidr
  ip="${token%%/*}"
  if [ "${token}" != "${ip}" ]; then
    cidr="${token##*/}"
    if ! [[ "${cidr}" =~ ^[0-9]+$ ]] || [ "${cidr}" -lt 0 ] || [ "${cidr}" -gt 128 ]; then
      return 1
    fi
  fi
  if [[ ! "${ip}" =~ ^[0-9A-Fa-f:]+$ ]]; then
    return 1
  fi
  if [[ "${ip}" != *:* ]]; then
    return 1
  fi
  return 0
}

validate_csv() {
  local value="$1"
  local token
  [ -n "${value}" ] || return 0
  IFS=',' read -r -a tokens <<< "${value}"
  for token in "${tokens[@]}"; do
    token="$(trim_csv_spaces "${token}")"
    [ -n "${token}" ] || continue
    if ipv4_token_valid "${token}" || ipv6_token_valid "${token}"; then
      continue
    fi
    warn "Invalid IP/CIDR token: ${token}"
    return 1
  done
  return 0
}

merge_csv() {
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

remove_tokens_from_csv() {
  local source="$1"
  local remove_csv="$2"
  local current token result list_remove
  if [ -z "${source}" ]; then
    return 0
  fi

  result=""
  IFS=',' read -r -a list_remove <<< "${remove_csv}"
  IFS=',' read -r -a list_current <<< "${source}"
  for current in "${list_current[@]}"; do
    current="$(trim_csv_spaces "${current}")"
    [ -n "${current}" ] || continue
    skip="0"
    for token in "${list_remove[@]}"; do
      token="$(trim_csv_spaces "${token}")"
      if [ "${current}" = "${token}" ]; then
        skip="1"
        break
      fi
    done
    if [ "${skip}" = "0" ]; then
      result="$(merge_csv "${result}" "${current}")"
    fi
  done
  printf '%s' "${result}"
}

normalize_allowlist() {
  local value="$1"
  value="$(trim_csv_spaces "${value}")"
  value="$(merge_csv "127.0.0.1,::1" "${value}")"
  value="$(dedupe_csv "${value}")"
  printf '%s' "${value}"
}

current_allowlist() {
  local value
  value="$(read_env_value "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS")"
  if [ -z "${value}" ]; then
    value="$(read_env_value "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS")"
  fi
  value="$(normalize_allowlist "${value}")"
  printf '%s' "${value}"
}

persist_allowlist() {
  local value="$1"
  upsert_env "${DBTOOLS_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS" "${value}"
  upsert_env "${SERVICE_ENV_FILE}" "AURAPANEL_DBTOOLS_ALLOWED_IPS" "${value}"
  chmod 600 "${DBTOOLS_ENV_FILE}" "${SERVICE_ENV_FILE}"
}

apply_hardening() {
  local hardening_script="${AURAPANEL_DBTOOLS_HARDENING_SCRIPT:-${HARDENING_SCRIPT_DEFAULT}}"
  if [ ! -x "${hardening_script}" ]; then
    if [ -f "${hardening_script}" ]; then
      chmod +x "${hardening_script}" >/dev/null 2>&1 || true
    fi
  fi
  if [ ! -x "${hardening_script}" ]; then
    warn "Hardening script not found/executable: ${hardening_script}"
    return 1
  fi
  "${hardening_script}"
}

main() {
  local cmd="${1:-list}"
  local value="${2:-}"
  local current updated

  case "${cmd}" in
    list)
      echo "AURAPANEL_DBTOOLS_ALLOWED_IPS=$(current_allowlist)"
      ;;
    set)
      value="$(trim_csv_spaces "${value}")"
      validate_csv "${value}" || exit 1
      updated="$(normalize_allowlist "${value}")"
      persist_allowlist "${updated}"
      apply_hardening
      log "Allowlist updated: ${updated}"
      ;;
    add)
      value="$(trim_csv_spaces "${value}")"
      validate_csv "${value}" || exit 1
      current="$(current_allowlist)"
      updated="$(merge_csv "${current}" "${value}")"
      updated="$(normalize_allowlist "${updated}")"
      persist_allowlist "${updated}"
      apply_hardening
      log "Allowlist updated: ${updated}"
      ;;
    remove)
      value="$(trim_csv_spaces "${value}")"
      validate_csv "${value}" || exit 1
      current="$(current_allowlist)"
      updated="$(remove_tokens_from_csv "${current}" "${value}")"
      updated="$(normalize_allowlist "${updated}")"
      persist_allowlist "${updated}"
      apply_hardening
      log "Allowlist updated: ${updated}"
      ;;
    apply)
      apply_hardening
      log "Hardening re-applied."
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
