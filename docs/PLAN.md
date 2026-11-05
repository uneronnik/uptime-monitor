# Uptime Monitor MVP

Status: application and PostgreSQL foundations implemented. HTTP health/readiness,
environment validation, logging, bounded shutdown, a PostgreSQL pool, Compose,
and explicit embedded SQL migrations are available. Management, probes,
scheduling, incidents, and delivery remain unimplemented.

## Product scope

A single owner monitors small public HTTP/HTTPS services, inspects check history,
and receives Telegram notifications when an incident opens or resolves.
One application instance runs the API, scheduler, probe workers, and notification
worker. PostgreSQL is the durable source of truth.

Registration, teams, billing, multi-region probes, distributed scheduling,
custom request bodies/headers, and automatic redirect following are outside MVP.
Monitoring private networks is also outside MVP; demonstrations must use a public
test endpoint or an isolated test harness with an injected transport.

## Check and incident contract

| Setting or event | MVP behavior |
| --- | --- |
| Method | GET over HTTP or HTTPS; no automatic redirects |
| Expected result | Exact configured HTTP status code; default 200 |
| Interval | Minimum 30 seconds; default 60 seconds |
| Timeout | Positive duration below the interval; default 5 seconds |
| New monitor | No data until its first completed check; eligible immediately |
| Success | Expected HTTP status received within the deadline |
| Failure | Unexpected status, timeout, DNS/TLS/network error, or blocked destination |
| Incident opens | Third consecutive failed check; one open incident per monitor |
| Incident resolves | First successful check after opening |
| Before an incident | A success resets the consecutive failure count |
| Pause | Stops new checks; does not resolve an incident or imply availability |
| Resume | Eligible immediately; reset failure streak, preserve any open incident |
| Check retention | 30 days |
| Incident retention | Kept independently of check retention until the monitor is deleted |

Keep latest check outcome separate from incident state: the first two failures
are visible even though they have not opened an incident. Record first failure,
incident opening, and recovery timestamps separately. Display observed incident
duration; do not present sampled results as exact real-world downtime.

Scheduling uses persisted due times, bounded concurrency, and at most one active
check per monitor. Advance the next due time from completion; do not overlap
checks or replay every missed interval after downtime. An interrupted check is
not evidence that the target failed. After restart, overdue monitors are eligible
for one new check, subject to worker limits.

Pause, delete, and target edits invalidate outstanding work so late results cannot
change the current state or recreate deleted data. Editing the URL or expected
status resets the failure streak and latest outcome. If an incident is open,
close it with a distinct configuration-change reason, never as a recovery.
An interval-only edit preserves incident state. Deletion explicitly removes that
monitor's history and pending notifications; require confirmation in the UI.

## Notifications

Persist an incident transition and its notification event in one transaction.
A database uniqueness constraint prevents duplicate events for the same
transition. Deliver opening and recovery events in order for each monitor.

Retry transient failures with bounded backoff; honor rate-limit retry guidance.
Permanent failures remain visible for diagnosis without an infinite hot loop.
Delivery state survives restart. An accepted Telegram request whose response is
lost can produce a duplicate after retry; exactly-once delivery is not promised.
Telegram configuration is optional: checks and incidents work without it, and
disabled delivery is visible. Enabling it does not replay historical incidents.

## Access and outbound requests

Management APIs and UI require owner authentication. Specify and test the exact
mechanism before implementing those interfaces; missing credentials must fail
closed. No public registration. Health responses expose no secrets or user data.

Permit HTTP/HTTPS public destinations only. Reject URL credentials and local,
private, link-local, multicast, unspecified, and other non-public destination
addresses, including IPv6 and IPv4-mapped forms. Validate actual dial addresses
to prevent DNS rebinding; do not let a default proxy bypass the destination policy.
Keep TLS certificate verification enabled. Bound request/response resources and
sanitize logs and user-facing errors.

## Implementation stages

Each independently reviewable increment uses a branch and PR. A stage may need
several PRs; it is not complete until its acceptance criteria are demonstrated.
Every increment also follows the documentation completion rules in
[AGENTS.md](../AGENTS.md): update affected documentation in the same PR, record
decisions covered by the ADR triggers, and report applicability and verification
in the PR description. Check off work only after its acceptance criteria pass.
The [ADR index](adr/README.md) maps the current design to its rationale.

### 1. Application foundation and monitor management

- [x] Pin Go; add configuration validation, logging, health behavior, bounded
  request handling, and graceful shutdown using only the standard library.
- [x] Pin dependency versions and add database-connectivity readiness checks.
- [x] Add PostgreSQL, Compose, a standalone goose migration command, and the
  initial monitor schema.
- [ ] Add monitor storage operations alongside authenticated management.
- [ ] Define owner authentication in an ADR; implement authenticated monitor
  create/read/update/pause/resume/delete endpoints with input and body limits.
- [ ] Add CI with formatting, vet, tests, PostgreSQL integration tests, and race detection.

Acceptance: invalid configuration fails safely; unauthorized requests are denied;
CRUD and pause/resume persist across restart; migrations are repeatable without
data loss; integration tests execute in CI. No probes run in this stage.

Implemented foundation contract: Go 1.27.0, `HTTP_ADDR` (default loopback port
8080), `SHUTDOWN_TIMEOUT` (default 10s, positive and at most 1m), and `LOG_LEVEL`
(DEBUG/INFO/WARN/ERROR). Explicit empty values are invalid. Bind addresses use
literal IPs or an empty host and ports 1–65535. Errors never echo input values.
`GET /healthz` returns 200 JSON `{"status":"ok"}`; HEAD omits the body, unsupported
methods return 405, and request bodies (including unknown-length bodies) return
413. Unknown paths return 404.
HTTP limits and executable commands are documented in the [README](../README.md).
SIGINT/SIGTERM stop new connections and drain requests within the configured
deadline; expiry cancels request contexts, closes connections, and exits with
failure. Tests cover validation, health, header limits, request draining,
forced shutdown, and process startup; Linux additionally checks SIGTERM.
Linux CI now provisions PostgreSQL for mandatory integration/process checks.
See [ADR 0005](adr/0005-application-foundation.md) for decisions and limitations.

Implemented PostgreSQL contract: DATABASE_URL is required, valid, and never
echoed; startup pings within 5 seconds before opening HTTP. The pool has at most
10 connections. GET/HEAD /readyz pings PostgreSQL with a 2-second maximum and
returns 200 ready or 503 unavailable JSON, without driver details; it does not
validate schema versions. Health stays independent. Both routes reject bodies.
The application does not apply migrations or expose management endpoints.

cmd/migrate applies pending embedded goose SQL with a one-minute work deadline,
transactional migrations, session advisory locking, and bounded unlock cleanup.
No destructive command is exposed. The initial monitors table has identity ID,
bounded nonempty name/URL, status 100–599 (default 200), interval >=30 seconds
(default 60), positive timeout below interval (default 5000 ms), paused=false,
and timestamps. Exact representation limits are in
[ADR 0006](adr/0006-postgresql-foundation.md). Destination validation and all
monitor operations remain pending; SQL URL constraints are not an SSRF policy.

Integration tests use a separate database and random isolated schemas. They
cover migration reapplication with preserved rows, reconnection, defaults and
constraints, transactional rollback/retry, concurrent migrations, and lock
cancellation. A missing TEST_DATABASE_URL skips locally and fails in CI.
Stage 1 remains incomplete until authenticated CRUD and pause/resume persistence
meet the full acceptance criteria.

### 2. Probe execution and scheduling

- [ ] Implement destination restrictions, status checks, timeout/cancellation,
  TLS verification, no-follow redirect behavior, and bounded response handling.
- [ ] Add persisted due times, bounded workers, result history, stale-work
  rejection, and bounded shutdown.

Acceptance: controlled test servers reproduce success, wrong status, slow
responses, and connection failures. Tests prove concurrency limits, no overlap,
restart recovery without catch-up floods, rejection of unsafe DNS/dial addresses,
and safe pause/edit/delete during active checks. CI race checks pass.

### 3. Incident lifecycle

- [ ] Implement failure streaks and atomic incident transitions.
- [ ] Expose latest outcomes and paginated check/incident history.

Acceptance: failure sequences of lengths 1, 2, and 3, intervening successes,
repeated failures, recovery, pause/resume, configuration changes, and restart are
covered. Database constraints prevent two open incidents for one monitor.

### 4. Telegram delivery

- [ ] Add durable notification events, delivery worker, bounded retries,
  per-monitor ordering, and visible delivery failure state.

Acceptance: mock Telegram responses cover success, rate limits, transient and
permanent errors, ambiguous responses, and restart. Tests verify atomic event
creation and no duplicate event creation when a transition is processed again.

### 5. Complete product and operational verification

- [ ] Add a small authenticated UI for monitors, last results, incidents, and
  delivery state; document setup and configuration from implemented behavior.
- [ ] Add bounded check-history cleanup and internal operational metrics.
- [ ] Build a container running without root; verify complete Compose startup,
  migrations, shutdown, and persistence without deleting volumes.
- [ ] Demonstrate a controlled outage and recovery against a public test service.

Acceptance: the owner can add, inspect, pause, resume, and remove a monitor;
observe failure/recovery and Telegram events; restart without losing history or
pending events; and follow a verified README setup. No credentials appear in
logs, responses, screenshots, or committed examples. Deployment is a separate action.

## Completion boundary

The MVP is complete when all five stages meet their acceptance criteria and the
end-to-end scenario is verified. Further features require an explicit product
need; the roadmap is not authorization to implement unrelated extensions.
