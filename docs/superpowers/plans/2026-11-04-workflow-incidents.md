# Workflow Incidents Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist workflow incidents (open/ack/resolve) from run and scheduler signals, with policies and HTTP API — phases 1 + 1.5, no frontend.

**Architecture:** Mirror expiry: `domain/incident` + CQRS commands/queries; evaluate in worker `RecordResult` (SAVEPOINT) and `worker-scheduler`; outbox → Kafka events → realtime; notify via `notifications.requests`.

**Tech Stack:** Go Fiber, GORM/Postgres (goose), Kafka outbox, Centrifugo via existing ports.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-11-04-workflow-incidents-design.md` (revised)
- Tenant key: `user_id` (known debt vs org_id)
- Routes under `/api`, JSON camelCase, error envelope `{message,errors?,code?}`
- Handler tests required for every new HTTP method; coverage floor ≥95%; patch 100% on handlers
- No realtime from HTTP path — outbox only
- Incident eval failure must not lose run results (SAVEPOINT)
- English code/comments

## File map

| Area | Paths |
| --- | --- |
| Migration | `migrations/00015_incidents.sql` |
| Domain | `internal/domain/incident/*.go` |
| Commands | `internal/application/command/incident/*.go` |
| Queries | `internal/application/query/incident/*.go` |
| Persistence | `internal/infrastructure/persistence/write/incident_*.go` |
| HTTP | `internal/interfaces/http/{dto,presenter,handler}/incident*` + `handler/test/incident/` |
| Wire | `cmd/api/{routes,di}`, `cmd/worker`, `cmd/worker-scheduler` |
| Contract | `internal/contract/notification.go` (+ incident fields) |

---

### Task 1: Migration + domain core

**Files:**
- Create: `migrations/00015_incidents.sql`
- Create: `internal/domain/incident/{kind.go,policy.go,incident.go,signal.go,fingerprint.go,errors.go,repository.go,incident_test.go}`

- [ ] **Step 1:** Add goose migration for `incident_policies`, `signal_state`, `incidents`, `incident_events` + CHECKs + partial unique index + role GRANTs (api/worker/scheduler) per spec.
- [ ] **Step 2:** Domain types: Kind, State, Severity, Policy, Incident, Event, SignalState, Cause; `ParseKind`; `DefaultPolicy(kind)`; `Fingerprint(workflowID, kind, callOrSubject)`; `ReopenWindow = 30m`.
- [ ] **Step 3:** Pure helper `NextCounters(failing bool, failures, successes int) (nf, ns int)` and `ShouldOpen/ShouldClose/ShouldReopen` with tests.
- [ ] **Step 4:** `go test ./internal/domain/incident/...`

---

### Task 2: Persistence repositories

**Files:**
- Create: `internal/infrastructure/persistence/write/incident_model.go`, `incident_repository.go`

- [ ] **Step 1:** GORM models matching migration columns (no domain gorm tags).
- [ ] **Step 2:** Implement `PolicyRepository` (Get/ListByWorkflow/Upsert), `SignalStateRepository` (LockGetOrCreate FOR UPDATE + upsert counters), `IncidentRepository` (FindActive, Insert, Update, List, GetByID), `EventRepository` (Append, ListByIncident). Use `DBWithContext`.
- [ ] **Step 3:** Compile `go build ./internal/infrastructure/persistence/write/...`

---

### Task 3: Policies HTTP API

**Files:**
- Create: command `upsert_policy.go`, query `list_policies.go`
- Create: dto/presenter/handler/ports + `handler/test/incident/`

- [ ] **Step 1:** `ListPolicies` returns all 7 kinds (stored or DefaultPolicy).
- [ ] **Step 2:** `UpsertPolicy` validates open/close 1–20, severity, params (`availability_low`: recoveryThreshold ≥ threshold).
- [ ] **Step 3:** Routes + DI; handler tests (success, unauthorized, invalid kind, validation 422, not found).
- [ ] **Step 4:** `go test ./internal/interfaces/http/handler/test/incident/...`

---

### Task 4: Incidents list / get / acknowledge

**Files:**
- Create: `acknowledge.go`, `list_incidents.go`, `get_incident.go`
- Extend handler

- [ ] **Step 1:** List paginated with state/severity filters; Get with events.
- [ ] **Step 2:** Acknowledge idempotent; outbox domain event `incident.acknowledged.v1`.
- [ ] **Step 3:** Handler tests for all three methods.
- [ ] **Step 4:** Coverage check on `incident_handler.go` → 100% statements.

---

### Task 5: apply_signals engine

**Files:**
- Create: `internal/application/command/incident/apply_signals.go` (+ unit tests)
- Modify: `record_result.go` (SAVEPOINT wrapper)

- [ ] **Step 1:** `ApplySignals(ctx, userID, workflowID, runID, startedAt, []Signal)` — lock state, order/idempotence, open/update/resolve/reopen, events, return notify intents.
- [ ] **Step 2:** Unit tests: double apply same run; older started_at ignored; reopen within 30m resets reminders.
- [ ] **Step 3:** Wire into `RecordResultHandler` inside SAVEPOINT; on error log + metric stub, continue commit.
- [ ] **Step 4:** Build signals for `call_failed`, `workflow_failed`, `call_slow` from run result (skip dependency-skipped for fail/success of those kinds).

---

### Task 6: Observation kinds + scheduler

**Files:**
- Modify: expiry evaluate path to feed tls healthy/failing
- Create: `scan_no_run.go`, `scan_availability.go`, `scan_reminders.go`
- Wire `cmd/worker-scheduler`

- [ ] **Step 1:** `tls_expiry` failing on last threshold/expired; healthy on renewal fingerprint.
- [ ] **Step 2:** `no_run` with mass-silence guard (≥5 and ≥50%); healthy when recently run.
- [ ] **Step 3:** `availability_low` hysteresis threshold / recoveryThreshold.
- [ ] **Step 4:** Reminders 1h / 24h for open unacked.
- [ ] **Step 5:** `chain_broken` last (producer open → skip open).

---

### Task 7: Notify + realtime

**Files:**
- Modify: `internal/contract/notification.go`
- Create: application/event handlers for incident.* → realtime publisher
- Register in worker DI

- [ ] **Step 1:** Extend notification contract with optional `incident_id`.
- [ ] **Step 2:** After apply, publish notification requests (opened/resolved/reminder).
- [ ] **Step 3:** Outbox event handlers publish Centrifugo `incident.*` ids-only.

---

### Task 8: Postgres integration test

**Files:**
- Create: `internal/infrastructure/persistence/incident_apply_integration_test.go` (`//go:build integration`)

- [ ] **Step 1:** Same run twice → counters unchanged.
- [ ] **Step 2:** Older run after newer → ignored.
- [ ] **Step 3:** Reopen within 30m → same id, reminder_count 0.
- [ ] **Step 4:** Document `make integration-tests` includes this package.

---

### Task 9: Verify

- [ ] **Step 1:** `go test ./internal/domain/incident/... ./internal/application/command/incident/... ./internal/interfaces/http/handler/test/incident/...`
- [ ] **Step 2:** Full handler coverage ≥95%.
- [ ] **Step 3:** `go build ./cmd/api ./cmd/worker ./cmd/worker-scheduler`
