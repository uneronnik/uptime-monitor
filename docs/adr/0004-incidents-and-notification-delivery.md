# Incident policy and notification delivery

Status: accepted design; implementation pending.

## Context

An isolated network failure should be visible without necessarily opening an
incident. Restart, configuration edits, and notification failures must not
silently erase the incident lifecycle or imply a recovery that was not observed.

## Decision and reasons

Separate last check outcome from incident state. Open one incident after three
consecutive failures and resolve it on the first success. This initial threshold
reduces alerts for isolated errors while keeping recovery prompt; it is a product
choice, not an experimentally established optimum.

Pause preserves an open incident; resume resets the failure streak. Changing the
target or expected status resets its observation state and closes an open
incident with a configuration-change reason, not recovery. Retain checks for
30 days and incidents until monitor deletion. Explicit monitor deletion removes
its history and pending events; the UI requires confirmation. This bounds detailed
history while preserving incident summaries for the monitor's lifetime.

Use optional Telegram delivery. Create notification events atomically with
incident transitions, with uniqueness per transition. Preserve per-monitor
event order, retry transient errors with bounded backoff, honor rate limits,
and expose permanent failures. Enabling Telegram does not replay historical
incidents. Exact retry budgets and treatment of permanently failed predecessor
events remain pending implementation decisions.

## Alternatives considered

- Alert on every failed check: creates repeated notifications for one outage.
- Close incidents on pause or edit as recovered: claims a success never observed.
- Send inside the incident transaction: cannot make the database commit atomic
  with an external Telegram request.
- Promise exactly-once delivery: cannot cover an accepted request with a lost
  response without external deduplication support.

## Consequences

Incident opening is delayed by the threshold. Sampled timestamps describe
observations, not exact outage boundaries. Notification duplicates remain possible
after ambiguous sends; database uniqueness prevents duplicate event creation,
not duplicate external delivery. Checks remain usable without Telegram.

## Related documents

- [Incident and notification specification](../PLAN.md)
- [Transactional outbox](0001-single-process-postgresql.md)
- [Scheduling semantics](0003-bounded-scheduling-and-check-semantics.md)
