
## 2024-05-24 - N+1 Anti-Pattern in `ListRuns`
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` originally gathered run IDs using a query and then ran an N+1 set of subqueries inside a loop using `GetRun` for each ID to rebuild standard data mapping, causing an unnecessary load when a `LEFT JOIN` over reports and scenario versions mapped directly into standard types mitigates this immediately.
**Action:** Always inspect the `List` methods inside raw SQL files to ensure they don't depend on iteration-based calls to `Get` methods. Use efficient `LEFT JOIN` queries instead.
## 2024-05-24 - O(N) Fetch for Metrics
**Learning:** In the Go backend, retrieving summary counts using application-level iteration (e.g. `ListRuns` inside a metrics endpoint) is an O(N) memory and CPU bottleneck since all underlying rows are deserialized only to count them.
**Action:** Always push aggregations (like simple counts grouped by status) to the database layer by implementing and utilizing `GROUP BY` SQL queries via newly exposed interface methods rather than fetching the data in memory.
