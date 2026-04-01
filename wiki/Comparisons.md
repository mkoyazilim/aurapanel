# Comparisons

This page provides a technical comparison between AuraPanel and commonly used control panels.

## Comparison Philosophy

- Focus on architecture and operations, not marketing claims.
- Avoid volatile pricing/licensing statements.
- Evaluate based on production reliability, security posture, and extensibility.
- Prefer stable technical criteria that remain valid across release cycles.

## Evaluation Lenses

- Control-plane design and isolation model
- Runtime transparency and observability
- Security defaults and privilege boundaries
- Extensibility path and automation friendliness
- Migration friction and operational risk

## Quick Matrix

| Area | AuraPanel | CyberPanel | cPanel/WHM | Plesk |
|---|---|---|---|---|
| Primary orientation | Decoupled control plane, explicit runtime state | OpenLiteSpeed-centric operations | Commercial shared-hosting standard | Commercial platform with broad extension model |
| Control/serving separation | Strongly emphasized | Partial, deployment-dependent | Generally integrated workflows | Integrated platform + modular extensions |
| Operational transparency | High emphasis on host-backed endpoint honesty | Moderate, module-dependent | Mature abstractions, less raw host visibility | Abstraction-heavy but feature rich |
| Security model direction | Zero-trust defaults, fail-closed behavior | Basic hardening toolset | Enterprise controls via paid tiers | Strong controls via paid features/extensions |
| Extensibility direction | API/gRPC-first, GitOps-friendly roadmap | Plugin and OLS-focused | Mature commercial ecosystem | Large extension marketplace |
| Best fit | Teams prioritizing transparent operations and custom automation | OLS-focused fast bootstrap teams | Standardized shared hosting businesses | Mixed workload teams needing broad integration catalog |

## Scenario-Based Guidance

### Choose AuraPanel when

- you want strict operational clarity over hidden abstraction
- you need control-plane/serving-plane separation as a hard requirement
- you plan to evolve toward API/gRPC and GitOps workflows
- you value deterministic automation and fail-closed behavior

### Choose CyberPanel when

- OpenLiteSpeed-centric workflow is the primary priority
- faster initial bootstrap matters more than deep custom control

### Choose cPanel/WHM when

- you need a mature commercial ecosystem and standard hosting workflows
- your team already has strong cPanel operational familiarity

### Choose Plesk when

- you need broad extension coverage and multi-workload convenience
- you value an integrated commercial control surface with vendor add-ons

## Migration Considerations

From CyberPanel/cPanel/Plesk to AuraPanel:

- map feature dependencies first (especially extensions/plugins)
- migrate in phases, not in one cutover
- validate DNS, SSL, mail, and DB connectivity in each phase
- keep rollback path active until acceptance criteria pass

## Strategic Positioning Summary

AuraPanel is built for teams that prioritize deterministic, observable, and security-first operations over legacy panel familiarity.

## Related Pages

- [Migration Guide](./Migration-Guide.md)
- [Architecture](./Architecture.md)
- [Operations Runbook](./Operations-Runbook.md)
