# Architecture decisions

Record decisions according to the mandatory triggers and lifecycle rules in
[AGENTS.md](../../AGENTS.md). An existing decision may cover multiple changes;
link it rather than duplicating its rationale. A new decision records context,
alternatives, choice and reasons, consequences, status, and related documents.

An accepted design is not proof of implemented behavior. All decisions below
describe the MVP design, with implementation status shown individually. Library
versions beyond the standard library, the owner authentication mechanism,
management API payloads, schema, retry limits, and worker
limits remain pending for their implementation stages. Record those choices
before marking the corresponding implementation ready for review.

| ADR | Decisions covered | Status |
| --- | --- | --- |
| [0001](0001-single-process-postgresql.md) | Single process, component boundaries, Go HTTP/logging stack, PostgreSQL/pgxpool, goose migrations, transactional outbox | HTTP/logging implemented; storage and workers pending |
| [0002](0002-owner-access-and-public-targets.md) | Single-owner access, public target restriction, DNS/dial enforcement, redirects, TLS, sensitive diagnostics | Accepted design; implementation pending |
| [0003](0003-bounded-scheduling-and-check-semantics.md) | Probe contract, bounded scheduling, restart behavior, stale results | Accepted design; implementation pending |
| [0004](0004-incidents-and-notification-delivery.md) | Incident thresholds, pause/edit behavior, retention/deletion, Telegram ordering/retries and delivery limits | Accepted design; implementation pending |
| [0005](0005-application-foundation.md) | Go/tool versions, health/configuration contract, HTTP resource limits, shutdown lifecycle, safe logs and basic CI | Accepted; application foundation implemented |

The [MVP specification](../PLAN.md) owns exact product rules and acceptance
criteria. ADRs explain the choices and constraints; implementation PRs must add
behavioral tests rather than treating either document as execution evidence.
