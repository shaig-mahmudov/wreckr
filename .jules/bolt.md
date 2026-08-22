## 2024-05-24 - Eliminated N+1 Query in ListRuns
**Learning:** Found an N+1 query issue in the Go backend where `ListRuns` iterated over query results and executed `GetRun` for each row, hitting the database to fetch run and report data in a loop.
**Action:** Always prefer `LEFT JOIN` and bulk fetching to pull all related data in a single query rather than iterating and calling single-fetch methods. Replaced the looped `GetRun` calls with a single query leveraging `LEFT JOIN` on `scenario_versions` and `reports`.
