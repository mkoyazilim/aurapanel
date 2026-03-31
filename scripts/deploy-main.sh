#!/usr/bin/env bash
set -euo pipefail

REPO_DIR="${AURAPANEL_REPO_DIR:-/opt/aurapanel}"
BRANCH="${AURAPANEL_DEPLOY_BRANCH:-main}"
PANEL_HEALTH_URL="${AURAPANEL_PANEL_HEALTH_URL:-http://127.0.0.1:8081/api/v1/health}"
API_HEALTH_URL="${AURAPANEL_API_HEALTH_URL:-http://127.0.0.1:8090/api/health}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing command: $1" >&2
    exit 1
  }
}

log() {
  printf '[deploy] %s\n' "$*"
}

require_cmd git
require_cmd go
require_cmd npm
require_cmd systemctl
require_cmd curl

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
go -C "${REPO_DIR}/panel-service" build -o "${REPO_DIR}/panel-service/panel-service" .

log "Building api-gateway"
go -C "${REPO_DIR}/api-gateway" build -o "${REPO_DIR}/api-gateway/apigw" .

log "Building frontend"
npm --prefix "${REPO_DIR}/frontend" ci
npm --prefix "${REPO_DIR}/frontend" run build

log "Restarting services"
systemctl daemon-reload
systemctl restart aurapanel-service aurapanel-api
systemctl is-active --quiet aurapanel-service
systemctl is-active --quiet aurapanel-api

log "Running health checks"
curl -fsS "${PANEL_HEALTH_URL}" >/dev/null
curl -fsS "${API_HEALTH_URL}" >/dev/null

log "Deployed commit: $(git rev-parse --short HEAD)"
log "Done"
