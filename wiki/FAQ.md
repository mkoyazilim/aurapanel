# FAQ

## Is AuraPanel production ready?

AuraPanel is designed for production-oriented workflows. Always validate in staging and run acceptance checks before broad rollout.

## Which web server model is used?

AuraPanel is aligned with OpenLiteSpeed serving workflows while keeping serving and control concerns separated.

## Can websites stay online if panel services restart?

That is a core architectural goal. Site serving should continue through the web server path even when control-plane services are restarted.

## Does AuraPanel support multi-tenant hosting?

Yes. Role-based access and operational boundaries are core design goals for tenant-aware environments.

## Is migration from cPanel/Plesk/CyberPanel possible?

Yes, via phased migration strategy. Start with inventory, pilot tenants, and strict validation checkpoints.

## Does AuraPanel include mail, DNS, and SSL workflows?

Yes. The platform includes runtime integrations for these areas, with verification flows recommended after each operation.

## How should updates be managed?

Use staged rollout, run smoke checks, and keep rollback artifacts available for high-safety operations.
