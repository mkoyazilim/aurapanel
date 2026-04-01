# AuraPanel

<p align="right">
  English | <a href="./README.tr.md">Turkce</a>
</p>

AuraPanel is a modern hosting control plane focused on three non-negotiables:

- `Performance first`: low-overhead runtime design and deterministic automation
- `Reliability first`: control plane is decoupled from serving plane
- `Security first`: fail-closed APIs, strict RBAC, and explicit runtime state

AuraPanel is being built for 2026-grade infrastructure operations where platform teams need clear control over website, DNS, SSL, mail, database, and service lifecycle in one coherent panel.

## Table of Contents

- [Vision](#vision)
- [Why AuraPanel](#why-aurapanel)
- [Architecture](#architecture)
- [Feature Surface](#feature-surface)
- [Security Model](#security-model)
- [Performance Model](#performance-model)
- [Comparison Snapshot](#comparison-snapshot)
- [Docs and Wiki](#docs-and-wiki)
- [Production Installation](#production-installation)
- [Local Development](#local-development)
- [Build and Packaging](#build-and-packaging)
- [Repository Layout](#repository-layout)
- [Roadmap Direction](#roadmap-direction)
- [Contribution Principles](#contribution-principles)
- [License](#license)

## Vision

AuraPanel is not a thin UI over shell commands.

It is an operator-grade control plane for hosting businesses, infrastructure teams, and multi-tenant environments that need:

- stable website serving under operational load
- transparent and testable automation paths
- low-friction day-2 operations
- clear security boundaries between users, services, and host-level actions

The core principle is simple: websites must keep running even when panel components are restarted, upgraded, or temporarily unavailable.

## Why AuraPanel

AuraPanel is designed around operational honesty:

- if a feature is not wired to a real host/API/config path, it should not be marked as active
- unsupported flows return `501 Not Implemented` rather than fake success
- acceptance checks and runtime probes are part of normal operations, not optional extras

This approach reduces hidden risk, shrinks mean-time-to-diagnose, and keeps teams aligned with real production state.

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
- Vue + Vite operator UI
- workflow-oriented views for hosting lifecycle
- RBAC-aware frontend permission boundaries

`api-gateway/`
- central authenticated entry point
- JWT validation, middleware chain, request IDs, CORS, RBAC checks
- controlled proxying to panel-service
- static panel delivery in production

`panel-service/`
- host-level automation and runtime orchestration
- provisioning, tuning, hardening, backup, migration, and integration endpoints
- deterministic command/config workflows across Linux environments

## Feature Surface

AuraPanel includes real runtime integrations for:

- Website lifecycle: domain onboarding, vhost sync, rewrite handling, htaccess write-through
- SSL/TLS: issuance, custom certs, wildcard bindings, hostname mapping
- DNS: zone and record lifecycle with PowerDNS
- Mail stack: Postfix, Dovecot, mailbox provisioning, forward/catch-all flows
- FTP/SFTP: account and access management
- Databases: MariaDB/PostgreSQL provisioning, credentials, tuning, remote access controls
- Backup: website and database backups, MinIO target support
- Docker runtime: service/app lifecycle endpoints
- Cloudflare integration surfaces
- WordPress workflows through `wp-cli`
- Malware scan and quarantine flows
- Firewall and SSH key management
- Panel port and service process visibility
- Migration upload, analysis, and import paths

For explicit runtime endpoint status, see [ENDPOINT_AUDIT.md](./ENDPOINT_AUDIT.md).

## Security Model

AuraPanel follows a zero-trust, fail-closed model:

- every protected request must be authenticated
- role-based access controls are enforced at the gateway
- unsupported routes do not return cosmetic success payloads
- installer writes environment/runtime artifacts with controlled permissions
- release bootstrap supports manifest and hash verification
- firewall automation opens only required ports
- generated credentials are validated through smoke checks
- ModSecurity + OWASP CRS integration can be enabled for WAF coverage

Security posture is treated as an always-on runtime property, not a one-time installation checkbox.

## Performance Model

Performance-sensitive decisions in AuraPanel:

- `Decoupled serving path`: websites are served by OpenLiteSpeed, not by panel runtime
- `Go services`: predictable startup and memory behavior
- `Focused proxy layer`: API Gateway forwards core `/api/v1/` surface directly to panel-service
- `Deterministic integrations`: host operations through managed CLI/config flows
- `Operational isolation`: panel updates/restarts do not imply website downtime

## Comparison Snapshot

This is a technical positioning snapshot (not a licensing/pricing comparison).

| Area | AuraPanel | CyberPanel | cPanel/WHM | Plesk |
|---|---|---|---|---|
| Core positioning | Decoupled control plane + explicit runtime honesty | OLS-centric panel with fast setup path | Mature commercial standard for shared hosting | Mature commercial platform with extension ecosystem |
| Serving/Control separation | Strong architectural focus | Moderate, depends on deployment style | Historically integrated workflows | Integrated with broad extension stack |
| Runtime transparency | Emphasis on verifiable host-backed endpoints | Varies by module | Mature UI abstractions, less host-level transparency by default | Strong UX abstractions via extensions |
| Security posture goal | Zero-trust defaults, fail-closed behavior | Basic hardening available | Enterprise features via commercial tiers | Enterprise/security add-ons via extensions |
| Extensibility direction | API/gRPC-first roadmap, GitOps-friendly | Plugin ecosystem, OLS-focused | Commercial ecosystem and partner integrations | Large extension catalog |
| Ops philosophy | Deterministic automation over cosmetic completeness | Simplicity and speed | Operational maturity and market standardization | Broad compatibility and managed workflows |

See detailed analysis in [Wiki Comparisons](./wiki/Comparisons.md).

## Docs and Wiki

### Core Technical Docs

- [Documentation Index](./docs/documentation-index.md)
- [API Contract v1](./docs/api_contract_v1.md)
- [Final System Audit (2026-03-30)](./docs/final-system-audit-2026-03-30.md)
- [Product Overview](./docs/product-overview.md)
- [Hosting Panel Comparison](./docs/hosting-panel-comparison.md)
- [Endpoint Audit](./ENDPOINT_AUDIT.md)
- [Changelog](./CHANGELOG.md)

### Wiki Source Pages (GitHub Wiki Seed)

The repository includes a full wiki starter set under `wiki/`:

- [Wiki Home](./wiki/Home.md)
- [Install Guide](./wiki/Install-Guide.md)
- [Architecture](./wiki/Architecture.md)
- [Security Model](./wiki/Security-Model.md)
- [Performance Model](./wiki/Performance-Model.md)
- [Operations Runbook](./wiki/Operations-Runbook.md)
- [Migration Guide](./wiki/Migration-Guide.md)
- [Comparisons](./wiki/Comparisons.md)
- [FAQ](./wiki/FAQ.md)
- [Troubleshooting](./wiki/Troubleshooting.md)

You can copy these pages directly into GitHub Wiki or keep them versioned in-repo as canonical docs.

## Production Installation

### Supported Targets

- Ubuntu `22.04` and `24.04`
- Debian `12+`
- AlmaLinux `8/9`
- Rocky Linux `8/9`

### 1. Standard Remote Install

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
```

### 2. Verified Release Bootstrap

```bash
export AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

Optional explicit manifest:

```bash
export AURAPANEL_MANIFEST_URL="https://example.com/releases/latest/aurapanel_release_manifest.env"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

### 3. Existing Host Update (Git Pull Deploy)

```bash
cd /opt/aurapanel
bash scripts/deploy-main.sh
```

## Local Development

### Requirements

- Go `1.22+`
- Node.js `20+`

### Windows helper

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

### Manual startup

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

## Build and Packaging

Build all components:

```bash
make build
```

Create release tarball:

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
|-- frontend/           # Vue + Vite control panel
|-- web-site/           # Public marketing and docs website
|-- installer/          # Production installation logic
|-- docs/               # Technical reference docs
|-- wiki/               # Wiki source pages
|-- aurapanel_bootstrap.sh
|-- aurapanel_installer.sh
|-- install.sh
|-- start-dev.ps1
|-- Makefile
`-- ENDPOINT_AUDIT.md
```

## Roadmap Direction

Near-term platform direction:

- service-to-service trust tightening and token lifecycle hardening
- eBPF-backed telemetry and runtime drift detection
- deeper GitOps control loops for repeatable fleet operations
- richer migration assistants for cPanel/Plesk/CyberPanel transitions
- expanded operator analytics and auto-remediation recommendations

## Contribution Principles

- keep runtime claims honest
- prioritize real integrations over simulated responses
- avoid heavy dependencies without measurable value
- preserve control-plane and serving-plane decoupling
- treat host-level automation as production infrastructure code

## License

AuraPanel is distributed under the [MIT License](./LICENSE).

## Developer

Mkoyazilim ([www.mkoyazilim.com](https://www.mkoyazilim.com)) and Tahamada
