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

Implemented in this first pass:

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
  - create scenario
  - list scenarios
  - start run
  - inspect run/report
- Demo target API with broken idempotency.

## Phase 2: Persistent Control Plane

Replace in-memory storage with PostgreSQL.

Core tables:

- `projects`
- `targets`
- `scenarios`
- `scenario_versions`
- `runs`
- `run_events`
- `run_metrics`
- `run_threshold_results`
- `run_invariant_results`
- `artifacts`
- `reports`

Important behavior:

- immutable scenario versions
- run snapshots contain the exact scenario used
- report artifacts are stored separately from relational metadata
- cancellation and timeout state transitions are explicit

## Phase 3: Async Orchestration

Introduce Redis + Asynq.

Jobs:

- `run.start`
- `run.cancel`
- `artifact.collect`
- `report.finalize`

Worker responsibilities:

- reserve run
- compile scenario
- launch runner
- stream run events
- collect summaries/logs
- finalize report

The current `runner` package should remain the domain engine. Asynq should orchestrate it, not replace it.

## Phase 4: k6 Compiler

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

## Phase 6: Frontend Dashboard

Dashboard flows:

- project/target setup
- scenario editor
- run launcher
- live run status
- report view
- invariant failure analysis
- artifact/log viewer

The dashboard should feel like an operational tool: dense, calm, and built for repeated use.

## Phase 7: Production Hardening

Guardrails:

- target allowlist
- max duration
- max concurrency
- max RPS
- max request body size
- encrypted secrets
- per-project quotas
- audit log
- cancellation
- artifact retention
- warning labels for production targets

Security:

- no arbitrary shell execution from scenario files
- secrets are redacted in logs/reports
- generated scripts are stored and inspectable
- runner containers have restricted network and filesystem access

## Phase 8: Protocols And Failure Modes

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
