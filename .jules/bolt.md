## 2024-08-08 - Optimized ListRuns N+1 Queries
**Learning:** The ListRuns query fetched a list of runs and then iterated over them to call GetRun which queries two more times per item (N+1 query problem). Refactoring to a single query with LEFT JOINs to `scenario_versions` and `reports` reduces database roundtrips.
**Action:** When returning lists, watch out for loops that hit the DB (like calling `GetRun` in a loop over query results).
