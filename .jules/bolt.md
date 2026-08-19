## 2025-02-14 - Fix N+1 Query in ListRuns
**Learning:** The original implementation of ListRuns used a loop over GetRun, resulting in N+1 queries across the runs, scenario_versions, and reports tables.
**Action:** Replaced the loop with a single SQL query using LEFT JOINs to fetch runs along with their related scenario_versions and reports data in one go.
