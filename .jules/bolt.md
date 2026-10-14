
## 2024-05-24 - N+1 Anti-Pattern in `ListRuns`
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` originally gathered run IDs using a query and then ran an N+1 set of subqueries inside a loop using `GetRun` for each ID to rebuild standard data mapping, causing an unnecessary load when a `LEFT JOIN` over reports and scenario versions mapped directly into standard types mitigates this immediately.
**Action:** Always inspect the `List` methods inside raw SQL files to ensure they don't depend on iteration-based calls to `Get` methods. Use efficient `LEFT JOIN` queries instead.

## 2024-05-24 - O(N) Anti-Pattern in `metrics` function
**Learning:** The `metrics` function in `apps/api/internal/httpapi/server.go` was previously fetching all run records into memory by calling `ListRuns` just to calculate status counts for Prometheus metrics. This can cause O(N) memory and CPU bottlenecks as the number of runs increases.
**Action:** When adding or optimizing endpoints that need summary counts (such as Prometheus `/metrics`), always push the aggregation to the database using `GROUP BY` rather than fetching full records into memory.
