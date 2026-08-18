## 2024-08-18 - Resolve N+1 query in ListRuns
**Learning:** The Go backend's raw SQL data access layer was susceptible to the N+1 query anti-pattern, particularly in list methods (e.g., `ListRuns`) that looped over IDs to call individual fetch methods (e.g., `GetRun` and subsequently `getReport`).
**Action:** Avoid reusing individual `Get` methods inside loops within `List` methods. Instead, write custom queries for `List` methods using `LEFT JOIN`s (e.g., joining `scenario_versions` and `reports` for runs) to retrieve all necessary nested records in a single, efficient O(1) query.
