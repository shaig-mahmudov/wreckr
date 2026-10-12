
## 2024-05-24 - N+1 Anti-Pattern in `ListRuns`
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` originally gathered run IDs using a query and then ran an N+1 set of subqueries inside a loop using `GetRun` for each ID to rebuild standard data mapping, causing an unnecessary load when a `LEFT JOIN` over reports and scenario versions mapped directly into standard types mitigates this immediately.
**Action:** Always inspect the `List` methods inside raw SQL files to ensure they don't depend on iteration-based calls to `Get` methods. Use efficient `LEFT JOIN` queries instead.
## 2024-10-12 - N+1 Memory Fetching in `/metrics`
**Learning:** Tallying statuses for Prometheus metrics using `ListRuns` fetches all entire records and JSON objects from the database into memory, causing CPU and memory spikes that worsen linearly with the amount of test data in the system.
**Action:** When calculating aggregations such as counts on specific columns, define specific database-pushed queries like `GetRunStatusCounts` using `GROUP BY` to drastically lower memory usage and compute cycles.
