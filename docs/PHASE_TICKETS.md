# Wreckr Phase Tickets

Last updated: 2026-06-16

These tickets translate the implementation phases into issue-ready work items. Each ticket can be copied into GitHub Issues, Linear, Jira, or kept as the local planning backlog.

## WRK-P1: Stabilize Current Vertical Slice

Status: mostly complete

Phase: 1 - Current Vertical Slice

Goal: Keep the MVP runner, API, CLI, dashboard, demo API, and Compose path reliable while later phases build on top of them.

Scope:

- Maintain black-box HTTP scenario execution across `load`, `burst`, `spike`, `race`, and `retry_storm`.
- Preserve setup/teardown hooks, expectations, thresholds, invariants, reports, and run events.
- Keep the CLI `wreckr run <scenario.json>` path working independently of the API and worker.
- Keep the demo API useful for idempotency and rate-limit scenarios.
- Keep CI green for Go tests, Go vet, frontend build, and Compose config validation.

Out of Scope:

- New protocol adapters.
- k6 execution.
- Production secrets, quotas, and audit logs.
- Major dashboard redesigns.

Limits:

- Keep this ticket limited to regression prevention and small stabilization fixes.
- Do not introduce new infrastructure dependencies.
- Every change must preserve the CLI local-file workflow.

Acceptance criteria:

- `go test ./...` passes.
- `go vet ./...` passes.
- `npm.cmd run build` passes from `apps/web`.
- `docker compose config --quiet` passes.
- Both sample scenarios still run against the demo API.
- README quick-start commands match the actual local execution paths.

Dependencies:

- None.

Notes:

- Treat this as the baseline quality gate for all other phase work.

## WRK-P2: Complete Persistent Control Plane

Status: partially complete

Phase: 2 - Persistent Control Plane

Goal: Finish the durable data model around projects, targets, scenarios, runs, events, reports, metrics, and artifacts.

Scope:

- Add project CRUD beyond the seeded/default project model.
- Add normalized run metrics tables.
- Add normalized threshold result tables.
- Add normalized invariant result tables.
- Add artifact metadata tables.
- Add retention policy fields and lifecycle metadata for reports/artifacts.
- Keep scenario versioning immutable and linked to historical runs.
- Ensure target-resolved scenario snapshots remain stable after target edits.

Out of Scope:

- Worker retry/dead-letter orchestration.
- k6 artifact generation.
- Dashboard artifact viewers.
- Non-PostgreSQL durable stores.

Limits:

- Keep schema changes backward-migratable through SQL migrations.
- Do not remove or weaken the memory store used by tests and local experiments.
- Keep historical run snapshots immutable after creation.

Acceptance criteria:

- API and worker can operate against the same PostgreSQL schema without relying on in-memory state.
- Historical runs continue to display the exact scenario snapshot, scenario version, and target used at execution time.
- Migration up/down paths work on a clean database.
- PostgreSQL integration tests cover targets, project ownership, scenario versions, runs, events, reports, and normalized result records.
- Database docs describe all persisted entities and retention behavior.

Dependencies:

- WRK-P1.

Notes:

- Target CRUD and target resolution now have API coverage.
- PostgreSQL integration coverage now verifies target persistence, run target IDs, target IDs in queued events, and target-deletion behavior for historical run snapshots.

## WRK-P3: Harden Async Orchestration

Status: partially complete

Phase: 3 - Async Orchestration

Goal: Make API-created run execution operationally reliable when owned by Redis/Asynq workers.

Scope:

- Add distributed cancellation for worker-owned running jobs.
- Persist worker retry attempts and final dead-letter state into run events.
- Expose retry/dead-letter visibility through API endpoints.
- Add worker metrics for queue depth, active jobs, retries, failures, and duration.
- Ensure worker shutdown and timeout behavior preserve clear terminal run states.
- Document local and production run execution topology.

Out of Scope:

- k6 execution backends.
- Kubernetes job orchestration.
- Dashboard redesign beyond exposing retry/cancel state.
- Full observability tracing.

Limits:

- Keep orchestration scoped to Redis/Asynq.
- Do not require callers to know which worker owns a run.
- Preserve existing synchronous test/embedded server behavior.
- Terminal run states must remain single and explicit: passed, failed, errored, or canceled.

Acceptance criteria:

- A running worker-owned job can be canceled from `POST /v1/runs/{id}/cancel`.
- Queued, running, retried, dead-lettered, failed, canceled, and completed worker states appear in the run event timeline.
- Failed jobs surface enough retry/dead-letter context for dashboard display.
- Worker metrics can be scraped by Prometheus.
- Tests cover queued cancellation, running distributed cancellation, retry metadata, and terminal state transitions.

Dependencies:

- WRK-P1.
- WRK-P2 for durable retry/dead-letter visibility.

Notes:

- Current API run creation already enqueues `runs.execute` jobs through Asynq.
- Durable cancellation requests, worker-owned running cancellation, and persisted worker retry/dead-letter timeline events are implemented; dashboard retry/dead-letter visibility and worker metrics remain open.

## WRK-P4: Add k6 Compiler

Status: planned

Phase: 4 - k6 Compiler

Goal: Compile Wreckr scenarios into k6 scripts for higher-scale HTTP workloads while preserving Wreckr reports.

Scope:

- Design the scenario-to-k6 compilation contract.
- Generate k6 scripts for constant VUs, ramping VUs, constant arrival rate, and ramping arrival rate.
- Support race/concurrency helpers.
- Support retry-storm helpers.
- Collect k6 JSON summaries, logs, and exit status.
- Normalize k6 results back into Wreckr report structures.
- Store generated scripts and raw summaries as artifacts once artifact storage exists.

Out of Scope:

- Replacing the Go runner.
- Kubernetes-native execution.
- Non-HTTP protocols.
- Full artifact retention policy implementation.

Limits:

- Compiler output must be deterministic for the same scenario input.
- Initial support is HTTP only.
- Generated scripts must remain inspectable before execution.
- Reports must normalize back into the existing Wreckr report shape.

Acceptance criteria:

- A scenario can be compiled into a deterministic k6 script.
- Generated k6 scripts run successfully against the demo API.
- k6 summaries map into Wreckr report status, latency, status-code, threshold, and failure fields.
- Compiler tests cover traffic mode mapping, headers/body handling, thresholds, and invalid scenarios.
- Generated artifacts are inspectable in local development.

Dependencies:

- WRK-P1.
- WRK-P2 for artifact metadata.
- WRK-P3 if k6 execution is worker-owned.

Notes:

- The runner package should remain the domain engine; k6 is an execution backend, not a replacement for the scenario model.
- Deterministic script generation and the `wreckr compile-k6` CLI command are implemented; k6 execution, summary parsing, and normalized report import remain open.

## WRK-P5: Expand Observability

Status: partially complete

Phase: 5 - Observability

Goal: Make Wreckr itself observable across API requests, worker execution, runner lifecycle, and per-run outcomes.

Scope:

- Add OpenTelemetry instrumentation for API request traces.
- Add worker run traces.
- Add runner lifecycle spans.
- Add per-run structured logs.
- Expand Prometheus metrics beyond the current basic API counters.
- Add invariant, request, error, and latency metrics.
- Add optional target correlation through trace IDs, PromQL checks, or scrape metadata.

Out of Scope:

- Full dashboard analytics.
- External hosted observability service setup.
- Protocol-specific metrics for future adapters.
- Business-level SLO management.

Limits:

- Use standard OpenTelemetry and Prometheus conventions.
- Metrics must keep stable names once documented.
- Avoid high-cardinality labels such as raw URLs, request bodies, or arbitrary header values.

Acceptance criteria:

- API, worker, and runner traces can be exported through standard OpenTelemetry configuration.
- Metrics include runs total, active runs, run duration, invariant failures, runner requests, and runner errors.
- Per-run events, logs, and metrics can be correlated by run ID.
- Observability docs describe local Prometheus and tracing setup.
- Tests or smoke checks verify metric names and basic trace wiring.

Dependencies:

- WRK-P1.
- WRK-P3 for worker metrics.

Notes:

- Current observability includes `/metrics`, persisted run timelines, and SSE progress streams.

## WRK-P6: Complete Frontend Dashboard

Status: partially complete

Phase: 6 - Frontend Dashboard

Goal: Turn the dashboard from a useful MVP console into the primary operational workspace for targets, scenarios, runs, events, and reports.

Scope:

- Add persisted scenario create/edit/version flows.
- Add project setup and navigation.
- Improve run launcher ergonomics.
- Improve live run status and timeline filtering/grouping.
- Add richer report and invariant failure analysis.
- Add artifact/log viewer.
- Add worker retry/dead-letter visibility.
- Add browser or component tests for critical workflows.

Out of Scope:

- Marketing or landing pages.
- Full multi-tenant administration.
- k6 script authoring UI.
- Non-HTTP protocol builders.

Limits:

- Keep the first screen as the operational console.
- Preserve raw JSON inspection for advanced users.
- Do not hide API errors; surface actionable messages.
- Critical controls must remain usable on common laptop-sized viewports.

Acceptance criteria:

- Users can create/edit targets and scenarios without editing raw JSON for common workflows.
- Users can launch a run from a persisted scenario and selected target.
- Timeline, report, failures, and raw JSON remain available for detailed inspection.
- Dashboard handles loading, empty, error, and offline API states cleanly.
- Target management, run launch, and timeline display are covered by automated tests or Playwright smoke checks.

Dependencies:

- WRK-P1.
- WRK-P2 for projects, normalized results, and artifacts.
- WRK-P3 for retry/dead-letter visibility.

Notes:

- Keep the dashboard dense, calm, and suited to repeated operational use.

## WRK-P7: Production Hardening

Status: partially complete

Phase: 7 - Production Hardening

Goal: Make Wreckr safer to run against real systems and easier to govern in team environments.

Scope:

- Add encrypted secrets.
- Add secret redaction in logs, run events, reports, and generated artifacts.
- Add per-project quotas.
- Add audit logs.
- Add artifact retention policies.
- Add warning labels and confirmations for production targets.
- Restrict runner containers with tighter network and filesystem controls.
- Ensure generated scripts are stored and inspectable before execution.

Out of Scope:

- Enterprise SSO.
- Billing.
- Full policy-as-code engine.
- Replacing existing guardrails.

Limits:

- Safety checks must fail closed.
- Secret redaction must apply consistently to API responses, reports, events, and logs.
- Quotas must be enforceable per project before a run starts.
- Production-target warnings must not block local/development targets.

Acceptance criteria:

- Secrets are never stored or displayed in plaintext outside the configured secret backend.
- Reports and event streams redact sensitive headers, bodies, and configured fields.
- Quotas prevent runaway concurrency, request rate, duration, and artifact usage per project.
- Audit logs capture target, scenario, run, cancellation, and secret-related changes.
- Production targets are visibly labeled and require explicit confirmation before high-risk runs.

Dependencies:

- WRK-P1.
- WRK-P2 for project, artifact, and audit persistence.
- WRK-P6 for production warning labels in the dashboard.

Notes:

- Current implemented hardening includes guardrails and no arbitrary shell execution from scenario files.

## WRK-P8: Add Protocols And Failure Modes

Status: planned

Phase: 8 - Protocols And Failure Modes

Goal: Extend Wreckr beyond HTTP request pressure into additional protocols and explicit failure simulation.

Scope:

- Add gRPC adapter.
- Add WebSocket adapter.
- Add Redis queue adapter.
- Add NATS adapter.
- Add Kafka adapter.
- Add webhook replay support.
- Add explicit dependency latency simulation.
- Add explicit dependency error-rate simulation.
- Add queue consumer slowdown simulation.
- Add network timeout and partial outage simulations.

Out of Scope:

- Rewriting existing HTTP scenario behavior.
- Supporting every protocol in one release.
- Implicit or hidden fault injection.
- Production chaos experiments without guardrails.

Limits:

- Add adapters incrementally, one protocol at a time.
- Every protocol extension needs schema validation, tests, examples, and safety docs.
- Failure modes must remain explicit and opt-in.
- Reports must keep protocol-specific detail while preserving the common Wreckr summary.

Acceptance criteria:

- Each new adapter has a scenario schema extension, validation, runner behavior, reports, and tests.
- Failure simulation remains explicit and opt-in.
- Non-HTTP results normalize into Wreckr reports without losing protocol-specific detail.
- Documentation includes examples and safety guardrails for each protocol or failure mode.
- Demo or example targets exist for at least the first non-HTTP adapter.

Dependencies:

- WRK-P1.
- WRK-P2 for durable protocol-specific metadata.
- WRK-P7 for safety controls around higher-risk failure modes.

Notes:

- Add adapters incrementally; do not block one protocol on the full phase.
