# Wreckr Project Status

Last updated: 2026-06-16

## Current State

Wreckr is a working MVP for language-agnostic backend scenario testing. It can define black-box HTTP scenarios, run production-style traffic against a target, validate technical thresholds and business invariants, persist run history, and report failures through both an API and a dashboard.

The current execution model has two paths:

- CLI runs execute in-process from a local scenario file.
- API-created runs are persisted as `queued`, enqueued to Redis with Asynq, and executed by the worker process.

The Docker Compose stack is the most complete local environment. It starts the API, worker, dashboard, demo API, PostgreSQL, Redis, migrations, and Prometheus.

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
- Run cancellation for in-process API runs, queued worker runs before execution, and worker-owned running jobs through durable cancellation requests.
- Partial canceled reports saved after cancellation.
- Scenario create, update, get, list, and version list endpoints.
- Target create, update, get, list, and delete endpoints.
- Run create, list, status, report, event timeline, live event stream, and cancellation endpoints.
- Target resolution through `target_id`, including target-level headers merged with scenario headers.
- Persistent run event timelines for lifecycle, hook, request, assertion, threshold, invariant, cancellation, and terminal-state events.
- Worker attempt, retry-scheduled, and dead-letter-exhausted events persisted into run timelines for Asynq-owned runs.
- Server-Sent Events stream for live run progress.
- Redis + Asynq orchestration for API-created runs.
- Separate worker process for queued run execution.
- Deterministic k6 script compiler and execution integration (K6Runner) for generating scripts, executing them, parsing summary JSON files, and importing normalized Wreckr reports.
- Immutable scenario versioning.
- Memory store and PostgreSQL store behind a shared `Store` interface.
- PostgreSQL migrations for projects, targets, scenarios, scenario versions, runs, run events, and reports.
- Guardrails for:
  - maximum concurrency
  - maximum request rate
  - maximum run duration
  - maximum request body size
  - target allowlist
  - metadata-service target blocking by default
  - unsafe target URL rejection
- Prometheus-style API metrics at `/metrics`.
- Next.js dashboard that connects to the API, edits sample scenario JSON, selects targets for runs, launches runs, lists runs, displays report metrics, shows failures, opens a live event timeline, and renders raw run/report JSON.
- Dashboard target management UI for creating, editing, listing, and deleting configured targets.
- GitHub Actions CI for backend tests, backend vet, frontend build, and Docker Compose config validation.
- Docker Compose stack for API, worker, dashboard, demo API, PostgreSQL, Redis, migrations, and Prometheus.

## Storage

The API can run with either storage backend:

- `WRECKR_STORE=memory` for local ephemeral testing.
- `WRECKR_STORE=postgres` for persistent target, scenario, run, report, event, and scenario-version history.

PostgreSQL integration tests are opt-in through `WRECKR_TEST_DATABASE_URL`.

## Current API Surface

- `GET /healthz`
- `GET /metrics`
- `GET /v1/targets`
- `POST /v1/targets`
- `GET /v1/targets/{id}`
- `PUT /v1/targets/{id}`
- `DELETE /v1/targets/{id}`
- `GET /v1/scenarios`
- `POST /v1/scenarios`
- `PUT /v1/scenarios/{id}`
- `GET /v1/scenarios/{id}`
- `GET /v1/scenarios/{id}/versions`
- `GET /v1/runs`
- `POST /v1/runs`
- `GET /v1/runs/{id}`
- `GET /v1/runs/{id}/report`
- `GET /v1/runs/{id}/events`
- `GET /v1/runs/{id}/events/stream`
- `POST /v1/runs/{id}/cancel`

## Operational Notes

- `cmd/api` always wires an Asynq enqueuer. A bare API process can serve health, metrics, and CRUD endpoints, but API run creation requires Redis to accept the enqueue.
- Worker execution requires access to the same storage backend as the API. In the Compose stack this is PostgreSQL.
- Memory storage is useful for local API experiments, but queued API runs are only practical when the API and worker share persistent storage.
- The CLI path does not use targets, scenario versions, storage, Redis, or the worker. It loads one scenario file and prints a text or JSON report.
- Target records use the API field name `baseUrl`; scenario JSON continues to use `target.base_url`.

## Verification

The repository currently has automated coverage for:

- HTTP API integration behavior.
- Scenario version immutability.
- PostgreSQL persistence behavior.
- PostgreSQL target persistence and target-linked run behavior.
- Runner traffic modes.
- Retry storm behavior.
- Race/idempotency detection.
- Hook ordering.
- Threshold failures.
- Context timeout behavior.
- Run cancellation.
- Run event persistence and retrieval.
- Live run event streaming.
- Guardrail enforcement.
- Request-rate pacing.
- Worker execution of queued runs.
- Distributed cancellation for worker-owned running runs.
- Target CRUD endpoints.
- Target resolution for API-created runs with `target_id`.
- k6 compiler script generation for current HTTP scenario files.

Coverage gaps:

- Dashboard target management and live timeline behavior are not covered by browser or component tests yet.

## Main Gaps

- Async orchestration does not yet expose retry/dead-letter dashboards.
- Run event streaming uses SSE today; WebSocket support is not implemented.
- Object storage and report artifact retention are not implemented yet.
- OpenTelemetry tracing is not implemented yet.
- Dashboard does not yet include persisted scenario editing, artifact/log viewing, project management, richer invariant analysis, or retry/dead-letter visibility.
- Non-HTTP protocols such as gRPC, WebSocket, NATS, Kafka, and queue replay are still planned.
- Secrets management, redaction, quotas, audit logs, and production-target warning labels are still planned.

## Recommended Next Milestones

1. Add worker retry/dead-letter visibility to the dashboard.
2. Add persisted scenario editing to the dashboard.
3. Add object storage for raw reports, logs, generated scripts, and artifacts.
4. Add OpenTelemetry traces and richer Prometheus metrics.
