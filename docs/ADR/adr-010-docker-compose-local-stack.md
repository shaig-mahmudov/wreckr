# ADR-010: Use Docker Compose As The Complete Local Stack

## Status
Accepted

## Context
Wreckr is built with a distributed multi-process topology (API, worker, Next.js frontend, Postgres database, Redis queue, Prometheus, etc.). Developers and contributors need a highly repeatable, zero-configuration way to exercise the complete integrated system without requiring a Kubernetes cluster.

## Decision
Docker Compose defines the local integrated environment for the API, worker, web dashboard, demo API, PostgreSQL, Redis, database migrations, and Prometheus.

## Consequences
- **Expected:** Contributors can run the complete MVP locally and CI can validate the Compose configuration.
- **Result (Observed):** Compose starts the complete stack, and CI validates `docker compose config --quiet`.

## Alternatives Considered

### Alternative 1: Use only local `go run` and `npm run dev` commands
- **Expected if chosen:** Local iteration would be lightweight, but Postgres, Redis, worker, migrations, and Prometheus integration would be easier to miss.

### Alternative 2: Use Kubernetes manifests first
- **Expected if chosen:** Production topology would be more realistic, but local development would be heavier and slower.
