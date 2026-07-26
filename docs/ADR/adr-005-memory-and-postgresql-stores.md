# ADR-005: Support Memory And PostgreSQL Stores Behind One Interface

## Status
Accepted

## Context
Pluggable storage is needed so that local development and unit tests are extremely fast with zero configuration, while production/control-plane deployments can run reliably with persistent records.

## Decision
Storage is abstracted behind a `Store` interface with memory and PostgreSQL implementations.

## Consequences
- **Expected:** The app can run in lightweight local mode or persistent mode without changing API behavior.
- **Result (Observed):** The same API-facing behavior works with both stores. PostgreSQL persists projects, targets, scenarios, scenario versions, runs, run events, and reports.

## Alternatives Considered

### Alternative 1: Use PostgreSQL only
- **Expected if chosen:** Fewer code paths would exist, but tests and local experiments would require more infrastructure.

### Alternative 2: Use in-memory storage only until later
- **Expected if chosen:** Early development would be faster, but async worker execution and historical run reporting would be fragile or impossible across processes.
