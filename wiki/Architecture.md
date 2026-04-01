# Architecture

## High-Level Flow

```text
Browser -> API Gateway -> Panel Service -> Host Integrations
```

AuraPanel is intentionally layered to isolate concerns.

## Layers

### Frontend

- Vue + Vite operator UI
- permission-aware workflows
- operational forms for website, SSL, DNS, mail, and services

### API Gateway

- authentication entry point
- RBAC enforcement and request controls
- central middleware chain (CORS, request IDs, auth guards)
- static panel serving in production

### Panel Service

- host operation orchestration
- integration dispatch for runtime modules
- deterministic config and command execution
- runtime probes and status surfaces

### Host Services

- OpenLiteSpeed
- MariaDB/PostgreSQL
- Postfix/Dovecot
- PowerDNS
- Redis
- Docker
- MinIO
- Other managed components

## Architectural Decisions

1. Decoupled serving path: website serving must not depend on panel process health.
2. Fail-closed endpoints: unsupported modules should explicitly report not implemented.
3. Deterministic automation: repeatable system actions over ad-hoc mutable state.
4. Layered trust boundaries: gateway and service roles are separated by design.

## Reliability Notes

- control plane outages should not interrupt active site traffic
- runtime operations should be observable and traceable
- long-running tasks should expose status and error context

## Extension Direction

- API-first surface for future worker/agent integrations
- gRPC-friendly boundaries for service decomposition
- GitOps-compatible declarative overlays for fleet operations
