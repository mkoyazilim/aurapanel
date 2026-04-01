# Operations Runbook

## Daily Health Checklist

- panel and gateway service status
- OpenLiteSpeed status and listener health
- database service health
- DNS and SSL operation success rate
- mail queue and delivery signals
- backup job success/failure summary

## Weekly Checks

- package and security update review
- failed login and auth anomaly review
- backup restore test (small sample)
- certificate expiry check
- service resource trend review

## Incident Handling Pattern

1. Detect and classify impact (control plane vs serving plane).
2. Stabilize affected integrations.
3. Collect logs and runtime state snapshots.
4. Apply fix with minimal blast radius.
5. Run post-fix acceptance checks.
6. Record root cause and prevention task.

## Change Management

Recommended path for production changes:

- use staging host before production rollout
- deploy with explicit version tracking
- run automated smoke checks after deploy
- maintain rollback artifact and procedure

## Backup and Recovery

- define RPO and RTO per tenant class
- backup website content + database + critical panel state
- encrypt backup storage where possible
- periodically validate restore viability

## Operational KPIs

Track and review:

- provisioning success rate
- SSL issuance success rate
- mean time to detect/repair
- failed operation ratio by module
- backup and restore reliability
