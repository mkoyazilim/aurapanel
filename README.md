# AuraPanel

<p align="right">
  English | <a href="./README.tr.md">Türkçe</a>
</p>

AuraPanel is a modern hosting control plane built for operators who want a fast, security-conscious, and operationally honest stack.

The platform is designed around a decoupled architecture:

- `Vue 3 + Vite` frontend for the administrative interface
- `Go API Gateway` for authentication, RBAC enforcement, static panel delivery, and controlled proxying
- `Go Panel Service` for host automation, runtime integrations, and system-level orchestration
- `OpenLiteSpeed` as the web-serving layer

The core design goal is simple: the control plane should remain cleanly separated from the serving plane, so websites can keep running even if the panel itself is restarted, upgraded, or unavailable.

## Why AuraPanel

AuraPanel is not intended to be a thin UI over shell commands. It is being shaped as a real hosting platform with:

- performance-first operational design
- fail-closed security defaults
- explicit runtime honesty
- deterministic infrastructure automation
- direct host integrations instead of fake or placeholder success states

If a capability is not wired to the host, an external API, or a managed file/config path, it should not be presented as active.

## Architecture

```text
Browser
  -> Vue Frontend
  -> Go API Gateway
  -> Go Panel Service
  -> Host Services / Integrations
     - OpenLiteSpeed
     - MariaDB
     - PostgreSQL
     - Postfix
     - Dovecot
     - Pure-FTPd
     - PowerDNS
     - Redis
     - MinIO
     - Docker
     - WP-CLI
     - Cloudflare
```

### Control Plane Layers

`frontend/`
- Operator UI built with Vue 3, Vite, Vue Router, and a store-driven frontend architecture
- Focused on operational workflows, visibility, and low-friction host management

`api-gateway/`
- Central entry point for authenticated traffic
- Enforces request middleware, JWT validation, role-based access control, CORS, request IDs, and service proxying
- Serves the built panel UI in production

`panel-service/`
- Executes host-level automation and coordinates real runtime actions
- Owns website provisioning, mail provisioning, database management, firewall operations, tuning endpoints, backup flows, runtime apps, and service control

## Performance Philosophy

AuraPanel is engineered with a performance-first mindset:

- `Decoupled serving path`: websites are served by OpenLiteSpeed, not by the panel runtime
- `Go-based control services`: low-overhead binaries with predictable startup and memory behavior
- `Minimal proxy layers`: the API Gateway forwards the main `/api/v1/` surface directly to the panel-service
- `Fast local integrations`: system actions rely on deterministic CLI, service, and config bindings instead of heavyweight orchestration layers
- `Operational isolation`: panel restarts do not imply website downtime
- `Focused tuning surfaces`: high-impact runtime tuning is exposed only where it matters, such as OpenLiteSpeed, databases, FTP, PHP, and mail

## Security Philosophy

AuraPanel is built with a zero-trust and fail-closed posture in mind:

- every protected request is authenticated
- RBAC is enforced at the gateway layer
- unsupported endpoints return `501 Not Implemented` instead of fake success
- installer flow writes dedicated environment files with controlled permissions
- signed release bootstrap is supported through manifest verification
- firewall automation opens only the required hosting and panel ports
- panel and service credentials are generated, synced, and smoke-checked during installation
- ModSecurity and OWASP CRS integration are supported for WAF coverage
- SSH key workflows, 2FA flows, and security status endpoints are first-class features

## Implemented Runtime Surface

AuraPanel currently includes real integrations for:

- website provisioning and OpenLiteSpeed vhost synchronization
- `.htaccess` write-through and OpenLiteSpeed rewrite handling
- PHP version assignment and `php.ini` management
- MariaDB and PostgreSQL provisioning, credentials, remote access, and tuning
- Postfix and Dovecot provisioning, mailboxes, forwards, catch-all, and mail SSL flows
- Pure-FTPd and SFTP provisioning
- PowerDNS zone and record management
- SSL issuance, custom certificates, wildcard and hostname bindings
- backups, database backups, and internal MinIO backup target support
- Docker runtime and application management
- Cloudflare status and integration workflows
- WordPress management through `wp-cli`
- malware scanning and quarantine flows
- firewall and SSH key management
- panel port management and service/process visibility
- migration upload, analysis, and import workflows

For a more explicit runtime status summary, see [ENDPOINT_AUDIT.md](./ENDPOINT_AUDIT.md).

## Supported Installation Targets

The production installer currently targets:

- Ubuntu `22.04` and `24.04`
- Debian `12+`
- AlmaLinux `8/9`
- Rocky Linux `8/9`

## Production Installation

### 1. Standard Remote Install

This is the simplest way to start a remote installation from GitHub:

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
```

This flow uses the main installer and prepares the host with the required runtime stack.

### Update Existing Installation (Git Pull Deploy)

For hosts that are already installed, use the deploy script from the repository root:

```bash
cd /opt/aurapanel
bash scripts/deploy-main.sh
```

This flow performs `git pull --ff-only` on `main`, rebuilds backend and frontend components, restarts `aurapanel-service` and `aurapanel-api`, and runs health checks.

### 2. Verified Release Bootstrap

AuraPanel also supports a verified release bootstrap flow based on a signed manifest and SHA-256 checked release bundle.

Example:

```bash
export AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

You can also point the bootstrap process to a specific manifest:

```bash
export AURAPANEL_MANIFEST_URL="https://example.com/releases/latest/aurapanel_release_manifest.env"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

### 3. Direct Bootstrap Script

If you want to call the verified bootstrap stage directly:

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/aurapanel_bootstrap.sh -o aurapanel_bootstrap.sh
chmod +x aurapanel_bootstrap.sh
sudo AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download" ./aurapanel_bootstrap.sh
```

## What the Production Installer Configures

The installer is designed to provision a full panel host, including:

- OpenLiteSpeed
- Node.js 20
- Go toolchain
- MariaDB
- PostgreSQL
- Redis
- Docker
- PowerDNS
- Pure-FTPd
- Postfix
- Dovecot
- MinIO
- Roundcube
- ModSecurity with OWASP CRS
- WP-CLI
- systemd services for AuraPanel components
- firewall baseline rules
- smoke checks for panel, gateway, OpenLiteSpeed, MinIO, and auth flows

### Installed systemd Units

Production deployment creates and manages:

- `aurapanel-service`
- `aurapanel-api`

Depending on the host and enabled modules, AuraPanel also coordinates:

- `lshttpd`
- `mariadb`
- `postgresql`
- `redis` or `redis-server`
- `postfix`
- `dovecot`
- `pure-ftpd`
- `minio`
- `docker`
- `pdns`

## Local Development

### Requirements

- Go `1.22+`
- Node.js `20+`

### Windows Helper

The repository includes a helper script that starts the full local stack:

```powershell
.\start-dev.ps1
```

Default local endpoints:

- Frontend: `http://127.0.0.1:5173`
- Gateway: `http://127.0.0.1:8090`
- Panel Service: `http://127.0.0.1:8081`

Default development login:

- Email: `admin@server.com`
- Password: `password123`

### Manual Development Startup

Panel service:

```powershell
cd panel-service
go run .
```

Gateway:

```powershell
cd api-gateway
$env:AURAPANEL_SERVICE_URL='http://127.0.0.1:8081'
go run .
```

Frontend:

```powershell
cd frontend
npm install
npm run dev
```

## Build

Build all components:

```bash
make build
```

Create a release tarball:

```bash
make package
```

Clean artifacts:

```bash
make clean
```

## Repository Layout

```text
aurapanel/
|-- api-gateway/        # Go API Gateway
|-- panel-service/      # Go host automation and runtime orchestration
|-- frontend/           # Vue 3 + Vite control panel
|-- installer/          # Production installation logic
|-- docs/               # Supporting technical documentation
|-- aurapanel_bootstrap.sh
|-- aurapanel_installer.sh
|-- install.sh
|-- start-dev.ps1
|-- Makefile
`-- ENDPOINT_AUDIT.md
```

## Operational Principles

AuraPanel follows a few non-negotiable principles:

- `Control plane != serving plane`
- `Operational honesty over cosmetic completeness`
- `Security defaults before convenience`
- `Deterministic automation over fragile hidden state`
- `Performance-sensitive code paths should stay simple`

## Notes for Contributors

- keep runtime claims honest
- prefer real integrations over simulated success payloads
- avoid introducing heavy dependencies without a measurable operational benefit
- preserve the decoupled model where panel failures do not take websites down
- treat host-level automation as production-grade infrastructure code

## License

AuraPanel is distributed under the [MIT License](./LICENSE).

## Developer

Mkoyazilim ([www.mkoyazilim.com](https://www.mkoyazilim.com)) & Tahamada
