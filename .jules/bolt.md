
## 2024-08-17 - ListX calling GetX Anti-Pattern
**Learning:** Found a severe N+1 query issue in the Go backend (`ListRuns`) where a "list" method loops through retrieved IDs to call a "get" method (`GetRun`). The `GetRun` method issues 3 queries of its own, meaning a single list request with 100 rows results in 301 database queries. This is an easy-to-make performance anti-pattern.
**Action:** Always prefer writing explicit, single queries using `LEFT JOIN`s over reusing `GetX` functions in a loop.
