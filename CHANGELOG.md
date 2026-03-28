# Changelog

## 2026-03-26

### Added
- Website lifecycle parity updates including `/vhost/update` and advanced website controls.
- DNS reconciliation and DNSSEC workflow endpoints.
- Guided website/database flow endpoints.
- Mail operational endpoints for forwards, catch-all, routing, DKIM, and webmail SSO.
- SFTP parity endpoints (`list`, `delete`, `password reset`) and state-backed management.
- Backup restore endpoint and hardened storage/minio flow.
- Core route/service tests, gateway auth/proxy contract tests, frontend vitest smoke tests.
- CI workflow with build/test/security gates.
- API v1 contract freeze document: `docs/api_contract_v1.md`.

### Changed
- Frontend auth storage strategy hardened with session-default and optional persistent remember mode.
- eBPF event pipeline moved from static list to collector-backed flow.
- Server status frontend aligned with backend payload contract.
- CTO architecture decisions codified:
  - gateway-only production API entry with core loopback bind enforcement
  - active-passive federated topology defaults
  - internal MinIO as default backup target
  - fail-closed security policy enforcement in production startup

### Security
- Added CI secret scanning and dependency/static analysis gates.
