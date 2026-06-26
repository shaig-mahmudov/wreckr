# Wreckr Decisions

Last updated: 2026-06-26

This document records the important product and technical decisions made so far. Each entry explains why the decision was made, what we expected, what we observed after implementation, and two alternatives we intentionally did not choose.

## DEC-001: Build Wreckr Black-Box First

Decision: Wreckr tests backends from the outside over HTTP before adding gray-box integrations.

Why: The product goal is language-agnostic production scenario testing. A black-box runner can test Go, C#, Java, Node.js, Python, and other stacks without requiring SDKs or application code changes.

Expected: Teams can point Wreckr at a target service, run realistic pressure scenarios, and detect behavioral failures such as weak idempotency, bad rate limiting, and broken retry handling.

Result: The MVP can run HTTP scenarios against the demo API and external targets, validate status expectations, thresholds, and invariants, and report failures without target-side instrumentation.

Alternative 1: Start with language-specific SDKs.

Expected if chosen: Wreckr would have deeper in-process visibility, but adoption would be slower and every supported language would need custom maintenance.

Alternative 2: Start with Kubernetes-only fault injection.

Expected if chosen: Wreckr would be closer to infrastructure chaos testing, but less useful for local development and less focused on business invariant failures.

## DEC-002: Use JSON Scenario Files As The Core Contract

Decision: Scenarios are versioned JSON documents with explicit target, traffic, request, threshold, and invariant sections.

Why: JSON is portable across CLI, API, dashboard, generated artifacts, and future execution backends such as k6.

Expected: Scenario files stay easy to inspect, commit, copy, replay, and compile into other formats.

Result: The CLI loads scenario JSON directly, the API accepts inline scenarios and persisted scenarios, and the dashboard can edit sample scenario JSON today.

Alternative 1: Create a custom DSL.

Expected if chosen: The language could be more expressive, but parsing, tooling, editor support, validation, and docs would cost more early in the project.

Alternative 2: Store scenarios only through database forms.

Expected if chosen: The dashboard could guide users more tightly, but reproducibility and CLI-first workflows would be weaker.

## DEC-003: Keep The Runner Core In Go

Decision: The scenario engine and HTTP runner are implemented in Go.

Why: Go is a good fit for concurrent HTTP traffic, cancellation, timeouts, simple deployment, and a small standalone CLI/API binary.

Expected: The runner can execute multiple traffic modes with predictable concurrency behavior and simple operational packaging.

Result: The MVP supports load, burst, spike, race, and retry-storm traffic modes, context cancellation, pacing, and structured reports with a compact codebase.

Alternative 1: Build the runner in Node.js.

Expected if chosen: The dashboard and runner could share a TypeScript ecosystem, but high-concurrency execution and binary distribution would likely be less straightforward.

Alternative 2: Use k6 as the only runner from day one.

Expected if chosen: High-scale HTTP workloads would arrive sooner, but Wreckr would have less control over the domain model, event timeline, cancellation behavior, and invariant evaluation.

## DEC-004: Separate CLI Execution From API Worker Execution

Decision: CLI runs execute in-process, while API-created runs are persisted, queued through Redis/Asynq, and executed by a worker.

Why: Local file runs should stay simple, but API runs need asynchronous execution, durable state, and a path toward retries, worker metrics, and orchestration.

Expected: Developers can use the CLI without infrastructure, while the control plane can scale toward production workflows.

Result: The CLI path works independently. The API creates queued runs, Asynq handles `runs.execute` tasks, and the worker reloads run snapshots from storage before execution.

Alternative 1: Execute all API runs in the API process.

Expected if chosen: The first implementation would be simpler, but long-running tests would tie up API resources and make cancellation, retries, and scaling harder.

Alternative 2: Make every run, including CLI, require Redis and a worker.

Expected if chosen: Execution paths would be uniform, but local usage would become heavier and less appealing for quick scenario development.

## DEC-005: Support Memory And PostgreSQL Stores Behind One Interface

Decision: Storage is abstracted behind a `Store` interface with memory and PostgreSQL implementations.

Why: Memory storage keeps tests and local experiments fast. PostgreSQL provides durable control-plane history for realistic API and worker operation.

Expected: The app can run in lightweight local mode or persistent mode without changing API behavior.

Result: The same API-facing behavior works with both stores. PostgreSQL persists projects, targets, scenarios, scenario versions, runs, run events, and reports.

Alternative 1: Use PostgreSQL only.

Expected if chosen: Fewer code paths would exist, but tests and local experiments would require more infrastructure.

Alternative 2: Use in-memory storage only until later.

Expected if chosen: Early development would be faster, but async worker execution and historical run reporting would be fragile or impossible across processes.

## DEC-006: Make Scenario Versions Immutable

Decision: Scenario updates create new immutable scenario version records, and runs keep version IDs plus scenario snapshots.

Why: Reports must remain reproducible and understandable even after a scenario changes.

Expected: Historical reports continue to describe exactly what executed.

Result: Runs link to the scenario version used at creation, reports retain version metadata, and old reports are not mutated by later scenario edits.

Alternative 1: Mutate scenarios in place without versions.

Expected if chosen: The data model would be simpler, but historical reports could become misleading after edits.

Alternative 2: Store only full snapshots and no version records.

Expected if chosen: Reproducibility would survive, but users would lose clear scenario revision history and diffable version lists.

## DEC-007: Persist Run Events And Stream With SSE

Decision: Wreckr stores run events and streams live progress with Server-Sent Events.

Why: Runs need an inspectable timeline for lifecycle transitions, requests, assertions, thresholds, invariants, cancellation, and terminal states. SSE fits one-way live updates with low frontend complexity.

Expected: Users can watch live progress and still inspect the same timeline later.

Result: The API exposes persisted events at `/v1/runs/{id}/events` and live streams at `/v1/runs/{id}/events/stream`; the dashboard displays a live event timeline.

Alternative 1: Use WebSockets immediately.

Expected if chosen: Bidirectional control would be possible, but the implementation would be more complex before there is a clear need for client-to-server streaming.

Alternative 2: Poll run status only.

Expected if chosen: The API would be simpler, but users would lose detailed execution context and failure diagnosis would be weaker.

## DEC-008: Add Guardrails Before Production-Target Features

Decision: The API validates scenarios with guardrails for concurrency, request rate, run duration, body size, target allowlists, metadata-service blocking, and unsafe URLs.

Why: Wreckr intentionally generates pressure. Safety controls need to exist before making production-oriented target management feel first-class.

Expected: Unsafe or excessive runs are rejected early with clear errors.

Result: Guardrail tests cover max concurrency, max request rate, max duration, max request body size, target allowlist behavior, credentials rejection, metadata protection, and absolute URL validation.

Alternative 1: Trust users and document warnings only.

Expected if chosen: The product would be easier to build, but accidental harmful runs would be more likely.

Alternative 2: Allow only localhost targets in the MVP.

Expected if chosen: Safety would improve, but realistic staging and shared development workflows would be blocked.

## DEC-009: Introduce Target Management As A Reusable Control-Plane Primitive

Decision: Targets are first-class records with names, base URLs, environments, descriptions, headers, and run-time resolution through `target_id`.

Why: Reusable targets reduce duplicate scenario JSON and allow one scenario to run against local, development, staging, or production-like environments.

Expected: Users can define an environment once, select it at run time, and keep scenario files focused on behavior.

Result: The API supports target CRUD and target resolution. The dashboard can create, edit, list, delete, and select targets. API tests now cover target CRUD and `target_id` run resolution.

Alternative 1: Keep target URLs only inside scenario JSON.

Expected if chosen: The schema would stay simpler, but environment reuse and dashboard workflows would be clumsy.

Alternative 2: Store targets only in environment variables.

Expected if chosen: Deployment configuration would be simpler, but users could not manage targets through the API or dashboard.

## DEC-010: Use Docker Compose As The Complete Local Stack

Decision: Docker Compose defines the local integrated environment for API, worker, web, demo API, PostgreSQL, Redis, migrations, and Prometheus.

Why: The project needs a repeatable way to exercise the real multi-process topology without requiring Kubernetes.

Expected: Contributors can run the complete MVP locally and CI can validate the Compose configuration.

Result: Compose starts the complete stack, and CI validates `docker compose config --quiet`.

Alternative 1: Use only local `go run` and `npm run dev` commands.

Expected if chosen: Local iteration would be lightweight, but Postgres, Redis, worker, migrations, and Prometheus integration would be easier to miss.

Alternative 2: Use Kubernetes manifests first.

Expected if chosen: Production topology would be more realistic, but local development would be heavier and slower.

## DEC-011: Build The Dashboard As An Operational Console, Not A Landing Page

Decision: The Next.js app opens directly into the Wreckr console instead of a marketing or landing page.

Why: The product is an operational tool. The first screen should help users connect to the API, edit/launch scenarios, inspect runs, view timelines, and manage targets.

Expected: The UI feels useful immediately and supports repeated workflows.

Result: The dashboard includes API connectivity, scenario JSON editing, target selection and management, run list, report metrics, failures, timeline, and raw JSON inspection.

Alternative 1: Build a marketing-style homepage first.

Expected if chosen: The product would look more polished externally, but users would need extra navigation before doing real work.

Alternative 2: Build only raw API docs first.

Expected if chosen: Backend workflows would be clear for developers, but non-API users would have no usable control-plane experience.

## DEC-012: Treat k6, Object Storage, And Kubernetes Jobs As Later Execution Layers

Decision: k6 compilation, object storage, and Kubernetes job orchestration are planned after the core runner, persistence, events, and worker path.

Why: The MVP needed a coherent domain model and control plane before adding heavier execution backends and artifact infrastructure.

Expected: Later orchestration layers can wrap the same scenario and report model instead of forcing a redesign.

Result: The implementation plan keeps k6 as Phase 4, observability as Phase 5, and production hardening/protocol expansion as later phases.

Alternative 1: Start with Kubernetes Jobs as the primary execution unit.

Expected if chosen: Isolation and scale would improve earlier, but local development and iteration speed would suffer.

Alternative 2: Start with object storage and artifacts before reports/events.

Expected if chosen: Artifact retention would be ready sooner, but there would be fewer meaningful artifacts to store before the report and event model stabilized.

## DEC-013: Add Visual Scenario Builder Alongside JSON/YAML Editing

Decision: Implement a visual, form-based scenario builder as the default interface while retaining the raw JSON/YAML text editor as a toggle.

Why: Non-technical users and developers wanting quick scenario mockups need an intuitive visual way to construct HTTP paths, header inputs, and traffic shapes without syntax errors. However, power users still need the ability to edit raw configurations.

Expected: Users can construct complex HTTP scenarios visually with instant feedback while retaining full fidelity and serialization control.

Result: Next.js dashboard features a visual builder for metadata, targets, traffic profiles, and requests list. Users can toggle to JSON/YAML mode seamlessly. If invalid formatting is input, we prevent visual mode switching and show a validation error.

Alternative 1: Replace raw JSON/YAML editor entirely with form inputs.

Expected if chosen: The UI would be simpler, but developers could not copy-paste complex config files or quickly customize details outside the form's structured bounds.

Alternative 2: Keep only raw text editor and add auto-complete/LSP tooling.

Expected if chosen: Helping users with schema definition would be possible, but it wouldn't eliminate the friction of learning the JSON/YAML structure, and errors would still be common.
