1. **Add `GetRunStatusCounts` to `Store` interface**
   - File: `apps/api/internal/store/store.go`
   - Method signature: `GetRunStatusCounts() map[RunStatus]int`
2. **Implement `GetRunStatusCounts` in `Memory` store**
   - File: `apps/api/internal/store/memory.go`
   - Iterate over `m.runs` and count by status.
3. **Implement `GetRunStatusCounts` in `Postgres` store**
   - File: `apps/api/internal/store/postgres.go`
   - Execute `SELECT status::text, COUNT(*) FROM runs WHERE project_id = $1 GROUP BY status`.
   - Scan results into a map and handle `rows.Err()`.
4. **Optimize `/metrics` handler in `Server`**
   - File: `apps/api/internal/httpapi/server.go`
   - Call `GetRunStatusCounts()` instead of `ListRuns()`.
   - Calculate totals and output the Prometheus metrics.
5. **Pre-commit checks**
   - Complete pre commit steps to make sure proper testing, verifications, reviews and reflections are done.
6. **Commit the changes**
   - Branch: `bolt/optimize-metrics-endpoint`
   - PR Title: `⚡ Bolt: [metrics] optimize memory and CPU by aggregating in DB`
