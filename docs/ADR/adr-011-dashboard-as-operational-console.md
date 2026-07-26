# ADR-011: Build The Dashboard As An Operational Console, Not A Landing Page

## Status
Accepted

## Context
Wreckr is fundamentally an operational developer tool. When opening the web interface, the primary user need is executing load tests, editing scenario contracts, inspecting run event streams, and resolving targets. A marketing-style home page introduces friction by forcing extra navigation.

## Decision
The Next.js dashboard app opens directly into the Wreckr console instead of a landing page.

## Consequences
- **Expected:** The UI feels useful immediately and supports repeated developer execution/monitoring workflows without unnecessary navigational layers.
- **Result (Observed):** The dashboard includes API connectivity, scenario JSON editing, target selection and management, run list, report metrics, failures, timeline, and raw JSON inspection.

## Alternatives Considered

### Alternative 1: Build a marketing-style homepage first
- **Expected if chosen:** The product would look more polished externally, but users would need extra navigation before doing real work.

### Alternative 2: Build only raw API docs first
- **Expected if chosen:** Backend workflows would be clear for developers, but non-API users would have no usable control-plane experience.
