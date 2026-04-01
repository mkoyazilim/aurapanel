# Performance Model

## Performance Priorities

AuraPanel optimizes for operational throughput, not just benchmark scores.

Core priorities:

- website traffic stability under panel activity
- predictable control-plane latency
- low overhead in critical host automation paths

## Practical Design Choices

- serving plane and control plane are separated
- Go services reduce runtime overhead
- gateway path is kept focused and explicit
- host actions use deterministic integrations over layered abstractions

## Throughput vs Safety

Performance is balanced with safety:

- validation and auth checks remain mandatory
- risky shortcuts that bypass guardrails are avoided
- tuning endpoints target bottleneck-heavy services only

## Capacity Planning Guidance

For higher scale environments:

- isolate panel components on dedicated nodes when needed
- split database and mail workloads based on traffic profile
- monitor service restart times and queue depth
- benchmark backup and migration windows regularly

## Tuning Targets

Operationally meaningful tuning areas:

- OpenLiteSpeed worker and cache settings
- MariaDB/PostgreSQL connection and memory parameters
- Redis memory policy and persistence settings
- mail queue performance and anti-abuse settings

## Anti-Patterns to Avoid

- coupling site serving to panel web process
- oversized dependency chains in runtime-critical endpoints
- silent retries without bounded timeouts
- untracked shell automation without structured result reporting
