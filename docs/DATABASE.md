# Database

Wreckr can run with an in-memory store for local demos or a PostgreSQL-backed store for persistent API state.

## Store Backend

Use memory storage:

```bash
WRECKR_STORE=memory
```

Use PostgreSQL storage:

```bash
WRECKR_STORE=postgres
DATABASE_URL=postgres://wreckr:wreckr@localhost:5432/wreckr?sslmode=disable
```

## Migrations

Migration files live in:

```text
deployments/postgres/migrations
```

Start Postgres and apply migrations:

```bash
docker compose up -d postgres
docker compose --profile tools run --rm migrate
```

## PostgreSQL Integration Test

The Postgres store test is opt-in so normal local and CI test runs do not require Docker or a database.

```bash
WRECKR_TEST_DATABASE_URL=postgres://wreckr:wreckr@localhost:5432/wreckr_test?sslmode=disable go test ./apps/api/internal/store
```
