# Uptime Monitor

HTTP uptime monitoring for small websites and APIs, with incident history and
Telegram notifications.

**Status: application foundation.** The Go process serves a health endpoint,
validates environment configuration, writes structured logs, and shuts down on
signals. Monitoring, PostgreSQL storage, management APIs, readiness checks, UI,
and Telegram delivery are not implemented yet.

## Run locally

Install Go 1.27.0 (the version in `go.mod`), then run from the repository root:

```sh
go run ./cmd/uptime-monitor
```

In another terminal:

```sh
curl -i http://127.0.0.1:8080/healthz
```

The response is HTTP 200 with `Content-Type: application/json` and
`{"status":"ok"}`. `HEAD /healthz` returns the same status without a body.
Other methods return 405; request bodies are rejected with 413. Unknown paths,
including `/readyz` and management routes, return 404. Health only confirms that
the HTTP process can respond; it does not indicate monitoring readiness.

For a standalone executable:

```sh
go build -o bin/uptime-monitor ./cmd/uptime-monitor
./bin/uptime-monitor
```

On Windows, build with `go build -o bin/uptime-monitor.exe ./cmd/uptime-monitor`
and run `.\bin\uptime-monitor.exe`. Stop the executable with Ctrl+C; on Linux,
SIGTERM also triggers graceful shutdown. For signal checks, use the executable
rather than the `go run` wrapper.

## Configuration and limits

Configuration is read from the environment at startup; `.env` files are not
loaded automatically. Missing variables use defaults; explicitly empty or
invalid values fail startup with exit code 1 without echoing their values.

| Variable | Default | Accepted values |
| --- | --- | --- |
| `HTTP_ADDR` | `127.0.0.1:8080` | Literal IPv4/IPv6 plus port 1–65535, or `:port`; bracket IPv6, e.g. `[::1]:8080` |
| `SHUTDOWN_TIMEOUT` | `10s` | Go duration greater than zero and at most `1m` |
| `LOG_LEVEL` | `INFO` | `DEBUG`, `INFO`, `WARN`, `ERROR`, case insensitive |

For example, in PowerShell: `$env:HTTP_ADDR = ':8080'`; in a POSIX shell:
`export HTTP_ADDR=:8080`. This binds all interfaces instead of loopback.
Only the health endpoint is exposed at this stage; management authentication
will be implemented before management routes.

The server uses a 5s header timeout, 10s read timeout, 15s write timeout, 60s idle
timeout, and a 16 KiB header setting (Go's HTTP parser permits an additional read
buffer allowance). Health accepts no request payload, including chunked bodies.
Shutdown stops accepting connections and waits up to `SHUTDOWN_TIMEOUT` for active
requests. If the deadline expires, request contexts are canceled and connections
are closed; the process exits with code 1. Successful shutdown exits with code 0.

Logs are JSON events on stdout. Request URLs, headers, bodies, configuration
values, and raw transport errors are not logged. At restrictive log levels,
normal startup/shutdown events are suppressed.

## Development checks

```sh
gofmt -w cmd internal
go vet ./...
go test -timeout 60s ./...
go build ./...
```

On a supported system with a C compiler, also run `go test -race -timeout 60s ./...`.
GitHub Actions runs formatting, vet, race-enabled tests, and build on Linux for
pull requests and changes to `main`. It does not publish or deploy artifacts.
PostgreSQL integration checks will be added alongside storage.

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
The current foundation uses only the standard library. PostgreSQL and migration
dependency versions will be pinned when storage is implemented.

## Project documents

- [MVP specification and implementation plan](docs/PLAN.md)
- [Architecture decision](docs/adr/0001-single-process-postgresql.md)
- [Development instructions](AGENTS.md)

Management API examples will be added alongside working functionality.
