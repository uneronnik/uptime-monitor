## Result

Describe the problem and resulting behavior.

## Validation

List the checks actually executed, their results, and any verification limits.

## Documentation

Complete every row with changed file links or a specific reason it does not apply.
Distinguish implemented behavior from plans. Check off PLAN items only after
their acceptance criteria pass.

| Category | Updates and verification, or reason not applicable |
| --- | --- |
| README and examples: setup, configuration, API, available features | |
| PLAN: scope, acceptance criteria, verified progress | |
| AGENTS: commands, structure, development conventions | |
| ADR: decisions and rationale | |

For ADRs, review all triggers: dependencies/tools/services; component interaction
and data lifecycle; API/compatibility/migrations; authorization/network access;
concurrency/transactions/retries/delivery/recovery; future constraints.
List decisions with their ADR links, or explain why no trigger applies.

- [ ] Affected documents are updated in this PR; links, examples, and consistency were checked.
- [ ] Changed product rules are reflected in the specification and behavioral tests, or implementation is explicitly pending.
- [ ] No required documentation or applicable validation is missing; otherwise this PR remains Draft with the gaps listed.
