# Wreckr Project Status

Last updated: 2026-06-12

## Current State

Wreckr is now a working MVP for language-agnostic backend scenario testing. It can define scenarios, run black-box HTTP traffic against a target, validate technical thresholds and business invariants, persist run history, and report production-style failures.

## Implemented

- Go HTTP runner with `load`, `burst`, `spike`, `race`, and `retry_storm` traffic modes.
- Setup and teardown hooks.
- HTTP status expectation validation.
- Business invariants:
  - response count checks
  - HTTP probe checks with JSON path assertions
- Threshold validation:
  - max error rate
  - p95 latency
- Request-rate pacing with `traffic.rate_per_second`.
- Context timeout handling for safe run shutdown.
- Run cancellation through `POST /v1/runs/{id}/cancel`.
- Partial canceled reports saved after cancellation.
- Scenario create, update, list, version list, run create, run status, and report endpoints.
- Immutable scenario versioning.
- Memory store and PostgreSQL store behind a shared `Store` interface.
- PostgreSQL migrations for projects, targets, scenarios, scenario versions, runs, run events, and reports.
- Guardrails for:
  - maximum concurrency
  - maximum request rate
  - maximum run duration
  - maximum request body size
  - target allowlist
  - unsafe target URL rejection
- Prometheus-style API metrics at `/metrics`.
- Next.js dashboard that connects to the API, launches sample scenarios, lists runs, and displays report details.
- GitHub Actions CI for backend tests, backend vet, frontend build, and Docker Compose config validation.
- Docker Compose stack for API, dashboard, demo API, PostgreSQL, Redis, migrations, and Prometheus.

## Storage

The API can run with either storage backend:

- `WRECKR_STORE=memory` for local ephemeral testing.
- `WRECKR_STORE=postgres` for persistent scenario, run, report, and scenario-version history.

PostgreSQL integration tests are opt-in through `WRECKR_TEST_DATABASE_URL`.

## Current API Surface

- `GET /healthz`
- `GET /metrics`
- `GET /v1/scenarios`
- `POST /v1/scenarios`
- `PUT /v1/scenarios/{id}`
- `GET /v1/scenarios/{id}`
- `GET /v1/scenarios/{id}/versions`
- `GET /v1/runs`
- `POST /v1/runs`
- `GET /v1/runs/{id}`
- `GET /v1/runs/{id}/report`
- `POST /v1/runs/{id}/cancel`

## Verification

The repository currently has automated coverage for:

- HTTP API integration behavior.
- Scenario version immutability.
- PostgreSQL persistence behavior.
- Runner traffic modes.
- Retry storm behavior.
- Race/idempotency detection.
- Hook ordering.
- Threshold failures.
- Context timeout behavior.
- Run cancellation.
- Guardrail enforcement.
- Request-rate pacing.

## Main Gaps

- Runner execution is still in-process inside the API.
- Redis is present in Docker Compose, but Asynq job orchestration is not implemented yet.
- k6 script generation is not implemented yet.
- Object storage and report artifact retention are not implemented yet.
- OpenTelemetry tracing is not implemented yet.
- Dashboard does not yet include a full scenario editor, live event stream, artifact viewer, or project/target management.
- Non-HTTP protocols such as gRPC, WebSocket, NATS, Kafka, and queue replay are still planned.
- Secrets management, redaction, quotas, audit logs, and production-target warning labels are still planned.

## Recommended Next Milestones

1. Add Redis + Asynq worker orchestration so API requests enqueue runs instead of executing them in-process.
2. Add run event streaming and persist per-run event timelines.
3. Add a scenario editor and target management to the dashboard.
4. Add object storage for raw reports, logs, generated scripts, and artifacts.
5. Add OpenTelemetry traces and richer Prometheus metrics.
6. Add k6 compilation for higher-scale HTTP workloads.
