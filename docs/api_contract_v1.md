# AuraPanel API v1 Contract Policy

## Scope
- Base path: `/api/v1`
- Actors: AuraPanel frontend, external automation clients, internal services through gateway.

## Freeze Rules
1. Existing `v1` endpoint paths cannot be removed or renamed.
2. Existing response fields cannot be removed or have incompatible type changes.
3. New fields must be additive and backward compatible.
4. Breaking changes require a new version namespace (`/api/v2`) and migration guide.

## Change Workflow
1. Open a PR with explicit contract change notes.
2. Update `CHANGELOG.md` under the current release section.
3. If the change is breaking, keep `v1` behavior and add `v2` route in parallel.
4. Keep gateway route mapping aligned with core route behavior.

## Deprecation Policy
1. Mark deprecated fields/endpoints in changelog.
2. Keep deprecated contract active for at least one minor release cycle.
3. Provide replacement endpoint/field and migration example before removal.
