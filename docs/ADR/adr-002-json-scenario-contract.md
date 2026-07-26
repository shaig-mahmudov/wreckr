# ADR-002: Use JSON Scenario Files As The Core Contract

## Status
Accepted

## Context
A unified configuration contract is required for portability. Scenario files must stay easy to inspect, commit, copy, replay, and compile into other formats across the CLI, API, dashboard, and potential future backends like k6.

## Decision
Scenarios are versioned JSON documents with explicit target, traffic, request, threshold, and invariant sections.

## Consequences
- **Expected:** Scenario files remain highly portable and accessible.
- **Result (Observed):** The CLI loads scenario JSON directly, the API accepts inline scenarios and persisted scenarios, and the dashboard can edit sample scenario JSON today.

## Alternatives Considered

### Alternative 1: Create a custom DSL
- **Expected if chosen:** The language could be more expressive, but parsing, tooling, editor support, validation, and docs would cost more early in the project.

### Alternative 2: Store scenarios only through database forms
- **Expected if chosen:** The dashboard could guide users more tightly, but reproducibility and CLI-first workflows would be weaker.
