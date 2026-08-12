## 2024-05-14 - [Backend Optimization: N+1 queries in Listing Methods]
**Learning:** The Go backend uses standard `database/sql` rather than an ORM. The `ListRuns` method (and potentially others) implements an N+1 query anti-pattern by querying the list of IDs and then calling `GetRun` (which itself executes multiple queries) in a loop for each record.
**Action:** When working on backend list endpoints, always check for loops calling `GetX` methods and refactor them to use single `LEFT JOIN` queries instead to avoid N+1 performance bottlenecks.
