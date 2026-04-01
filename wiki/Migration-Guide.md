# Migration Guide

## Migration Scope

Common migration sources:

- cPanel/WHM
- Plesk
- CyberPanel

Typical objects to migrate:

- websites and vhost mappings
- databases and users
- mailboxes and aliases
- DNS zones/records
- SSL assets

## Pre-Migration Audit

- inventory domains, databases, mailboxes, and custom configs
- identify unsupported or custom extension dependencies
- map PHP versions and required modules
- measure data volume and acceptable migration window

## Recommended Migration Phases

1. Discovery and inventory export
2. Pilot migration for low-risk tenants
3. Validation and SLA measurement
4. Batch migration by tenant/profile
5. Post-migration optimization and cleanup

## Validation Checklist

- DNS correctness (A/AAAA/MX/TXT/SPF/DKIM where relevant)
- website response integrity
- SSL validity and chain checks
- database application connectivity
- mailbox send/receive verification
- cron and scheduled task behavior

## Risk Controls

- keep source panel read-only during final sync window
- maintain rollback DNS plan
- migrate in controlled batches
- define cutover windows with stakeholder communication

## Post-Migration Hardening

- rotate imported credentials
- remove legacy insecure settings
- align firewall and service policies
- run security and acceptance test suite
