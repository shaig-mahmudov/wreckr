# Wreckr

**Break your backend before production does.**

Wreckr is a production scenario testing tool for backend systems. It simulates real-world failure scenarios like load spikes, race conditions, duplicate transactions, broken idempotency, retry storms, weak rate limiting, queue backlogs, and slow dependencies.

Unlike unit tests, Wreckr tests a system from the outside under production-like pressure. The tested backend can be written in Go, C#, Java, Node.js, Python, or any other stack.

## Current MVP

This repository now contains a working MVP control plane and runner:

- Go scenario engine
- black-box HTTP runner
- load, burst, spike, race, and retry-storm traffic modes
- optional request-rate pacing with `traffic.rate_per_second`
- response and probe-based business invariants
- latency/error/status reports
- HTTP API for scenarios, scenario versions, runs, reports, event timelines, live event streams, and run cancellation
- target management for defining reusable backend environments
- pluggable storage with memory and PostgreSQL implementations
- immutable scenario versions linked to historical runs and reports
- persistent run event timelines for lifecycle, request, assertion, threshold, and invariant events
- run guardrails for concurrency, request rate, duration, request body size, and target allowlists
- Redis + Asynq background worker orchestration for API-created runs
- PostgreSQL migrations for the persistent control-plane schema
- CLI runner
- intentionally vulnerable demo API
- Next.js dashboard with API connectivity, run list, live event timeline, and report view
- GitHub Actions CI for backend, frontend, and Docker Compose validation
- Docker Compose scaffold for API, demo target, Postgres, Redis, Prometheus, and web

The CLI still runs scenarios in-process for local files. API-created runs are enqueued to Redis and executed by the worker. Object storage, k6, and Kubernetes jobs remain planned as later orchestration layers around this core.

## Quick Start

Run the intentionally vulnerable demo API:

```bash
go run ./examples/demo-api/cmd
```

In another terminal, run a production-style idempotency race scenario:

```bash
go run ./apps/api/cmd/wreckr run ./examples/scenarios/checkout-idempotency-race.json
```

Expected result: the scenario fails because the demo API creates duplicate orders for simultaneous checkout requests.

Run the Wreckr API:

```bash
go run ./apps/api/cmd/api
```

Health check:

```bash
curl http://localhost:8080/healthz
```

Use PostgreSQL storage instead of memory:

```bash
docker compose up -d postgres
docker compose run --rm migrate
WRECKR_STORE=postgres go run ./apps/api/cmd/api
```

Run the API with the background worker locally:

```bash
docker compose up api worker redis postgres
```

## Run Guardrails

The API validates every scenario before it is stored or executed. Configure safety limits with environment variables:

```bash
WRECKR_MAX_CONCURRENCY=1000
WRECKR_MAX_REQUEST_RATE_PER_SECOND=5000
WRECKR_MAX_RUN_DURATION_SECONDS=300
WRECKR_MAX_REQUEST_BODY_BYTES=1048576
WRECKR_TARGET_ALLOWLIST=api.example.com,*.internal.example.com
```

The configured request-rate limit caps outgoing traffic even when a scenario omits `traffic.rate_per_second`; set `traffic.rate_per_second` when a scenario should run below that cap. Absolute request URLs are rejected unless they match the target or the configured allowlist.

## Example Scenario

```json
{
  "version": 1,
  "name": "checkout-idempotency-race",
  "target": {
    "base_url": "http://localhost:9090"
  },
  "traffic": {
    "type": "race",
    "concurrency": 2,
    "iterations": 1
  },
  "setup": [
    {
      "name": "reset-demo-state",
      "method": "POST",
      "path": "/reset",
      "expect": {
        "status": [200]
      }
    }
  ],
  "requests": [
    {
      "name": "checkout",
      "method": "POST",
      "path": "/checkout",
      "headers": {
        "Idempotency-Key": "same-key-123"
      },
      "json": {
        "userId": "user-123",
        "sku": "item-abc",
        "quantity": 1
      },
      "expect": {
        "status": [201, 409]
      }
    }
  ],
  "invariants": [
    {
      "name": "only-one-order-created",
      "type": "http_probe",
      "method": "GET",
      "path": "/orders?userId=user-123&sku=item-abc",
      "expect": {
        "json_path": "$.count",
        "equals": 1
      }
    }
  ]
}
```

## Architecture Direction

```text
Next.js dashboard
        |
Go API -------------- PostgreSQL
        |
Redis + Asynq
        |
Runner worker ------- Object storage
        |
Docker/k6 today, Kubernetes Jobs later
        |
Target backend
```

See [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) for the staged implementation plan.

See [docs/PROJECT_STATUS.md](docs/PROJECT_STATUS.md) for the current implementation state, known gaps, and recommended next milestones.
