# Bounded scheduling and check semantics

Status: accepted design; implementation pending.

## Context

Slow targets, simultaneous due times, application downtime, and monitor edits
can otherwise create overlapping work, resource exhaustion, or stale results.

## Decision and reasons

Use a bounded worker pool and bounded queues, with at most one active check per
monitor. Store due times in PostgreSQL. Schedule the next check from completion;
after restart run one eligible check rather than replaying missed intervals.
Treat interruption by application shutdown as unknown rather than target failure.

Use GET, an exact expected status, and a positive timeout below the interval.
The initial defaults are 200, 60 seconds between checks, and a 5-second timeout;
the minimum interval is 30 seconds. These are initial product defaults, not
load-tested capacity claims. Bound response handling and propagate cancellation.

Pause, delete, and target edits invalidate outstanding work. Late results cannot
update a newer monitor configuration or recreate deleted state. The exact
versioning/transaction mechanism remains an implementation decision to record.

## Alternatives considered

- One unrestricted goroutine per due monitor: allows target volume to exhaust
  local resources.
- Fixed-rate catch-up after downtime: creates bursts and cannot reconstruct
  historical target availability.
- Overlapping checks: can reorder outcomes and corrupt failure streak meaning.
- Timing real external sites in tests: introduces nondeterminism and dependence
  on external network conditions.

## Consequences

Slow checks and queue pressure can delay observations. The configured interval
is a delay from completion, not a guaranteed wall-clock sampling frequency.
Use controlled clocks and local test servers through injected dependencies.
Distributed job ownership is outside the single-instance MVP.

## Related documents

- [Check contract](../PLAN.md#check-and-incident-contract)
- [Single-instance architecture](0001-single-process-postgresql.md)
- [Incident interpretation](0004-incidents-and-notification-delivery.md)
