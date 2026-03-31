#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="${AURAPANEL_REPO_DIR:-/opt/aurapanel}"
BRANCH="${AURAPANEL_DEPLOY_BRANCH:-main}"
PANEL_HEALTH_URL="${AURAPANEL_PANEL_HEALTH_URL:-http://127.0.0.1:8081/api/v1/health}"
API_HEALTH_URL="${AURAPANEL_API_HEALTH_URL:-http://127.0.0.1:8090/api/health}"
DEPLOY_SKIP_RESTART="${AURAPANEL_DEPLOY_SKIP_RESTART:-0}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing command: $1" >&2
    exit 1
  }
}

log() {
  printf '[deploy] %s\n' "$*"
}

resolve_go_bin() {
  if command -v go >/dev/null 2>&1; then
    command -v go
    return 0
  fi
  if [ -x /usr/local/go/bin/go ]; then
    echo /usr/local/go/bin/go
    return 0
  fi
  return 1
}

require_cmd git
require_cmd npm
require_cmd systemctl
require_cmd curl

GO_BIN="$(resolve_go_bin || true)"
if [ -z "${GO_BIN}" ]; then
  echo "Missing command: go (/usr/local/go/bin/go is also unavailable)" >&2
  exit 1
fi

if [ ! -d "${REPO_DIR}/.git" ]; then
  echo "Repository not found: ${REPO_DIR}" >&2
  exit 1
fi

cd "${REPO_DIR}"

dirty="$(git status --porcelain --untracked-files=no)"
if [ -n "${dirty}" ]; then
  echo "Tracked working tree is dirty. Refusing deploy." >&2
  git status --short --branch >&2
  exit 1
fi

log "Fetching latest refs"
git fetch origin
git checkout "${BRANCH}"
git pull --ff-only origin "${BRANCH}"

log "Building panel-service"
"${GO_BIN}" -C "${REPO_DIR}/panel-service" build -o "${REPO_DIR}/panel-service/panel-service" .

log "Building api-gateway"
"${GO_BIN}" -C "${REPO_DIR}/api-gateway" build -o "${REPO_DIR}/api-gateway/apigw" .

log "Building frontend"
npm --prefix "${REPO_DIR}/frontend" ci
npm --prefix "${REPO_DIR}/frontend" run build

if [ "${DEPLOY_SKIP_RESTART}" = "1" ]; then
  log "Skipping service restart and health checks (AURAPANEL_DEPLOY_SKIP_RESTART=1)"
else
  log "Restarting services"
  systemctl daemon-reload
  systemctl restart aurapanel-service aurapanel-api
  systemctl is-active --quiet aurapanel-service
  systemctl is-active --quiet aurapanel-api

  log "Running health checks"
  curl -fsS "${PANEL_HEALTH_URL}" >/dev/null
  curl -fsS "${API_HEALTH_URL}" >/dev/null
fi

log "Deployed commit: $(git rev-parse --short HEAD)"
log "Done"
