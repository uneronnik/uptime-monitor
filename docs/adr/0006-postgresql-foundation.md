# PostgreSQL foundation and explicit migrations

Status: accepted; implements the storage foundation. Extends ADR 0001 and
supersedes ADR 0005 only for standard-library-only dependencies, startup without
a database, and the absence of readiness. Other foundation decisions remain.

## Context

Stage 1 needs durable storage before authenticated management is introduced.
The first storage increment must prove repeatable migrations and preservation
of data without exposing an unauthenticated management API or prematurely
implementing scheduling and incident storage.

## Decision and reasons

Use pgx/v5 v5.12.0 and goose/v3 v3.29.0, pinned in go.mod/go.sum. Use PostgreSQL
17.11, with the same immutable image digest in Compose and Linux CI. PostgreSQL
17 is a deliberate supported major version; upgrades require verification and
an explicit image update. Compose runs only PostgreSQL in this increment; the
Go commands run on the host. Containerizing the application remains stage 5.

Require DATABASE_URL for both executables: a postgres/postgresql URL with a host
and database. Reject missing, empty, or invalid values without echoing them.
TLS and PostgreSQL authentication follow the URL/pgx configuration. Examples
disable TLS only for loopback development. Do not load .env in Go; Compose uses
it for local database settings. Do not supply a production connection default.

At API startup, create a pgxpool pool and ping within 5 seconds before binding
HTTP. Set a maximum of 10 connections and no minimum/idle minimum, overriding
URL pool-size settings. Per-connection establishment is limited to 5 seconds.
The process fails safely if PostgreSQL cannot be reached. Drain HTTP before
closing the pool so active database requests can finish or be canceled.

Expose GET/HEAD /readyz without authentication or user data. Ping PostgreSQL
using the request context with a 2-second maximum; return 200 with status ready
or 503 with status unavailable. Reject request bodies, never return driver
errors, and disable caching. Health stays independent of PostgreSQL. Readiness
reports connectivity only: it does not verify schema versions, permissions for
future management operations, or monitoring readiness. Run migrations explicitly
before use; startup never runs them.

Embed numbered SQL files in internal/postgres/migrations. cmd/migrate accepts
no arguments and only applies pending migrations through a goose provider,
using pgx's database/sql adapter and one connection. Each migration is
transactional. Serialize migration commands with goose's PostgreSQL session
advisory lock: retry every second, up to 30 retries. Limit the command's work to
one minute, honor signals, and allow up to 5 unlock retries at one-second
intervals during cleanup. Session termination releases any remaining lock.
Do not expose down/reset operations; applied files are immutable and later
changes use new numbered migrations. Return category-only errors to avoid SQL,
connection strings, or target URLs appearing in output.

The first migration creates monitors with a bigint identity ID, name (1–200
characters after trimming spaces), URL (1–2048 characters), expected status
(100–599, default 200), interval in whole seconds (30–2147483647, default 60),
timeout in whole milliseconds (1–2147483647 and strictly below the interval,
default 5000), paused (default false), and creation/update timestamps. Updates
must set updated_at explicitly; no trigger is installed. Arithmetic compares
timeout and interval as bigint to avoid integer overflow. URL length/nonempty
checks are storage constraints, not destination validation; the public-target
policy in ADR 0002 must be enforced before probes or management exist. No monitor
repository or HTTP CRUD is exposed yet. Scheduling, stale-work versions,
history, incidents, and outbox tables will arrive with their own behavior.

Compose binds PostgreSQL to loopback and keeps application data in a named
volume. An explicitly selected test profile provides a separate ephemeral
PostgreSQL server on another port. Test credentials are fixtures, not production
secrets. Integration tests use TEST_DATABASE_URL only, create random isolated
schemas, and drop only those schemas during bounded cleanup. Missing test DB
configuration can skip locally but fails in CI. Linux CI provides PostgreSQL
and runs integration, subprocess, and race checks; it does not deploy.

## Alternatives considered

- Automatically migrate during API startup: hides schema-changing operations
  inside normal process startup and complicates recovery and permissions.
- Create every future table now: commits to untested scheduling and incident
  details before their implementation.
- Introduce an in-memory repository: does not exercise the selected persistence
  guarantees and would be throwaway application storage.
- Use an unpinned image tag or a different database in tests: weakens parity.
- Readiness as a health alias or cached connection state: cannot detect a lost
  database connection after startup.
- Expose rollback: risks deleting monitor data; use forward migrations instead.

## Consequences and limitations

Existing startup commands now require PostgreSQL and DATABASE_URL. Availability
of PostgreSQL is required at startup; a running process stays live during a DB
outage and becomes ready again when connectivity returns. No application-level
retry loop is added. Pool limits and deadlines are intentionally fixed for this
single-instance increment. Schema constraints do not make direct SQL inserts
equivalent to validated public API requests. Full stage 1 remains unfinished
until management/authentication and its persistence behavior are implemented.

## Related documents

- [README](../../README.md)
- [Specification and verified progress](../PLAN.md)
- [Architecture](0001-single-process-postgresql.md)
- [Foundation decisions](0005-application-foundation.md)
- [Access and destination policy](0002-owner-access-and-public-targets.md)
