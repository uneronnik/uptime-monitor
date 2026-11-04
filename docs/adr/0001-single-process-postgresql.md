# Single application process with durable PostgreSQL state

Status: accepted for the initial MVP design; HTTP/logging foundation implemented
under [ADR 0005](0005-application-foundation.md). Storage and background components
remain pending.

## Context

The service must run scheduled HTTP checks, retain incident history, and deliver
notifications after transient failures and application restarts. Its initial
scope is one owner and one active application instance.

## Decision

Use one Go application for HTTP management, scheduling, bounded check workers,
and notification delivery. Use PostgreSQL for monitors, scheduling state, check
results, incidents, and notification delivery state.

Keep business rules separate from HTTP, SQL, and Telegram adapters. Use standard
`net/http`, `log/slog`, and `pgxpool`. Apply versioned SQL migrations through a
separate goose command with embedded SQL; do not run migrations implicitly at API
startup or modify migrations already applied.

Write incident transitions and notification events in the same database
transaction (transactional outbox). The notification worker retries pending
events. Persisted event uniqueness does not guarantee exactly-once delivery to
an external API after an ambiguous network outcome.

## Consequences

Local operation needs only the application and PostgreSQL. No message broker or
distributed coordination service is required. In-memory work can be discarded
on shutdown; durable scheduling and notification state allow recovery.

Only one application instance may be active. Running additional replicas would
require explicit job ownership and coordination, and is not supported by MVP.
Bounded goroutines control concurrency within the process; they do not make
the scheduler distributed. Availability of the monitoring application itself
limits what outages can be observed.

Use GitHub Flow for changes. Automated checks validate changes; publishing
containers, deploying the service, and merging PRs remain separate operations.

## Alternatives considered

- Separate API, scheduler, and worker services with a message broker: adds
  deployment and coordination requirements before the single-owner MVP needs them.
- In-memory scheduling and notification state: simpler initially, but loses work
  and incident context across restarts.
- A framework router or an external migration executable: unnecessary for the
  initial REST scope; standard HTTP tooling and a Go migration command keep the
  toolchain small. pgxpool provides PostgreSQL connection pooling; goose manages
  migration versions rather than introducing custom migration tracking.

## Related documents

- [Product specification](../PLAN.md)
- [Decision index](README.md)
- [Access and outbound request policy](0002-owner-access-and-public-targets.md)
- [Scheduling and check semantics](0003-bounded-scheduling-and-check-semantics.md)
- [Incident and notification policy](0004-incidents-and-notification-delivery.md)
