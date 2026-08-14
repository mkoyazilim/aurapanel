# AuraPanel

**A lightweight, security-first control panel for OpenLiteSpeed.**

AuraPanel is a modern hosting control panel purpose-built for [OpenLiteSpeed](https://openlitespeed.org/). It combines real per-site isolation, a hardened single-binary architecture, and a clean, professional web UI — designed for Ubuntu 24.04 LTS servers.

> ⚠️ **Status:** In active development. The installer goes live with the first tagged release (`v1.0.0`).

## Why AuraPanel?

Most control panels are heavy, expose a large attack surface, and treat the web server as an afterthought. AuraPanel flips that:

- **Built for OpenLiteSpeed** — OLS is the native serving engine, managed through a validated config pipeline with automatic rollback. A failed config change can never take down your sites or the panel.
- **Real per-site isolation** — every site gets its own Linux UID/GID, LSPHP process identity, cgroup v2 limits (CPU / RAM / PIDs / IO), filesystem quota, private tmp and private logs. One compromised site cannot touch another.
- **One static binary** — the entire panel (API, scheduler, config renderer, ACME client, backup engine) is a single CGO-free Go binary. The Vue 3 web UI and Monaco editor are embedded inside it.
- **Security first** — the panel never runs as root; privileged operations go through a tiny allowlist-only helper. Server-side sessions, CSRF protection, TOTP/WebAuthn MFA, rate limiting, fail2ban, optional reCAPTCHA, and append-only audit logs come standard.
- **Stable installs everywhere** — pinned version manifests guarantee that every install uses the same tested component set, whether it happens today or six months from now.

## Features

- Domain & site management (main domains, subdomains, aliases)
- SSL: Let's Encrypt (ACME), custom certificates, automatic renewal, HSTS
- Multiple PHP versions (LSPHP) with per-site selection and a php.ini editor
- **File Manager** — secure site-scoped file management with a Monaco code editor, chunked/resumable uploads, ZIP/TAR.GZ support, trash with retention, and strict path-traversal, symlink and archive-bomb protections
- MariaDB databases & users with gated Adminer access
- Jailed SFTP accounts (no shell access by default)
- Scheduled, encrypted backups (local, S3-compatible, MinIO, remote)
- Per-site resource limits: CPU, RAM, processes, IO, disk & inode quota
- Live logs, cron management, health checks
- Security profiles (Compatibility / Balanced / Hardened) and per-feature toggles
- Configuration drift detection with one-click repair
- Optional ModSecurity + OWASP CRS WAF per site
- Update Center with a compatibility matrix and automatic rollback

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

AuraPanel is distributed **free of charge**. The distribution license is being finalized and will be published with the first release.

## Disclaimer

AuraPanel is an independent project and is not affiliated with or endorsed by LiteSpeed Technologies. "OpenLiteSpeed" is a trademark of LiteSpeed Technologies, Inc.
