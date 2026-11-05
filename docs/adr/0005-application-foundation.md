# Application foundation and operational contract

Status: accepted; implemented for the application foundation.

Partially superseded by [ADR 0006](0006-postgresql-foundation.md): the application
now requires PostgreSQL, adds pinned dependencies, and exposes readiness. The
original increment and its reasons below are retained as historical context.

## Context

The first executable increment needs reproducible builds, a small operational
surface, safe startup errors, and bounded shutdown before storage or management
exists. ADR 0001 selects standard HTTP/logging and the eventual PostgreSQL design;
ADR 0002 requires authenticated management and safe diagnostics.

## Decision and reasons

Use Go 1.27.0, matching the implementation toolchain, with only standard-library
dependencies. `cmd/uptime-monitor` validates configuration, establishes signal
handling, binds the listener, and runs `internal/httpapi`. `internal/config`
reads supplied environment lookups so tests do not need global environment state.
There is no placeholder domain, storage layer, or in-memory monitor repository.

Configure the bind address, shutdown timeout, and log level through environment
variables documented in README. Default to loopback. Restrict addresses to IP
literals (or an empty host) with a nonzero port, avoiding startup DNS dependencies.
Reject explicit empty values. Limit shutdown to a positive duration of at most
one minute to keep termination bounded. Do not load dotenv files implicitly.

Expose only exact `GET /healthz` (and HEAD) with a constant JSON response. This
unauthenticated route contains no user data and reports process responsiveness.
Reject payloads without reading them. Do not add a misleading readiness route
before storage dependencies exist. Management endpoints and authentication remain
pending under ADR 0002; none are exposed by this increment.

Use net/http timeouts: header 5s, read 10s, write 15s, idle 60s; set headers to
16 KiB, subject to Go's additional parser read-buffer allowance. There is no
global connection/concurrency cap in this increment; slow connections are bounded
by timeouts, and worker limits belong to later stages.

On SIGINT/SIGTERM, close the listener and gracefully drain active requests with
an independent shutdown context. Active request contexts are independent of the
signal context so ordinary shutdown does not cancel them early. At deadline,
cancel request contexts and close connections; return a failure status. Handlers
must honor cancellation; Go cannot forcibly terminate arbitrary handler goroutines.
The main process exits after the bounded server operation returns.

Write JSON slog events to stdout. Never log raw bind/server errors, request
details, or configuration values. Convert standard HTTP transport diagnostics to
a fixed category, trading detail for protection from input leakage. Configuration
errors include only the setting name and accepted form. No access log is enabled.

Add GitHub Actions for Linux formatting, vet, race-enabled tests, and build. Pin
checkout v4.2.2 and setup-go v5.5.0 to commit SHAs, with read-only contents access.
Use the Go version from go.mod and no dependency cache while no go.sum exists.
Run on pull requests and main pushes; no deployment, release, or publishing.
PostgreSQL integration checks must arrive with storage, as specified in PLAN.

## Alternatives considered

- Implement the entire management/storage stage at once: delays a separately
  verifiable executable and adds unrelated persistence/authentication decisions.
- Cancel all request contexts on the signal: interrupts requests that could
  complete during the drain period.
- Add readiness as an alias for health: confuses process responsiveness with
  future dependency availability.
- Use dotenv/config libraries or a framework: unnecessary dependencies for three
  settings and a single operational route.
- Log raw server errors: easier diagnosis, but risks exposing untrusted input.

## Consequences and limitations

This executable cannot monitor services yet. Production readiness, authentication,
storage, request-body limits for management payloads, and worker concurrency are
future increments. Their contracts must be implemented and tested before use.
Fixed HTTP limits are deliberately simple; changes require updating their
documented contract. Race checks require a C compiler and are run on Linux CI.
Process signal testing runs on Linux because Windows os.Process.Signal cannot
send a console Ctrl+C event; in-process shutdown tests run on both platforms.

## Related documents

- [README and commands](../../README.md)
- [Stage 1 and acceptance criteria](../PLAN.md#1-application-foundation-and-monitor-management)
- [Application architecture](0001-single-process-postgresql.md)
- [Access policy](0002-owner-access-and-public-targets.md)
