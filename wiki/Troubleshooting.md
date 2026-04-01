# Troubleshooting

## 1. Panel Login Fails

Checks:

- confirm gateway and panel-service are running
- verify auth environment variables
- inspect service logs for token/credential errors
- validate firewall reachability for panel endpoint

## 2. SSL Issuance Fails

Checks:

- domain A/AAAA records point to correct host
- `80/443` reachability from public internet
- OpenLiteSpeed status and vhost mapping
- Let's Encrypt rate-limit or challenge errors

## 3. DNS Changes Not Applying

Checks:

- PowerDNS service health
- zone ownership and record syntax
- propagation delay and TTL expectations
- authoritative nameserver configuration

## 4. Mail Delivery Problems

Checks:

- Postfix/Dovecot service status
- MX/SPF/DKIM/DMARC correctness
- blocked ports (25/465/587/993/995)
- mailbox credential and quota state

## 5. Database Access Errors

Checks:

- MariaDB/PostgreSQL service status
- user grants and host allow-list rules
- database endpoint and credentials
- connection limit or resource exhaustion

## 6. Backup Job Fails

Checks:

- target storage reachability (local/MinIO)
- disk space and permissions
- job timeout configuration
- backup retention policy conflicts

## 7. Upgrade Regression

Response path:

1. isolate affected module
2. collect logs and runtime metadata
3. rollback using previous known-good artifact if needed
4. re-run acceptance test matrix

## 8. Escalation Package

Before opening an issue, prepare:

- affected module and operation
- expected vs actual behavior
- relevant logs (with secrets masked)
- environment info (OS, versions, network assumptions)
- exact reproduction steps
