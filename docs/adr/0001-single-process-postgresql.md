# Single application process with durable PostgreSQL state

Status: accepted for the initial MVP design; implementation pending.

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
