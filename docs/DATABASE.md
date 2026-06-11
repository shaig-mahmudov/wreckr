# Database

Wreckr uses PostgreSQL for persistent control-plane data.

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

Run lifecycle state is stored in the `run_status` enum and used by both `runs.status` and `reports.status`.
