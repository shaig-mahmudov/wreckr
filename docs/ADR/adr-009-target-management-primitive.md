# ADR-009: Introduce Target Management As A Reusable Control-Plane Primitive

## Status
Accepted

## Context
Hardcoding environments and base URLs inside individual scenario files leads to a massive amount of duplicate JSON/YAML definitions. It forces users to recreate scenarios just to run the exact same behaviors against different environments (e.g., local, dev, staging, or production).

## Decision
Targets are modeled as first-class, reusable records with names, base URLs, environments, descriptions, headers, and run-time resolution through `target_id`.

## Consequences
- **Expected:** Users can define an environment once, select it at run time, and keep scenario files completely focused on behavioral specifications.
- **Result (Observed):** The API supports target CRUD and target resolution. The dashboard can create, edit, list, delete, and select targets. API tests cover target CRUD and `target_id` run resolution.

## Alternatives Considered

### Alternative 1: Keep target URLs only inside scenario JSON
- **Expected if chosen:** The schema would stay simpler, but environment reuse and dashboard workflows would be clumsy.

### Alternative 2: Store targets only in environment variables
- **Expected if chosen:** Deployment configuration would be simpler, but users could not manage targets through the API or dashboard.
