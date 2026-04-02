#!/usr/bin/env bash
# AuraPanel release bootstrap: manifest verify + bundle extract + entrypoint run

set -euo pipefail

log_err() {
  echo "[aurapanel-bootstrap] $1" >&2
}

log_info() {
  echo "[aurapanel-bootstrap] $1"
}

fail() {
  log_err "$1"
  exit 1
}

download_file() {
  local url="$1"
  local output="$2"
  local auth="${AURAPANEL_DOWNLOAD_AUTH:-}"
  local user=""
  local pass=""
  if command -v curl >/dev/null 2>&1; then
    if [ -n "${auth}" ]; then
      curl -fsSL -u "${auth}" "$url" -o "$output"
    else
      curl -fsSL "$url" -o "$output"
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ -n "${auth}" ] && [[ "${auth}" == *:* ]]; then
      user="${auth%%:*}"
      pass="${auth#*:}"
      wget -q --user="${user}" --password="${pass}" "$url" -O "$output"
    else
      wget -q "$url" -O "$output"
    fi
  else
    fail "curl or wget is required"
  fi
}

sha256_file() {
  local file_path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file_path" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file_path" | awk '{print $1}'
    return
  fi
  fail "sha256sum or shasum is required"
}

read_manifest_value() {
  local key="$1"
  local manifest_file="$2"
  awk -F'=' -v key="$key" '
    $1 == key {
      sub(/^[^=]*=/, "", $0)
      gsub(/^"|"$/, "", $0)
      print $0
      exit
    }
  ' "$manifest_file"
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="$(mktemp -d /tmp/aurapanel-bootstrap.XXXXXX)"
trap 'rm -rf "$WORK_DIR"' EXIT
AURAPANEL_DOWNLOAD_BASE_URL="${AURAPANEL_DOWNLOAD_BASE_URL:-https://downloads.aurapanel.info/mirror}"

if [ -n "${AURAPANEL_MANIFEST_URL:-}" ]; then
  MANIFEST_URL="$AURAPANEL_MANIFEST_URL"
elif [ -n "${AURAPANEL_RELEASE_BASE:-}" ]; then
  MANIFEST_URL="${AURAPANEL_RELEASE_BASE%/}/latest/aurapanel_release_manifest.env"
else
  MANIFEST_URL="${AURAPANEL_DOWNLOAD_BASE_URL}/releases/latest/aurapanel_release_manifest.env"
fi

MANIFEST_SIG_URL="${AURAPANEL_MANIFEST_SIG_URL:-${MANIFEST_URL}.sig}"
MANIFEST_PUBKEY="${AURAPANEL_MANIFEST_PUBKEY:-$SCRIPT_DIR/keys/aurapanel_manifest_public.pem}"
MANIFEST_FILE="$WORK_DIR/aurapanel_release_manifest.env"
MANIFEST_SIG_FILE="$WORK_DIR/aurapanel_release_manifest.env.sig"

log_info "downloading manifest"
download_file "$MANIFEST_URL" "$MANIFEST_FILE"

if [ -f "$MANIFEST_PUBKEY" ]; then
  log_info "verifying signed manifest"
  download_file "$MANIFEST_SIG_URL" "$MANIFEST_SIG_FILE"
  openssl dgst -sha256 -verify "$MANIFEST_PUBKEY" -signature "$MANIFEST_SIG_FILE" "$MANIFEST_FILE" >/dev/null 2>&1 \
    || fail "manifest signature verification failed"
elif [ "${AURAPANEL_ALLOW_UNSIGNED_MANIFEST:-0}" = "1" ]; then
  log_info "unsigned manifest allowed for this run"
else
  fail "no manifest public key found and unsigned manifests are disabled"
fi

BUNDLE_URL="$(read_manifest_value "bundle_url" "$MANIFEST_FILE")"
BUNDLE_SHA256="$(read_manifest_value "bundle_sha256" "$MANIFEST_FILE")"
BUNDLE_NAME="$(read_manifest_value "bundle_name" "$MANIFEST_FILE")"
ENTRYPOINT="$(read_manifest_value "entrypoint" "$MANIFEST_FILE")"
RELEASE_VERSION="$(read_manifest_value "version" "$MANIFEST_FILE")"

[ -n "$BUNDLE_URL" ] || fail "bundle_url missing in manifest"
[ -n "$BUNDLE_SHA256" ] || fail "bundle_sha256 missing in manifest"
[ -n "$ENTRYPOINT" ] || fail "entrypoint missing in manifest"

if [ -z "$BUNDLE_NAME" ]; then
  BUNDLE_NAME="$(basename "$BUNDLE_URL")"
fi

BUNDLE_FILE="$WORK_DIR/$BUNDLE_NAME"
EXTRACT_DIR="$WORK_DIR/release"

log_info "downloading bundle ${RELEASE_VERSION:-unknown}"
download_file "$BUNDLE_URL" "$BUNDLE_FILE"

ACTUAL_SHA256="$(sha256_file "$BUNDLE_FILE")"
[ "$ACTUAL_SHA256" = "$BUNDLE_SHA256" ] || fail "bundle sha256 mismatch"

mkdir -p "$EXTRACT_DIR"
tar -xzf "$BUNDLE_FILE" -C "$EXTRACT_DIR"

ENTRYPOINT_PATH="$EXTRACT_DIR/$ENTRYPOINT"
[ -x "$ENTRYPOINT_PATH" ] || fail "entrypoint not found or not executable: $ENTRYPOINT"

log_info "launching installer entrypoint"
exec "$ENTRYPOINT_PATH" "$@"
