# Database

Wreckr uses PostgreSQL for persistent control-plane data.

The API can run with either storage backend:

```bash
WRECKR_STORE=memory
WRECKR_STORE=postgres
```

When `WRECKR_STORE=postgres`, the API uses `DATABASE_URL` and persists scenarios, immutable scenario versions, runs, reports, and run state.

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
docker compose --profile tools run --rm migrate
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
docker compose -p wreckr_migration_test --profile tools run --rm migrate
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

Runs store both `scenario_id` and `scenario_version_id`, plus a JSON scenario snapshot, so old reports continue to show the exact scenario version that executed even after the scenario is edited.

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
