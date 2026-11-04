# Uptime Monitor

HTTP uptime monitoring for small websites and APIs, with incident history and
Telegram notifications.

**Status: planning.** This repository currently contains the project specification
and development guidelines. The application, database schema, UI, and CI are not
implemented yet; there is no runnable service at this revision.

## Planned workflow

1. Add an HTTP/HTTPS URL, expected status code, interval, and timeout.
2. Run scheduled checks with bounded concurrency and record their results.
3. Open an incident after three consecutive failures and notify the owner.
4. Close the incident after a successful check and send a recovery notification.
5. Inspect recent checks and incident history in a small web interface.

The MVP serves one owner and runs a single application instance. Management
access will require authentication. Public registration, multi-region checks,
and distributed scheduling are outside its scope.

## Planned architecture

One Go application hosts the HTTP API, scheduler, check workers, and notification
worker. PostgreSQL stores monitors, check results, incidents, scheduling state,
and pending notifications. A durable notification queue in PostgreSQL avoids an
additional message broker.

The initial stack is Go's standard `net/http`, `log/slog`, PostgreSQL with
`pgxpool`, SQL migrations through goose, and Docker Compose for local operation.
Dependencies and toolchain versions will be pinned when implementation starts.

## Project documents

- [MVP specification and implementation plan](docs/PLAN.md)
- [Architecture decision](docs/adr/0001-single-process-postgresql.md)
- [Development instructions](AGENTS.md)

Setup commands and API examples will be added alongside working functionality.
