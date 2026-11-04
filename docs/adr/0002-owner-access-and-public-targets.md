# Owner access and public monitoring targets

Status: accepted design; implementation pending.

## Context

The application accepts URLs and makes outbound requests. Management access can
change targets, inspect history, and delete data. A single owner needs these
operations; arbitrary visitors do not.

## Decision and reasons

Require owner authentication for management API and UI. Missing credentials
must fail closed. Choose the exact authentication mechanism in an ADR before
implementing management endpoints; it is not decided by this record.

Support public HTTP/HTTPS targets only. Validate actual connection addresses,
including DNS results and IPv6 forms, and prevent proxy bypass of this policy.
Disable automatic redirects and retain TLS certificate verification. Reject
URL credentials and sanitize diagnostics so URLs, credentials, and response
bodies cannot leak through logs or error messages.

These restrictions keep owner-supplied targets from becoming unrestricted
access to the server's local network. Enforce them at the dial boundary because
an initial URL/DNS validation alone cannot prevent DNS rebinding.

## Alternatives considered

- Public unauthenticated management: exposes data and destructive operations.
- Registration and multiple owners: introduces account and tenancy contracts
  outside the single-owner MVP.
- Private network monitoring: useful in other products, but needs a different
  trust and network policy.
- Automatic redirects: convenient for websites, but obscures the exact status
  contract and adds destinations requiring validation.

## Consequences

Loopback, private-network targets, and insecure TLS endpoints are unsupported.
A redirect is evaluated as a response status rather than followed. Controlled
tests inject a transport; production has no switch to disable restrictions.
The precise address policy and denial cases require implementation tests.

## Related documents

- [Access and outbound contract](../PLAN.md#access-and-outbound-requests)
- [Application architecture](0001-single-process-postgresql.md)
