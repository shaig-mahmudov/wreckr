# Database

Wreckr uses PostgreSQL for persistent control-plane data.

The API can run with either storage backend:

```bash
WRECKR_STORE=memory
WRECKR_STORE=postgres
```

When `WRECKR_STORE=postgres`, the API uses `DATABASE_URL` and persists targets, scenarios, immutable scenario versions, runs, run events, reports, and run state.

API-created runs are queued through Redis/Asynq and executed by the worker. In a multi-process setup, the API and worker must use the same PostgreSQL database so the worker can reload queued run snapshots and persist final reports.

## Migration Tool

Migrations are plain SQL files under:

```text
deployments/postgres/migrations
```

They are executed with the `migrate/migrate` Docker image through Docker Compose.

## Run Migrations

Start Postgres:

```bash
docker compose up -d postgres
```

Apply all pending migrations:

```bash
docker compose run --rm migrate
```

PowerShell helper:

```powershell
.\scripts\migrate.ps1 up
```

Rollback one migration:

```powershell
.\scripts\migrate.ps1 down 1
```

## Clean Database Verification

Run migrations against an isolated clean database:

```bash
docker compose -p wreckr_migration_test run --rm migrate
docker compose -p wreckr_migration_test down -v
```

## Initial Schema

The initial schema includes:

- `projects`
- `targets`
- `scenarios`
- `scenario_versions`
- `runs`
- `run_events`
- `reports`

Scenario versioning is modeled explicitly with immutable `scenario_versions` rows and a nullable `scenarios.current_version_id` pointer.

Targets are modeled in `targets` and can be linked to runs through `runs.target_id`. When a run is created with a target ID, the API resolves the target base URL and merges target headers into the scenario snapshot before queuing execution.

Runs store `target_id`, `scenario_id`, and `scenario_version_id`, plus a JSON scenario snapshot, so old reports continue to show the exact target-resolved scenario version that executed even after the scenario or target is edited.

Run events are stored in `run_events` with a per-run sequence number, event level, event type, message, structured JSON metadata, and timestamp. The API returns them in chronological sequence through `GET /v1/runs/{id}/events` and streams them live through `GET /v1/runs/{id}/events/stream`. Worker-owned Asynq runs also persist `worker_attempt_started`, `worker_attempt_failed`, `worker_retry_scheduled`, and `worker_dead_lettered` events with task, queue, attempt, and retry metadata.

Run lifecycle state is stored in the `run_status` enum and used by both `runs.status` and `reports.status`.

Current run statuses:

- `queued`
- `running`
- `passed`
- `failed`
- `errored`
- `canceled`

## Integration Tests

PostgreSQL store tests are opt-in. Set `WRECKR_TEST_DATABASE_URL` to a disposable database URL before running:

```bash
WRECKR_TEST_DATABASE_URL=postgres://wreckr:wreckr@localhost:5432/wreckr_test?sslmode=disable go test ./apps/api/internal/store
```
