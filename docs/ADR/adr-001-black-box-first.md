# ADR-001: Build Wreckr Black-Box First

## Status
Accepted

## Context
The product goal is language-agnostic production scenario testing. A black-box runner can test Go, C#, Java, Node.js, Python, and other stacks without requiring SDKs or application code changes.

## Decision
Wreckr tests backends from the outside over HTTP before adding gray-box integrations.

## Consequences
- **Expected:** Teams can point Wreckr at a target service, run realistic pressure scenarios, and detect behavioral failures such as weak idempotency, bad rate limiting, and broken retry handling.
- **Result (Observed):** The MVP can run HTTP scenarios against the demo API and external targets, validate status expectations, thresholds, and invariants, and report failures without target-side instrumentation.

## Alternatives Considered

### Alternative 1: Start with language-specific SDKs
- **Expected if chosen:** Wreckr would have deeper in-process visibility, but adoption would be slower and every supported language would need custom maintenance.

### Alternative 2: Start with Kubernetes-only fault injection
- **Expected if chosen:** Wreckr would be closer to infrastructure chaos testing, but less useful for local development and less focused on business invariant failures.
