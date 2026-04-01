# AuraPanel Final System Audit

Date: 2026-03-30  
Analyst: Codex (local + ssh_root MCP verification)

## 1) Scope

- Local stack validation on `D:\Projeler\aurapanel`
- Remote production host validation on `vmi3153886` (Ubuntu 24.04, kernel `6.8.0-106-generic`)
- End-to-end checks for:
  - installability and runtime status of core background services
  - gateway <-> panel-service <-> frontend communication
  - auth and zero-trust request flow

## 2) Local Validation Summary

### 2.1 Toolchain / Build / Tests

- Go: `go1.26.1` (windows/amd64)
- Node.js: `v24.14.1`
- npm: `11.11.0`

Results:

- `api-gateway`: `go test ./...` -> passed
- `panel-service`: `go test ./...` -> passed
- `frontend`: `npm test` -> passed (`2` files, `5` tests)
- `frontend`: `npm run build` -> passed

### 2.2 Runtime and Connectivity

Local runtime processes started and verified:

- panel-service: `127.0.0.1:8081`
- api-gateway: `:8090`
- frontend (Vite dev): `127.0.0.1:5173`

Verified:

- `GET http://127.0.0.1:8081/api/v1/health` -> `200`
- `GET http://127.0.0.1:8090/api/health` -> `200`
- `GET http://127.0.0.1:5173` -> `200`
- `GET http://127.0.0.1:5173/api/health` (Vite proxy -> gateway) -> `200`

Auth and protected route chain:

- `POST /api/v1/auth/login` via gateway -> token issued
- `GET /api/v1/auth/me` via gateway with bearer token -> `200`
- Additional protected endpoints via gateway (all `200` in test):
  - `/api/v1/status/services`
  - `/api/v1/status/processes`
  - `/api/v1/packages/list`
  - `/api/v1/vhost/list`
  - `/api/v1/security/status`
  - `/api/v1/cloudflare/status`

Artifacts:

- Local smoke report JSON: `.codex-runtime/local-smoke-report.json`

## 3) Remote Host Validation (ssh_root MCP)

### 3.1 AuraPanel units

- `aurapanel-api`: active + enabled, `NRestarts=0`
- `aurapanel-service`: active + enabled, `NRestarts=0`
- Both entered active state on `2026-03-30 20:12:03 CEST`

Systemd unit wiring:

- `aurapanel-api` reads `/etc/aurapanel/aurapanel.env`
- `aurapanel-service` reads `/etc/aurapanel/aurapanel-service.env`

### 3.2 Service matrix

Active on host:

- OpenLiteSpeed (`lshttpd`)
- MariaDB, PostgreSQL, Redis
- Postfix, Dovecot, Pure-FTPd
- MinIO, Docker, PowerDNS
- Fail2Ban (verified jail status)
- UFW (active)

### 3.3 Port and network exposure

Observed listeners:

- Public web path: `80`, `443` (OpenLiteSpeed)
- Panel/API: `8090` (gateway, externally reachable)
- Panel service: `127.0.0.1:8081` (loopback only)
- OLS admin: `7080`
- MinIO: `127.0.0.1:9000` (loopback)
- Databases loopback: `3306`, `5432`, `6379`
- Mail/FTP/DNS ports present as expected

### 3.4 Gateway, Frontend, Service communication

Verified:

- `GET http://127.0.0.1:8090/api/health` -> `200`
- `GET http://127.0.0.1:8081/api/v1/health` -> `200`
- `GET http://127.0.0.1:8090/` -> `200` with frontend HTML (gateway static panel serve is active)
- Login + protected API chain through gateway:
  - `/api/v1/auth/login` -> token issued
  - `/api/v1/auth/me` -> `200`
  - `/api/v1/status/services` -> `200` (returns running service list)
  - `/api/v1/security/status` -> `200`

Zero-trust behavior check on panel-service:

- Direct call without proxy headers -> `401 Unauthorized`
- Same call with valid internal proxy token + auth headers -> `200`

This confirms gateway-to-service trust boundary is enforced.

### 3.5 Runtime performance snapshot

Resource snapshots looked healthy:

- Load avg ~ `0.45 / 0.27 / 0.25`
- RAM ~ `1.4 GiB / 47 GiB`
- Disk usage on `/` ~ `4%`
- Process footprint (approx):
  - `panel-service` RSS ~ `16 MB`
  - `apigw` RSS ~ `12 MB`

## 4) Critical Findings and Risks

### High

1. Secrets present in plaintext env files

- `/etc/aurapanel/aurapanel.env` and `/etc/aurapanel/aurapanel-service.env` include sensitive values (admin credentials, internal tokens, service credentials).
- Permissions are strict (`600`, root-owned), but plaintext-at-rest still creates rotation and incident-response risk.

Recommendation:

- Move secrets to Vault/SOPS/age-backed encrypted source, inject at runtime.
- Rotate all currently deployed secrets and credentials after migration.

### Medium

2. Panel API port `8090` is internet-exposed

- UFW currently allows `8090/tcp` publicly.

Recommendation:

- Restrict `8090` to management CIDRs or place gateway strictly behind reverse proxy/WAF with ACL and rate-limit controls.

3. OpenLiteSpeed public pages are not currently serving AuraPanel UI by default

- `80/443` currently return OpenLiteSpeed welcome page; panel UI is reachable on `:8090`.

Recommendation:

- If single-domain panel access is intended, add OLS vhost/reverse-proxy mapping (`443 -> 8090`) with TLS and strict header policy.

## 5) Architecture-Level Notes

- Decoupled model is working as intended:
  - websites served by OLS
  - control-plane services isolated on dedicated ports/processes
- Gateway-enforced authentication and service proxy token model behaves correctly.
- Service list and security-status surfaces are wired to real host runtimes (no cosmetic mock behavior observed in tested routes).

## 6) Final Verdict

`READY WITH HARDENING ACTIONS`

The stack is operational end-to-end (local and remote), installs/runs correctly, and gateway/frontend/service communication is valid.  
Before broad production exposure, prioritize:

1. secret management hardening and credential rotation
2. external exposure policy for `8090`
3. optional 443 routing design for panel UX consistency

