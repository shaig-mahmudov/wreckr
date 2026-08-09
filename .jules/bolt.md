## 2024-03-24 - [Avoid N+1 Queries in PostgreSQL Data Access]
**Learning:** In `apps/api/internal/store/postgres.go`, `ListRuns` exhibited a classic N+1 query problem, fetching each run using `GetRun` within a loop over IDs.
**Action:** Always verify loops fetching detailed objects from IDs to ensure they are fetched with `JOIN`s in a single query.
