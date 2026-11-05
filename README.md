# Uptime Monitor

HTTP uptime monitoring for small websites and APIs, with incident history and
Telegram notifications.

**Status: PostgreSQL foundation.** The Go process validates configuration,
connects to PostgreSQL, serves health/readiness endpoints, writes structured
logs, and shuts down on signals. A separate migration command creates the
initial monitor table. Management APIs, probes, scheduling, incidents, UI, and
Telegram delivery are not implemented yet.

## Run locally

Install Go 1.27.0 and Docker with Compose. PostgreSQL 17.11 is pinned by image
digest in Compose and CI. From the repository root, in PowerShell:

```powershell
if (!(Test-Path .env)) { Copy-Item .env.example .env }
# For a new database, replace the placeholder password in .env.
# Preserve existing database credentials when a volume already exists.
docker compose up -d --wait db
$dbCredential = Get-Credential -UserName uptime -Message 'Local PostgreSQL password from .env'
$dbPassword = [uri]::EscapeDataString($dbCredential.GetNetworkCredential().Password)
$env:DATABASE_URL = "postgres://uptime:${dbPassword}@127.0.0.1:5432/uptime?sslmode=disable"
Remove-Variable dbCredential, dbPassword
go run ./cmd/migrate
go run ./cmd/uptime-monitor
```

The password prompt avoids putting the password itself in shell history. `.env`
is ignored by Git and used by Compose; Go reads only environment variables. The
example assumes default POSTGRES_PORT=5432; adjust DATABASE_URL if you change it.
Use URL encoding for credentials. `sslmode=disable` is for loopback development;
use verified TLS for remote PostgreSQL. Database ports bind only to loopback.
Changing POSTGRES_PASSWORD does not change credentials in an existing volume;
retain the original password or change it through PostgreSQL administration.

For POSIX shells, set DATABASE_URL securely in the environment, then use the same
Compose and Go commands. Do not commit credentials or paste connection strings
into reports.

In another terminal (use `curl.exe` on Windows PowerShell):

```sh
curl -i http://127.0.0.1:8080/healthz
curl -i http://127.0.0.1:8080/readyz
```

Health returns HTTP 200 JSON `{"status":"ok"}`. Readiness pings PostgreSQL with
a 2-second deadline and returns HTTP 200 JSON `{"status":"ready"}` or HTTP 503
JSON `{"status":"unavailable"}`. Both support HEAD without a body, reject other
methods with 405 and request bodies with 413, and disable caching. Unknown paths,
including management routes, return 404. Health checks only the HTTP process;
readiness checks database connectivity, not schema version or monitor execution.

For standalone executables:

```sh
go build -o bin/uptime-monitor ./cmd/uptime-monitor
go build -o bin/migrate ./cmd/migrate
./bin/migrate
./bin/uptime-monitor
```

On Windows, use `.exe` output names and run `.\bin\migrate.exe` followed by
`.\bin\uptime-monitor.exe`. Stop the executable with Ctrl+C; on Linux, SIGTERM
also triggers graceful shutdown. For signal checks, use the executable rather
than the `go run` wrapper.

## Configuration and limits

Configuration is read from the environment at startup; `.env` files are not
loaded automatically. Missing optional variables use defaults; explicitly empty
or invalid values fail startup with exit code 1 without echoing their values.
DATABASE_URL has no default and is required by both executables. The migration
command does not read HTTP_ADDR, SHUTDOWN_TIMEOUT, or LOG_LEVEL.

| Variable | Default | Accepted values |
| --- | --- | --- |
| `DATABASE_URL` | Required | Valid `postgres://` or `postgresql://` URL with host and database |
| `HTTP_ADDR` | `127.0.0.1:8080` | Literal IPv4/IPv6 plus port 1–65535, or `:port`; bracket IPv6, e.g. `[::1]:8080` |
| `SHUTDOWN_TIMEOUT` | `10s` | Go duration greater than zero and at most `1m` |
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR`, case insensitive |

For example, in PowerShell: `$env:HTTP_ADDR = ':8080'`; in a POSIX shell:
`export HTTP_ADDR=:8080`. This binds all interfaces instead of loopback.
Only health and readiness are exposed; management authentication will be
implemented before management routes.

The server uses a 5s header timeout, 10s read timeout, 15s write timeout, 60s idle
timeout, and a 16 KiB header setting (Go's HTTP parser permits an additional read
buffer allowance). Both routes reject request payloads, including chunked bodies.
The application must connect to PostgreSQL within 5 seconds before opening HTTP.
The pool has at most 10 connections and no required minimum, overriding URL pool
size parameters. Startup never runs migrations. A running process stays live
during a database outage and reports 503 readiness until PostgreSQL returns.
Shutdown drains requests within SHUTDOWN_TIMEOUT before closing the pool. Expiry
cancels request contexts, closes connections, and exits with code 1. Successful
shutdown exits with code 0.

Logs are JSON events on stdout. Request URLs, headers, bodies, connection strings,
and raw transport/driver/SQL errors are not logged. At restrictive log levels,
normal startup/shutdown events are suppressed.

## Migrations and persistent data

`go run ./cmd/migrate` applies pending embedded goose SQL migrations; rerunning it
does not recreate tables or remove rows. It accepts no arguments and exposes no
down/reset operation. It uses a 1-minute work deadline, a PostgreSQL advisory lock
(up to 30 retries at 1-second intervals), and bounded unlock cleanup (up to 5
retries). Applied migration files are immutable; add new numbered files instead.

The initial `monitors` table stores ID, name, URL, expected status, interval,
timeout, paused state, and timestamps. Defaults are 200, 60 seconds, 5000 ms,
and unpaused. Constraints enforce nonempty bounded text, status 100–599, interval
at least 30 seconds, and positive timeout below the interval. See
[ADR 0006](docs/adr/0006-postgresql-foundation.md) for exact storage limits.
No monitor-management endpoint is available. URL/destination validation will be
implemented before management/probes; SQL constraints alone do not enforce it.

Compose keeps application data in a named volume. `docker compose stop db` and
`docker compose up -d --wait db` preserve data. Do not use `down -v`, volume
removal, or database deletion to repair startup. The application is run on the
host in this increment; full application containerization remains planned.

## Development checks

The test profile starts a separate ephemeral PostgreSQL server. After creating
`.env` as above:

```powershell
docker compose --profile test up -d --wait test-db
$env:TEST_DATABASE_URL = 'postgres://uptime_test:uptime_test@127.0.0.1:5433/uptime_test?sslmode=disable'
gofmt -w cmd internal
go vet ./...
go test -count=1 -timeout 60s ./...
go build ./...
```

Adjust the URL if TEST_POSTGRES_PORT changes. TEST_DATABASE_URL must reference a
separate test database, never application data. Tests create random schemas and
remove only their own schemas. The test server uses tmpfs; its data is disposable.
Without TEST_DATABASE_URL, DB integration and DB-dependent process tests skip
locally. Missing configuration fails those tests in CI.

On a supported system with a C compiler, also run `go test -race -timeout 60s ./...`.
GitHub Actions provides the same PostgreSQL image and runs formatting, vet,
race-enabled unit/integration/process tests, and build on Linux for pull requests
and changes to main. It does not publish or deploy artifacts. Windows subprocess
tests cannot automate console Ctrl+C; Linux CI tests SIGTERM.

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
The current foundation pins pgx/v5 and goose/v3 in go.mod/go.sum.

## Project documents

- [MVP specification and implementation plan](docs/PLAN.md)
- [Architecture decision](docs/adr/0001-single-process-postgresql.md)
- [Development instructions](AGENTS.md)

Management API examples will be added alongside working functionality.
