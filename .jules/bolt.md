## 2024-05-24 - Fix N+1 Query in database/sql List Function
**Learning:** Found an N+1 query pattern in `apps/api/internal/store/postgres.go` where `ListRuns` fetched IDs, then ran `GetRun` inside a loop for each ID, firing 1 + (2*N) queries.
**Action:** When implementing `ListX` methods in raw `database/sql`, avoid reusing `GetX` in a loop. Instead, write a dedicated `LEFT JOIN` query in `ListX` to fetch and map all required main and child relation data in a single round-trip.
