# AuraPanel vs CyberPanel vs cPanel vs Plesk

This document compares panel choices through technical and operational lenses.

## Scope and Method

- Focus on architecture, control, and operations quality
- Avoid volatile licensing and pricing claims
- Evaluate practical migration and reliability concerns

## Technical Lens Comparison

| Lens | AuraPanel | CyberPanel | cPanel/WHM | Plesk |
|---|---|---|---|---|
| Core orientation | Decoupled control-plane architecture | OpenLiteSpeed-centric panel experience | Commercial shared hosting standard | Commercial integrated platform with extensions |
| Serving/control separation | Strongly emphasized | Moderate, deployment dependent | Usually integrated operations | Integrated + extension workflow |
| Runtime transparency | Host-backed operation honesty focus | Module-dependent | High-level abstractions | High-level abstractions + extensions |
| Security direction | Zero-trust and fail-closed defaults | Core hardening options | Strong enterprise controls in paid tiers | Strong controls via paid modules |
| Extensibility direction | API/gRPC-first roadmap | Plugin-centric | Commercial ecosystem integrations | Extension marketplace |

## Scenario Guidance

### Choose AuraPanel if

- you need deterministic and auditable operations
- you want strict architectural separation
- you are building long-term automation and GitOps workflows

### Choose CyberPanel if

- your team is strongly OpenLiteSpeed-first and needs quick bootstrap

### Choose cPanel if

- your organization is deeply standardized on cPanel workflows
- you require mature commercial shared-hosting ecosystem defaults

### Choose Plesk if

- you need broad extension compatibility across mixed workloads
- your team prefers extension-driven operational UX

## Migration Notes

- Run phased migration, not one-shot cutover
- Audit plugin/extension dependencies up front
- Validate DNS, SSL, mail, and DB connectivity in each phase
- Preserve rollback strategy until acceptance checks are complete
