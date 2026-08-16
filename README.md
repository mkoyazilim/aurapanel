# AuraPanel

<img width="1171" height="675" alt="panel2" src="https://github.com/user-attachments/assets/7fcefdc2-2f94-4f57-8713-6b5ba00acdbc" />
<img width="1861" height="648" alt="panel1" src="https://github.com/user-attachments/assets/9a9c9cb4-51b5-48a2-96e1-5f7e35a30b1b" />

**A lightweight, security-first control panel for OpenLiteSpeed.**

AuraPanel is a modern hosting control panel purpose-built for [OpenLiteSpeed](https://openlitespeed.org/). It combines real per-site isolation, a hardened single-binary architecture, and a clean, professional web UI — designed perfectly for **Ubuntu 24.04 LTS** and **Debian 12 (Bookworm)** servers.

> 🚀 **Status: Production Ready (Canlı Kullanıma Hazır).** AuraPanel v1.0 tamamlanmış, tüm güvenlik/izolasyon testlerinden geçmiş ve açık kaynak olarak kullanıma sunulmuştur. Kurulum adımları aşağıdadır.

## Why AuraPanel?

Most control panels are heavy, expose a large attack surface, and treat the web server as an afterthought. AuraPanel flips that:

- **Built for OpenLiteSpeed** — OLS is the native serving engine, managed through a validated config pipeline with automatic rollback. A failed config change can never take down your sites or the panel.
- **Real per-site isolation** — every site gets its own Linux UID/GID, LSPHP process identity, cgroup v2 limits (CPU / RAM / PIDs / IO), filesystem quota, private tmp and private logs. One compromised site cannot touch another.
- **One static binary** — the entire panel (API, scheduler, config renderer, ACME client, backup engine, external DNS) is a single CGO-free Go binary. The Vue 3 web UI and Monaco editor are embedded inside it.
- **Security first** — the panel never runs as root; privileged operations go through a tiny allowlist-only helper. Server-side sessions, CSRF protection, TOTP/WebAuthn MFA, rate limiting, fail2ban, optional reCAPTCHA, and append-only audit logs come standard.
- **Stable installs everywhere** — pinned version manifests guarantee that every install uses the same tested component set, whether it happens today or six months from now.

## Features

- **Core Hosting**
  - Domain & site management (main domains, subdomains, aliases)
  - SSL: Let's Encrypt (ACME), custom certificates, automatic renewal, HSTS
  - Multiple PHP versions (LSPHP) with per-site selection and a php.ini editor
  - MariaDB databases & users with gated Adminer access
  - Jailed SFTP accounts (no shell access by default)
  - Per-site resource limits: CPU, RAM, processes, IO, disk & inode quota
  - Configuration drift detection with one-click repair
  - Update Center with a compatibility matrix and automatic rollback

- **File Manager**
  - Secure site-scoped file management with a Monaco code editor
  - Chunked/resumable uploads with WebSocket progress tracking
  - ZIP/TAR.GZ support with archive-bomb protections
  - Trash system with automatic retention cleanup

- **Advanced Features (Phase 2)**
  - WordPress 1-Click Installer
  - Git Deploy (Webhook-driven deployments)
  - **Native Node.js App Management:** Smart project type selection (PHP vs Node.js) during site creation with automatic internal port allocation and built-in OpenLiteSpeed Reverse Proxy context routing.
  - Staging Environments (Clone & Push to Production)
  - Isolated Mail Server Integration

- **Enterprise & Scale (Phase 3)**
  - **Multi-Server Cluster:** Manage multiple AuraPanel nodes from a single dashboard.
  - **Remote Agents:** Lightweight, secure mTLS agent protocol for deploying sites across servers.
  - **Reseller System:** Resource quotas, user isolation, and dedicated reseller dashboards.
  - **External DNS:** Bi-directional sync with Cloudflare and AWS Route53. Full PowerDNS API v1 support with DNSSEC.
  - **Advanced WAF:** OWASP CRS integration with ModSecurity, custom rule editor, dry-run testing, and request logging.
  - **CDN Management:** OLS and Cloudflare cache purging, page-rule configuration, and hit/miss statistics.

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/v1.0.0/installer/aurapanel.sh | sudo -E bash
```

> The installer pins every component to tested versions (OpenLiteSpeed, LSPHP, MariaDB) and locks them against unattended upgrades. Updates are delivered through the panel's Update Center after CI validation.
> Available once `v1.0.0` is released.

## Architecture

```
AuraPanel
    |
    +-- aurapanel (unprivileged)
    |       REST/WS server · OLS config renderer · apply pipeline
    |       drift detector · ACME client · backup engine · scheduler
    |
    +-- aurapanel-priv (minimal privileged helper, allowlist-only ops)

SQLite (panel metadata) ── OpenLiteSpeed (serving engine)
     │                            │
     └── desired state ──► renderer ──► validate ──► snapshot
                                        apply ──► reload ──► health check
                                                        └── failure → rollback
```

## Tech Stack

- **Backend:** Go (CGO-free, single static binary)
- **Frontend:** Vue 3 + Vite, embedded via `go:embed`; Monaco Editor
- **Web server:** OpenLiteSpeed (native vhosts)
- **Data:** SQLite (WAL) for panel metadata, MariaDB for site databases
- **Isolation:** Linux user separation + cgroups v2 + filesystem quotas
- **Platform:** Ubuntu 24.04 LTS

## Documentation

| Document | Description |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Full architecture & security design, including the unchangeable principles |
| [ROADMAP.md](ROADMAP.md) | Development plan, work packages, test strategy |
| [FILE_MANAGER.md](FILE_MANAGER.md) | File Manager module specification |
| [DEPENDENCIES.md](DEPENDENCIES.md) | Dependency inventory & license policy |

## License

AuraPanel is released under the **MIT License**. See the `LICENSE` file for details.

## Disclaimer

AuraPanel is an independent project and is not affiliated with or endorsed by LiteSpeed Technologies. "OpenLiteSpeed" is a trademark of LiteSpeed Technologies, Inc.
