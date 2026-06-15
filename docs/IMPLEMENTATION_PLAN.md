# Wreckr Implementation Plan

## Product Goal

Wreckr should answer one production question:

> What happens when this backend is hit by realistic failure pressure?

The product remains language-agnostic by default. It tests targets from the outside over HTTP first, then adds optional gray-box integrations through Prometheus, OpenTelemetry, read-only probe endpoints, queues, or database checks.

## MVP Principles

- Black-box first.
- Business invariant failures matter as much as latency failures.
- Every run is reproducible from a scenario file plus generated artifacts.
- The runner core should not depend on the web dashboard.
- Infrastructure should be swappable: local process now, Docker/k6 next, Kubernetes jobs later.

## Phase 1: Current Vertical Slice

Implemented:

- JSON scenario schema.
- Scenario validation.
- Setup and teardown request hooks.
- HTTP request runner.
- Traffic modes:
  - `load`
  - `burst`
  - `spike`
  - `race`
  - `retry_storm`
- Thresholds:
  - max error rate
  - p95 latency
- Invariants:
  - `response_count`
  - `http_probe`
- CLI command:
  - `wreckr run <scenario.json>`
- HTTP API:
  - health check
  - Prometheus-style metrics
  - target create/update/get/list/delete
  - create scenario
  - update scenario
  - list scenarios
  - list scenario versions
  - start run
  - run target override with `target_id`
  - inspect run/report
  - inspect run event timeline
  - stream live run events with SSE
  - cancel running run
- Store interface with memory and PostgreSQL implementations.
- PostgreSQL migration setup.
- Immutable scenario versions linked to run history.
- Persistent run event timelines for lifecycle, request, assertion, threshold, and invariant events.
- Live run progress streaming with Server-Sent Events.
- Run guardrails:
  - target allowlist
  - maximum duration
  - maximum concurrency
  - maximum request rate
  - maximum request body size
  - unsafe target URL rejection
- Next.js dashboard:
  - API health connection
  - sample scenario launcher
  - sample scenario JSON editor
  - target selector
  - target management
  - run list
  - live event timeline
  - report view
  - raw run/report JSON view
- GitHub Actions CI for Go tests, Go vet, frontend build, and Docker Compose config validation.
- Redis + Asynq worker orchestration for API-created runs.
- Demo target API with broken idempotency.

## Phase 2: Persistent Control Plane

Status: mostly implemented for the MVP.

Implemented core tables:

- `projects`
- `targets`
- `scenarios`
- `scenario_versions`
- `runs`
- `run_events`
- `reports`

Implemented behavior:

- target CRUD in memory and PostgreSQL stores
- immutable scenario versions
- run snapshots contain the exact scenario used
- cancellation and timeout state transitions are explicit
- run event timelines are persisted in chronological order
- event metadata is stored as structured JSON
- memory and PostgreSQL stores share the same API-facing `Store` interface
- application can switch between memory and PostgreSQL with `WRECKR_STORE`

Still planned:

- project CRUD beyond the seeded/default project model
- normalized run metrics tables
- normalized threshold and invariant result tables
- artifact metadata and object storage integration
- report artifact retention policies

## Phase 3: Async Orchestration

Status: initial implementation complete.

Implemented:

- Redis-backed Asynq queue.
- `runs.execute` task payload containing the run ID.
- API-created runs are persisted as `queued` and enqueued instead of executing in the API process.
- Separate `cmd/worker` entrypoint consumes queued run jobs.
- Worker reloads the run snapshot from the store, executes the existing runner, and persists status/report transitions.
- Docker Compose starts API, worker, Redis, Postgres, and migrations together.

Current worker responsibilities:

- load queued run
- launch runner
- finalize report

Operational note:

- The production `cmd/api` entrypoint always enqueues API-created runs through Redis/Asynq. Local in-process API execution exists in the server package for tests and embedded use, but the normal API plus worker path requires Redis and shared storage.

Still planned:

- dedicated cancellation task for running worker-owned jobs
- artifact collection
- richer retry/dead-letter visibility
- worker metrics

The current `runner` package should remain the domain engine. Asynq should orchestrate it, not replace it.

## Phase 4: k6 Compiler

Status: planned.

Compile Wreckr scenarios into generated k6 scripts for high-scale HTTP workloads.

Initial compiler targets:

- `constant-vus`
- `ramping-vus`
- `constant-arrival-rate`
- `ramping-arrival-rate`
- race/concurrency helpers
- retry-storm helpers

Artifacts:

- generated k6 script
- k6 JSON summary
- stdout/stderr logs
- normalized Wreckr report

## Phase 5: Observability

Status: partially implemented. The API exposes basic Prometheus metrics at `/metrics`, stores per-run event timelines, and streams run progress with SSE. OpenTelemetry and richer per-run metrics are planned.

Add OpenTelemetry instrumentation to Wreckr itself:

- API request traces
- worker run traces
- runner lifecycle spans
- per-run logs

Add Prometheus metrics:

- `wreckr_runs_total`
- `wreckr_runs_active`
- `wreckr_run_duration_seconds`
- `wreckr_invariant_failures_total`
- `wreckr_runner_requests_total`
- `wreckr_runner_errors_total`

Optional target correlation:

- PromQL invariant checks
- target scrape metadata
- trace IDs injected into requests

Live monitoring follow-up:

- richer event filtering and grouping in the dashboard
- optional WebSocket support if bidirectional control becomes necessary
- add worker retry/dead-letter event visibility

## Phase 6: Frontend Dashboard

Status: partially implemented. The dashboard builds, connects to the API, edits sample scenario JSON, selects a target for a run, launches runs, lists runs, displays report details, shows failures, renders raw run/report JSON, and displays a live event timeline through SSE. It also includes a target management UI.

Planned dashboard flows:

- project setup
- persisted scenario editor
- run launcher
- richer live run status
- richer report view
- invariant failure analysis
- artifact/log viewer
- worker retry/dead-letter visibility

The dashboard should feel like an operational tool: dense, calm, and built for repeated use.

## Phase 7: Production Hardening

Status: partially implemented.

Implemented guardrails:

- target allowlist
- max duration
- max concurrency
- max RPS
- max request body size
- cancellation

Still planned:

- encrypted secrets
- per-project quotas
- audit log
- artifact retention
- warning labels for production targets

Implemented security posture:

- no arbitrary shell execution from scenario files

Still planned security hardening:

- secrets are redacted in logs/reports
- generated scripts are stored and inspectable
- runner containers have restricted network and filesystem access

## Phase 8: Protocols And Failure Modes

Status: planned.

Add adapters:

- gRPC
- WebSocket
- Redis queues
- NATS
- Kafka
- webhook replay

Add fault simulation:

- dependency latency
- dependency error rate
- queue consumer slowdown
- network timeout
- partial outage

Fault injection should remain explicit and opt-in.
