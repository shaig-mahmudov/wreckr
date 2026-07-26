# ADR-004: Separate CLI Execution From API Worker Execution

## Status
Accepted

## Context
Wreckr must serve two main use cases: rapid local scenario development (which should remain lightweight and zero-dependency) and durable API/control-plane workflow triggers (which require scaling, persistence, queuing, and long-running execution support).

## Decision
CLI runs execute in-process, while API-created runs are persisted, queued through Redis/Asynq, and executed by an asynchronous worker.

## Consequences
- **Expected:** Developers can use the CLI without infrastructure, while the control plane can scale toward production workflows.
- **Result (Observed):** The CLI path works independently. The API creates queued runs, Asynq handles `runs.execute` tasks, and the worker reloads run snapshots from storage before execution.

## Alternatives Considered

### Alternative 1: Execute all API runs in the API process
- **Expected if chosen:** The first implementation would be simpler, but long-running tests would tie up API resources and make cancellation, retries, and scaling harder.

### Alternative 2: Make every run, including CLI, require Redis and a worker
- **Expected if chosen:** Execution paths would be uniform, but local usage would become heavier and less appealing for quick scenario development.
