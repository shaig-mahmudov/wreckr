# ADR-007: Persist Run Events And Stream With SSE

## Status
Accepted

## Context
Runs need an inspectable, granular timeline for lifecycle transitions, requests, assertions, thresholds, invariants, cancellations, and terminal states. Users must be able to observe this timeline in real time, but also inspect it historically for post-mortem debugging.

## Decision
Wreckr stores run events and streams live progress with Server-Sent Events (SSE).

## Consequences
- **Expected:** Users can watch live progress and still inspect the same timeline later with minimal frontend or transport-layer complexity.
- **Result (Observed):** The API exposes persisted events at `/v1/runs/{id}/events` and live streams at `/v1/runs/{id}/events/stream`; the dashboard displays a live event timeline.

## Alternatives Considered

### Alternative 1: Use WebSockets immediately
- **Expected if chosen:** Bidirectional control would be possible, but the implementation would be more complex before there is a clear need for client-to-server streaming.

### Alternative 2: Poll run status only
- **Expected if chosen:** The API would be simpler, but users would lose detailed execution context and failure diagnosis would be weaker.
