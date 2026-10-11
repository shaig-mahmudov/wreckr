
## 2024-05-24 - N+1 Anti-Pattern in `ListRuns`
**Learning:** The method `ListRuns` in `apps/api/internal/store/postgres.go` originally gathered run IDs using a query and then ran an N+1 set of subqueries inside a loop using `GetRun` for each ID to rebuild standard data mapping, causing an unnecessary load when a `LEFT JOIN` over reports and scenario versions mapped directly into standard types mitigates this immediately.
**Action:** Always inspect the `List` methods inside raw SQL files to ensure they don't depend on iteration-based calls to `Get` methods. Use efficient `LEFT JOIN` queries instead.

## $(date +%Y-%m-%d) - Optimization of /metrics by aggregating counts in the DB
**Learning:** The Prometheus `/metrics` endpoint was originally pulling all run records using `ListRuns` and then tallying their statuses in memory. This pulls large JSON snapshots into memory and scales at O(N) with the number of runs. Pushing the aggregation to the database using `GROUP BY` avoids this significant memory and CPU bottleneck.
**Action:** When adding or optimizing endpoints that only require summary counts, always push the aggregation to the database rather than fetching full records into memory.
