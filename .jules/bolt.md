
## 2024-05-24 - N+1 Anti-Pattern in `ListRuns`
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` originally gathered run IDs using a query and then ran an N+1 set of subqueries inside a loop using `GetRun` for each ID to rebuild standard data mapping, causing an unnecessary load when a `LEFT JOIN` over reports and scenario versions mapped directly into standard types mitigates this immediately.
**Action:** Always inspect the `List` methods inside raw SQL files to ensure they don't depend on iteration-based calls to `Get` methods. Use efficient `LEFT JOIN` queries instead.
## 2025-02-27 - Push aggregations to the database layer
**Learning:** Fetching full records into memory to calculate counts or summaries (e.g., iterating through all runs in `metrics` endpoint) creates an O(N) memory and CPU bottleneck as data grows.
**Action:** Always push aggregations to the database by using SQL queries (e.g., `COUNT(*) FILTER (WHERE ...)` or `GROUP BY`) and extend the `Store` interface to support returning aggregated metrics directly.
