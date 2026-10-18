1. **Analyze the performance bottleneck**:
   The `/metrics` endpoint in `apps/api/internal/httpapi/server.go` retrieves all runs using `s.store.ListRuns()` and iterates over them in memory to count runs grouped by status. As the number of runs grows, this requires pulling large volumes of data into memory, causing CPU and memory bottlenecks.

2. **Add `GetRunStatusCounts` to `Store` interface**:
   Modify `apps/api/internal/store/store.go` to add `GetRunStatusCounts() map[RunStatus]int` to the `Store` interface.

3. **Implement `GetRunStatusCounts` in `Postgres`**:
   Modify `apps/api/internal/store/postgres.go` to add the `GetRunStatusCounts` method that uses a SQL query with `GROUP BY status` to aggregate the counts in the database and return a map of `RunStatus` to `int`.

4. **Implement `GetRunStatusCounts` in `Memory`**:
   Modify `apps/api/internal/store/memory.go` to add the `GetRunStatusCounts` method that iterates over `m.runs` to calculate the status counts (since it's an in-memory store, iteration is necessary and expected).

5. **Update `/metrics` endpoint to use `GetRunStatusCounts`**:
   Modify `apps/api/internal/httpapi/server.go`'s `metrics` function to use `s.store.GetRunStatusCounts()` to efficiently get the run status counts. We'll also need a way to get the total number of runs, so we can calculate it from the sum of the counts.

6. **Pre-commit and Test**:
   Run tests and linting to ensure everything works and performance is improved.
