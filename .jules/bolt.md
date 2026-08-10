## 2024-08-11 - Fixed N+1 Query in `ListRuns`
**Learning:** `apps/api/internal/store/postgres.go` manually implemented relational joins by querying for a list of IDs and then running 3 sub-queries per ID (via `GetRun`). This acts like an ORM N+1 query problem, but hidden within manual query implementations.
**Action:** When working with `postgres.go` or other raw SQL stores in this codebase, explicitly look for methods like `ListX` that loop over IDs and call `GetX`. These are primary targets for replacing with a single `LEFT JOIN` query.
