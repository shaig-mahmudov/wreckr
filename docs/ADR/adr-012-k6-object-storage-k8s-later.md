# ADR-012: Treat k6, Object Storage, And Kubernetes Jobs As Later Execution Layers

## Status
Accepted

## Context
During MVP development, Wreckr needed a coherent domain model, database persistence, run execution timeline, and an async worker core before adding heavier execution backends, cloud integrations, or distributed scaling systems.

## Decision
k6 script compilation, S3-compatible object storage, and Kubernetes job orchestration are scheduled as later execution/scaling layers after the core Go runner, persistence, events, and Redis worker path.

## Consequences
- **Expected:** Later orchestration and execution layers can wrap the same scenario and report models instead of forcing a major early redesign.
- **Result (Observed):** The core control plane stabilized first. Following this, the k6 script compiler and runner execution (Phase 4) and S3 Object Storage integration (Phase 2 follow-up) were introduced as additive modules around the unified Wreckr models.

## Alternatives Considered

### Alternative 1: Start with Kubernetes Jobs as the primary execution unit
- **Expected if chosen:** Isolation and scale would improve earlier, but local development and iteration speed would suffer.

### Alternative 2: Start with object storage and artifacts before reports/events
- **Expected if chosen:** Artifact retention would be ready sooner, but there would be fewer meaningful artifacts to store before the report and event model stabilized.
