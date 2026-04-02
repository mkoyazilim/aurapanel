#!/usr/bin/env bash
# AuraPanel bootstrap wrapper
# Usage:
# curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
# Optional (if mirror is protected): export AURAPANEL_DOWNLOAD_AUTH="user:pass"

set -euo pipefail

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
    return 0
  fi

  if command -v wget >/dev/null 2>&1; then
    if [ -n "${auth}" ] && [[ "${auth}" == *:* ]]; then
      user="${auth%%:*}"
      pass="${auth#*:}"
      wget -q --user="${user}" --password="${pass}" "$url" -O "$output"
    else
      wget -q "$url" -O "$output"
    fi
    return 0
  fi

  return 1
}

cleanup() {
  rm -f aurapanel.sh aurapanel_bootstrap.sh
}

trap cleanup EXIT

AURAPANEL_DOWNLOAD_BASE_URL="${AURAPANEL_DOWNLOAD_BASE_URL:-https://downloads.aurapanel.info/mirror}"
INSTALLER_BASE_URL="${AURAPANEL_INSTALLER_BASE:-${AURAPANEL_DOWNLOAD_BASE_URL}/installer}"

if [ -n "${AURAPANEL_MANIFEST_URL:-}" ] || [ -n "${AURAPANEL_RELEASE_BASE:-}" ] || [ -n "${AURAPANEL_BOOTSTRAP_URL:-}" ]; then
  if [ -n "${AURAPANEL_BOOTSTRAP_URL:-}" ]; then
    BOOTSTRAP_URL="$AURAPANEL_BOOTSTRAP_URL"
  elif [ -n "${AURAPANEL_RELEASE_BASE:-}" ]; then
    BOOTSTRAP_URL="${AURAPANEL_RELEASE_BASE%/}/aurapanel_bootstrap.sh"
  else
    BOOTSTRAP_URL="${INSTALLER_BASE_URL}/aurapanel_bootstrap.sh"
  fi

  download_file "$BOOTSTRAP_URL" "aurapanel_bootstrap.sh" || {
    echo "Unable to download aurapanel_bootstrap.sh from: $BOOTSTRAP_URL"
    exit 1
  }

  chmod +x aurapanel_bootstrap.sh
  ./aurapanel_bootstrap.sh "$@"
  exit $?
fi

MAIN_INSTALLER_URL="${AURAPANEL_MAIN_INSTALLER_URL:-${INSTALLER_BASE_URL}/aurapanel.sh}"
download_file "$MAIN_INSTALLER_URL" "aurapanel.sh" || {
  echo "Unable to download aurapanel.sh from: $MAIN_INSTALLER_URL"
  exit 1
}

chmod +x aurapanel.sh
./aurapanel.sh "$@"
