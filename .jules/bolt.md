
## 2024-05-24 - N+1 Anti-Pattern in `ListRuns`
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` originally gathered run IDs using a query and then ran an N+1 set of subqueries inside a loop using `GetRun` for each ID to rebuild standard data mapping, causing an unnecessary load when a `LEFT JOIN` over reports and scenario versions mapped directly into standard types mitigates this immediately.
**Action:** Always inspect the `List` methods inside raw SQL files to ensure they don't depend on iteration-based calls to `Get` methods. Use efficient `LEFT JOIN` queries instead.
## 2026-10-15 - O(N) memory allocation in  endpoint
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` fetches full run records, and the metrics endpoint in `apps/api/internal/httpapi/server.go` iterates through all run records to count run statuses. This causes an (N)$ memory and CPU bottleneck.
**Action:** When calculating aggregations, avoid loading full records into memory by implementing specific summary functions like `GetRunStatusCounts` that aggregate directly using `GROUP BY` inside the database.
## 2024-05-24 - O(N) memory allocation in metrics endpoint
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` fetches full run records, and the metrics endpoint in `apps/api/internal/httpapi/server.go` iterates through all run records to count run statuses. This causes an O(N) memory and CPU bottleneck.
**Action:** When calculating aggregations, avoid loading full records into memory by implementing specific summary functions like `GetRunStatusCounts` that aggregate directly using GROUP BY inside the database.
