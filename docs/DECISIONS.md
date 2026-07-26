# Wreckr Decisions

Last updated: 2026-07-26

This document serves as an index for all important product and technical architectural decisions made throughout the lifecycle of Wreckr. 

Each detailed decision is recorded as an **Architecture Decision Record (ADR)** in the `docs/ADR/` folder, detailing the status, context, decision, expected/observed consequences, and alternatives considered.

## Architecture Decision Records (ADRs)

| ID | Title | Status | Link |
| :--- | :--- | :---: | :--- |
| **ADR-001** | Build Wreckr Black-Box First | Accepted | [adr-001-black-box-first.md](ADR/adr-001-black-box-first.md) |
| **ADR-002** | Use JSON Scenario Files As The Core Contract | Accepted | [adr-002-json-scenario-contract.md](ADR/adr-002-json-scenario-contract.md) |
| **ADR-003** | Keep The Runner Core In Go | Accepted | [adr-003-runner-core-in-go.md](ADR/adr-003-runner-core-in-go.md) |
| **ADR-004** | Separate CLI Execution From API Worker Execution | Accepted | [adr-004-separate-cli-from-api-worker.md](ADR/adr-004-separate-cli-from-api-worker.md) |
| **ADR-005** | Support Memory And PostgreSQL Stores Behind One Interface | Accepted | [adr-005-memory-and-postgresql-stores.md](ADR/adr-005-memory-and-postgresql-stores.md) |
| **ADR-006** | Make Scenario Versions Immutable | Accepted | [adr-006-immutable-scenario-versions.md](ADR/adr-006-immutable-scenario-versions.md) |
| **ADR-007** | Persist Run Events And Stream With SSE | Accepted | [adr-007-persist-run-events-stream-sse.md](ADR/adr-007-persist-run-events-stream-sse.md) |
| **ADR-008** | Add Guardrails Before Production-Target Features | Accepted | [adr-008-guardrails-before-production-target-features.md](ADR/adr-008-guardrails-before-production-target-features.md) |
| **ADR-009** | Introduce Target Management As A Reusable Control-Plane Primitive | Accepted | [adr-009-target-management-primitive.md](ADR/adr-009-target-management-primitive.md) |
| **ADR-010** | Use Docker Compose As The Complete Local Stack | Accepted | [adr-010-docker-compose-local-stack.md](ADR/adr-010-docker-compose-local-stack.md) |
| **ADR-011** | Build The Dashboard As An Operational Console, Not A Landing Page | Accepted | [adr-011-dashboard-as-operational-console.md](ADR/adr-011-dashboard-as-operational-console.md) |
| **ADR-012** | Treat k6, Object Storage, And Kubernetes Jobs As Later Execution Layers | Accepted | [adr-012-k6-object-storage-k8s-later.md](ADR/adr-012-k6-object-storage-k8s-later.md) |
| **ADR-013** | Add Visual Scenario Builder Alongside JSON/YAML Editing | Accepted | [adr-013-visual-scenario-builder.md](ADR/adr-013-visual-scenario-builder.md) |

---

For any questions about these decisions, please consult the respective `.md` file in the `docs/ADR/` directory.
