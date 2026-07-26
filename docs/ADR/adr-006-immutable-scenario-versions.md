# ADR-006: Make Scenario Versions Immutable

## Status
Accepted

## Context
When running scenario load tests, historical reports must remain completely reproducible and understandable over time. If a scenario is updated, existing reports from old runs should still describe exactly what ran, without being mutated or invalidated by subsequent scenario edits.

## Decision
Scenario updates create new immutable scenario version records, and runs keep version IDs plus scenario snapshots.

## Consequences
- **Expected:** Historical reports continue to describe exactly what executed, preserving audit and test integrity.
- **Result (Observed):** Runs link to the scenario version used at creation, reports retain version metadata, and old reports are not mutated by later scenario edits.

## Alternatives Considered

### Alternative 1: Mutate scenarios in place without versions
- **Expected if chosen:** The data model would be simpler, but historical reports could become misleading after edits.

### Alternative 2: Store only full snapshots and no version records
- **Expected if chosen:** Reproducibility would survive, but users would lose clear scenario revision history and diffable version lists.
