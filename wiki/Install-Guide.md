# Install Guide

## 1. Supported Operating Systems

- Ubuntu 22.04 / 24.04
- Debian 12+
- AlmaLinux 8/9
- Rocky Linux 8/9

## 2. Minimum Host Profile

Recommended baseline for small production pilots:

- 2 vCPU
- 4 GB RAM
- 40+ GB SSD
- Public IPv4 (IPv6 optional)
- Root or sudo access

## 3. Network and DNS Checklist

Required before first deployment:

- Open ports: `22`, `80`, `443`
- For panel and OLS workflows: `7080`, `8090`
- If mail is enabled: `25`, `465`, `587`, `110`, `995`, `143`, `993`
- Hostname A/AAAA records must resolve correctly

## 4. Standard Installation

```bash
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo bash
```

## 5. Verified Bootstrap Installation

```bash
export AURAPANEL_RELEASE_BASE="https://github.com/mkoyazilim/aurapanel/releases/latest/download"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

Optional explicit manifest:

```bash
export AURAPANEL_MANIFEST_URL="https://example.com/releases/latest/aurapanel_release_manifest.env"
curl -fsSL https://raw.githubusercontent.com/mkoyazilim/aurapanel/main/install.sh | sudo -E bash
```

## 6. Post-Install Validation

- Verify API and panel services are active
- Login with generated/admin credentials
- Check OpenLiteSpeed runtime health
- Run SSL issuance on a test domain
- Confirm DNS record create/update operations
- If mail enabled, run send/receive smoke test

## 7. Upgrade Existing Host

```bash
cd /opt/aurapanel
bash scripts/deploy-main.sh
```

## 8. Rollback Strategy

Suggested rollback approach:

- snapshot VM or volume before upgrade
- backup panel env and key config files
- keep previous build artifact available
- use staged rollout host before fleet-wide rollout

## 9. Hardening After Install

- rotate initial credentials
- enforce SSH key login policy
- review firewall policy and close unused ports
- enable WAF profile (ModSecurity + OWASP CRS) where needed
- define backup retention and restore drills

## Related Pages

- [Operations Runbook](./Operations-Runbook.md)
- [Troubleshooting](./Troubleshooting.md)
- [Security Model](./Security-Model.md)
