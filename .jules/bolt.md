
## 2024-05-24 - N+1 Anti-Pattern in `ListRuns`
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` originally gathered run IDs using a query and then ran an N+1 set of subqueries inside a loop using `GetRun` for each ID to rebuild standard data mapping, causing an unnecessary load when a `LEFT JOIN` over reports and scenario versions mapped directly into standard types mitigates this immediately.
**Action:** Always inspect the `List` methods inside raw SQL files to ensure they don't depend on iteration-based calls to `Get` methods. Use efficient `LEFT JOIN` queries instead.
## 2025-05-25 - Push Prometheus Metric Aggregations to the Database
**Learning:** In `apps/api/internal/httpapi/server.go`, calculating Prometheus summary metrics by calling `s.store.ListRuns()` causes full memory hydration of every run object merely to count statuses. This creates an O(N) memory and CPU bottleneck as runs increase.
**Action:** Always push the aggregation to the database using `GROUP BY` rather than fetching full records into memory to prevent O(N) memory and CPU bottlenecks when you only need summary counts.
