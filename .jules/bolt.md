## 2025-02-23 - [Go database/sql N+1 Query in List Methods]
**Learning:** Reusing existing single-fetch wrapper methods (like `GetRun` or `getReport`) inside a list loop (like `ListRuns`) can easily create N+1 query patterns in `database/sql` based backends.
**Action:** When implementing `ListX` methods, always write single queries utilizing `LEFT JOIN`s to fetch all required relations simultaneously instead of iterating over IDs and querying individually.
