# ADR-003: Keep The Runner Core In Go

## Status
Accepted

## Context
High-concurrency load testing requires predictable execution, concurrency controls, efficient resource usage, context cancellation, and simple packaging.

## Decision
The scenario engine and HTTP runner are implemented in Go.

## Consequences
- **Expected:** The runner can execute multiple traffic modes with predictable concurrency behavior and simple operational packaging.
- **Result (Observed):** The MVP supports load, burst, spike, race, and retry-storm traffic modes, context cancellation, pacing, and structured reports with a compact codebase.

## Alternatives Considered

### Alternative 1: Build the runner in Node.js
- **Expected if chosen:** The dashboard and runner could share a TypeScript ecosystem, but high-concurrency execution and binary distribution would likely be less straightforward.

### Alternative 2: Use k6 as the only runner from day one
- **Expected if chosen:** High-scale HTTP workloads would arrive sooner, but Wreckr would have less control over the domain model, event timeline, cancellation behavior, and invariant evaluation.
