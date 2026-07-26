# ADR-008: Add Guardrails Before Production-Target Features

## Status
Accepted

## Context
Wreckr intentionally generates extreme failure pressure. If safety controls are absent, accidental high-traffic runs could overwhelm production dependencies, trigger massive bills, scan internal networks (e.g. AWS metadata services), or violate security boundaries.

## Decision
The API validates scenarios with strict guardrails for concurrency, request rate, run duration, body size, target allowlists, metadata-service blocking, and unsafe URLs.

## Consequences
- **Expected:** Unsafe or excessive runs are rejected early with clear, actionable errors.
- **Result (Observed):** Guardrail tests cover max concurrency, max request rate, max duration, max request body size, target allowlist behavior, credentials rejection, metadata protection, and absolute URL validation.

## Alternatives Considered

### Alternative 1: Trust users and document warnings only
- **Expected if chosen:** The product would be easier to build, but accidental harmful runs would be more likely.

### Alternative 2: Allow only localhost targets in the MVP
- **Expected if chosen:** Safety would improve, but realistic staging and shared development workflows would be blocked.
