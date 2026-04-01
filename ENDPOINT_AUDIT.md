# AuraPanel Endpoint Audit

Last updated: 2026-03-28

This file tracks runtime honesty for the current Go panel-service surface.

## Real Runtime

- Host metrics, service/process control
- Firewall status and rule management
- SSH key management
- Mailbox, forward, catch-all, mail domain provisioning
- Cloudflare status and DNS/settings API integration
- Database create/list/drop/password/remote access
- FTP/SFTP provisioning
- Cron provisioning
- File manager, archive, extract, trash, delete
- Site and database backups
- GitOps repo deploy
- Malware scan/quarantine/restore
- Docker containers/images and runtime app systemd flows (Full support with env/volume/limits)
- Redis isolation
- Migration upload/analyze/import/status
- SSL certificate issue/custom store/inspection
- Panel-wide SSL consumer bindings (Hostname to OLS, Mail to Postfix/Dovecot, Wildcard resolution)
- Roundcube webmail full server-side SSO bridge (Dovecot master password + autologin token)
- OLS website runtime:
  - managed vhost generation
  - listener map sync
  - `.htaccess` write-through
  - `open_basedir`
  - alias mapping
  - site suspend/unsuspend enforcement
  - site SSL binding to OLS vhost
  - tuning block read/apply
- ModSecurity / OWASP CRS:
  - installer support
  - runtime detection
  - WAF request inspection endpoint
- WordPress Manager: Full `wp-cli` integration for scans, plugin/theme updates, and deletions.
- CMS Installer: Async `wp-cli` auto-download, config generation, and installation.
- Reseller & Quota: System-level disk quota application via `setquota` / `xfs_quota`.

## Partial Runtime

- None remaining! (All mock/partial surfaces have been converted to real CLI bindings).

## Explicitly Not Implemented

- Any route that falls through to the generic fallback now returns `501 Not Implemented`.
- This is intentional: unsupported endpoints must fail honestly instead of returning fake success payloads.

## Rule

If a feature is not wired to the host, external API, or a deterministic managed file/config path, it must not be presented as active.
