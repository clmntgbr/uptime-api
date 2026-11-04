# Workflow incidents — design

**Date:** 2026-11-04 (revised after review)  
**Status:** approved for planning  
**Scope:** Phases 1 + 1.5, API + worker + scheduler only (no frontend)

## Goal

Give each workflow persistent **incidents**: they open when a failure **signal** stays bad long enough, stay open (optionally acknowledged), and resolve when the signal recovers. Rules and one-shot notifications stay as today; incidents add memory, duration, and ack.

## Out of scope

- Frontend (Alerts card, banner, Incidents tab, settings UI)
- Maintenance windows, escalation, relative p95 slow threshold, public status page, org-level policies
- `audit_log` (does not exist yet)
- Changing expiry threshold notifications (they remain); incidents only at last TLS threshold or expired

## Known debt

- **Tenant key is `user_id`**, not `org_id`. Acceptable while there are no organizations. When the Team plan (organizations) ships, `incident_policies`, `incidents` and `incident_events` need an `org_id` column and a backfill. Keep every query behind a repository method so the change stays local.

## Vocabulary

| Term | Meaning |
| --- | --- |
| **Rule** | Call assertion (response time, status code, …) |
| **Signal** | Point-in-time evaluation result, **failing or healthy** (rule fail, call error, run fail, window metric, TLS) |
| **Incident** | Persistent state keyed by fingerprint; opens/closes with consecutive counters |
| **Notification** | Message on open / reminder / resolve via `notifications.requests` |

## Model (B)

One active incident per `(workflow_id, fingerprint)` where:

```text
fingerprint = "{workflow_id}:{kind}:{call_id}"
```

- `call_id` is an empty UUID / omitted segment for workflow-scoped kinds (`workflow_failed`, `availability_low`, `no_run`) and for `tls_expiry` use a stable subject key (e.g. host:port) in place of call_id.
- Multiple failing rules of the same kind on the same call feed **one** incident; causes are listed in `cause` JSON.
- `call_failed` (failed/error) and `call_slow` (response-time rule) remain **separate** incidents.

Skipped calls caused by an upstream failure: **neither failure nor success** for counters; do not open `chain_broken` if a producer incident is already open for that dependency chain.

## Incident kinds (phase 1 + 1.5)

| Kind | Failing signal | Healthy signal (closes) | Default open | Default severity | Default enabled |
| --- | --- | --- | --- | --- | --- |
| `call_failed` | Call `failed` or `error` | Call `success` | 3 consecutive | critical | true |
| `workflow_failed` | Run `failed` | Run `success` | 3 | critical | true |
| `call_slow` | Response-time rule failed | Response-time rule passed | 3 | warning | true (only meaningful if such a rule exists) |
| `chain_broken` | Call `skipped` for missing variable | Call executed (not skipped) | 2 | warning | true |
| `availability_low` | Success runs / total runs over 24h below `params.threshold` (default 99) | Ratio at or above `params.recoveryThreshold` (default 99.5) | `open_after=1` per observation | warning | false |
| `no_run` | No run for `params.multiplier × interval` (default 3) | A run finished within that delay | `open_after=1` per scheduler observation | critical | true |
| `tls_expiry` | Last threshold crossed, or certificate expired | New certificate fingerprint with `days_left` above the last threshold | `open_after=1` | warning; critical if expired | true |

Severities: `info` | `warning` | `critical` (overridable per policy).

For run-based kinds (`call_*`, `workflow_failed`, `chain_broken`), `open_after` / `close_after` count consecutive runs. For observation kinds (`availability_low`, `no_run`, `tls_expiry`), each scheduler/expiry pass is one observation: defaults use `open_after=1` and `close_after=1` (or 2 for availability recovery).

### Healthy signals for observation kinds

Every observation kind must emit **both** failing and healthy signals, otherwise its incident never closes.

- `tls_expiry`: the expiry evaluation feeds the incident evaluator on every observation. A renewed certificate (new fingerprint, `expires_at` later than the last threshold) emits a healthy signal. Earlier thresholds stay notification-only and emit nothing.
- `no_run`: the scheduler pass emits a healthy signal for every enabled workflow that has run recently.
- `availability_low`: **hysteresis.** The incident opens below `params.threshold` (99) and closes only at or above `params.recoveryThreshold` (default 99.5). Without it, three failures out of 288 daily runs keep the ratio under 99 % for 24 hours after recovery.

## Lifecycle

States: `open` → `acknowledged` (user) → `resolved` (automatic only).

| From | Event | To |
| --- | --- | --- |
| none | `consecutive_failures >= open_after` | `open` |
| `open` | Acknowledge | `acknowledged` (stops reminders) |
| `open` / `acknowledged` | `consecutive_successes >= close_after` | `resolved` |
| `resolved` | **One** failing signal within 30 minutes of `resolved_at` | re-`open` same row (`reopen_count++`, `reminder_count = 0`, `last_reminder_at = NULL`, `acknowledged_*` cleared) |
| `resolved` | Failure after 30 minutes | new incident (normal `open_after` rule) |

Defaults: `open_after=3`, `close_after=2` (per-kind overrides allowed, range 1–20).  
Reminders: while `open` and not ack, at 1h then 24h after the last (re)open (scheduler).  
Resolution is always automatic.

## Data model

Tenant key is **`user_id`** (not `org_id`), see Known debt. Align with workflows / expiry.

```sql
-- kinds and states as TEXT + CHECK (or Postgres ENUM); prefer TEXT + CHECK for goose simplicity matching expiry.

CREATE TABLE incident_policies (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  enabled BOOLEAN NOT NULL,
  severity TEXT NOT NULL,
  open_after INT NOT NULL DEFAULT 3 CHECK (open_after BETWEEN 1 AND 20),
  close_after INT NOT NULL DEFAULT 2 CHECK (close_after BETWEEN 1 AND 20),
  params JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (workflow_id, kind),
  CHECK (kind IN ('call_failed','workflow_failed','call_slow','chain_broken','availability_low','no_run','tls_expiry')),
  CHECK (severity IN ('info','warning','critical'))
);

CREATE TABLE signal_state (
  workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
  fingerprint TEXT NOT NULL,
  consecutive_failures INT NOT NULL DEFAULT 0,
  consecutive_successes INT NOT NULL DEFAULT 0,
  last_run_id UUID,
  last_signal_at TIMESTAMPTZ,              -- started_at of the last signal applied, used to ignore older signals
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workflow_id, fingerprint)
);

CREATE TABLE incidents (
  id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  call_id UUID,
  fingerprint TEXT NOT NULL,
  severity TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT 'open',
  cause JSONB NOT NULL DEFAULT '{}',
  opened_at TIMESTAMPTZ NOT NULL,
  acknowledged_at TIMESTAMPTZ,
  acknowledged_by TEXT,
  resolved_at TIMESTAMPTZ,
  reopen_count INT NOT NULL DEFAULT 0,
  opened_by_run_id UUID,
  last_failure_run_id UUID,
  last_reminder_at TIMESTAMPTZ,
  reminder_count INT NOT NULL DEFAULT 0,
  CHECK (state IN ('open','acknowledged','resolved')),
  CHECK (kind IN ('call_failed','workflow_failed','call_slow','chain_broken','availability_low','no_run','tls_expiry')),
  CHECK (severity IN ('info','warning','critical'))
);

CREATE UNIQUE INDEX incidents_one_active
  ON incidents (workflow_id, fingerprint) WHERE state <> 'resolved';

CREATE TABLE incident_events (
  id UUID PRIMARY KEY,
  incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
  at TIMESTAMPTZ NOT NULL,
  type TEXT NOT NULL, -- opened, failing, acknowledged, resolved, reopened, reminder
  run_id UUID,
  payload JSONB NOT NULL DEFAULT '{}'
);
```

Missing policy rows → **virtual defaults** (same pattern as expiry policies).

Grants: `uptime_api` R/W policies+incidents+events+signal_state; `uptime_worker` R/W signal_state+incidents+events, R policies; `uptime_scheduler` R policies/incidents/workflows/runs as needed, W incidents/events/signal_state for no_run/availability/reminders, outbox access as existing.

## Evaluation placement

### Common rules (worker and scheduler)

- **Lock.** Both write `signal_state` and `incidents` for the same fingerprints. Every evaluation starts with `SELECT … FROM signal_state WHERE workflow_id = $1 AND fingerprint = $2 FOR UPDATE` (insert the row first with `ON CONFLICT DO NOTHING`). The partial unique index prevents duplicate incidents, the lock prevents counter races.
- **Idempotence and ordering.** Skip a signal when its run `started_at` is **less than or equal to** `signal_state.last_signal_at`, and also when `last_run_id` equals the run. This covers redelivery, and an older run replayed from a `*.retry` topic after a newer one. Observation kinds use the observation time instead of a run `started_at`.
- **Healthy signals count.** A healthy signal resets `consecutive_failures` and increments `consecutive_successes`, and the reverse for a failing one.

### Worker (`RecordResultHandler`)

In the **same DB transaction** as run persistence (after calls/results are written):

1. Build signals from the finished run (ignore pure dependency `skipped` for fail/success counters).
2. For each fingerprint: lock and load/create `signal_state`, apply the common rules.
3. Update consecutive counters; open / update cause / resolve / reopen per rules above.
4. Append `incident_events`.
5. Collect notification + realtime intents; publish **after commit** (same pattern as expiry notes).

**Isolation.** Run steps 2 to 4 inside a `SAVEPOINT`. If the incident evaluation fails, roll back to the savepoint, log the error, increment `incident_eval_errors_total`, and still commit the run. A bug in incident logic must never lose a run result. The missed signal is accepted: counters catch up on the next run.

`tls_expiry`: on every expiry evaluation, feed the incident evaluator for kind `tls_expiry` (subject fingerprint): a failing signal when the **last** configured threshold is crossed or the certificate is expired, a healthy signal on renewal (see Healthy signals). Earlier thresholds remain expiry-only notifications.

### Scheduler (`worker-scheduler`)

Periodic jobs:

- **`no_run`**: enabled workflows with `now - last_run_at > multiplier * interval` (or never run since enable + age) emit a failing signal. Workflows that ran recently emit a healthy signal.
- **`availability_low`**: for enabled policies, compute the 24h success ratio. Below `params.threshold` emits a failing signal, at or above `params.recoveryThreshold` emits a healthy signal, in between emits nothing.
- **Reminders**: open incidents (not acknowledged) with `reminder_count=0` and age since `opened_at` (or last reopen) ≥1h → remind; `=1` and age ≥24h → second remind; set `last_reminder_at` / `reminder_count`.

**Platform-outage guard for `no_run`.** If the scheduler itself is late, or Kafka or the worker is down, every workflow looks silent at once. Before opening any `no_run` incident, count the silent workflows in the pass. If there are at least 5 silent workflows **and** they make up at least 50 % of the enabled ones (both configurable), open nothing, log a warning and increment `no_run_suppressed_total`. Also skip the pass when the scheduler's own lag exceeds one workflow interval.

## HTTP API

All under `/api`, JWT protected, workflow ownership by `user_id`.

| Method | Path | Behavior |
| --- | --- | --- |
| GET | `/workflows/:id/incidents` | Paginated list; query `state` (csv), `severity`; default sort: active first, then severity, then `opened_at` desc |
| GET | `/incidents/:id` | Detail + events timeline |
| POST | `/incidents/:id/acknowledge` | Idempotent ack; sets `acknowledged_by` from auth user id/email string |
| GET | `/workflows/:id/incident-policies` | All 7 kinds (stored or default) |
| PUT | `/workflows/:id/incident-policies/:kind` | Upsert; validate open/close 1–20, severity, params per kind (`availability_low`: `recoveryThreshold` must be ≥ `threshold`) |

JSON camelCase. Error envelope `{ message, errors?, code? }`. Codes: `UNKNOWN_INCIDENT_KIND`, `INVALID_INCIDENT_POLICY`, etc.

Profiles (Strict / Recommended / Calm) are **client presets** only; API stores raw fields.

Optional create-time `incidentPolicies` on workflow create is **not** required in this phase (defaults virtual); can mirror expiry later.

## Notifications & realtime

After commit:

- Kafka `notifications.requests`: kinds `incident.opened`, `incident.reminder`, `incident.resolved` with workflow/incident ids, kind, call, severity, message. Extend `contract.NotificationRequest` with optional `incident_id` / keep message self-contained. `worker-notify` stays log-only.
- Centrifugo user channel: types `incident.opened`, `incident.acknowledged`, `incident.resolved`, `incident.reopened` — **ids only**.

All state changes that need realtime (including acknowledge) persist an outbox domain event in the same transaction; the main worker handler publishes Centrifugo ids-only events. Do not publish realtime from the HTTP request path.

## Package layout

Follow existing CQRS:

```text
internal/domain/incident/          # kinds, policy defaults, fingerprint, evaluate pure helpers
internal/application/command/incident/  # upsert policy, acknowledge, apply_signals, scan_no_run, scan_availability, scan_reminders
internal/application/query/incident/    # list, get, list_policies
internal/infrastructure/persistence/write/  # repos
internal/interfaces/http/handler/incident_*.go + test/incident/
migrations/00015_incidents.sql
```

Wire in `cmd/api/di`, `cmd/worker` (record_result + event handlers), `cmd/worker-scheduler`.

Implementation order suggestion: policies and API first, then `call_failed` / `workflow_failed` / `call_slow`, then `tls_expiry`, `no_run`, `availability_low`, reminders, and `chain_broken` last (it needs the producer-call mapping).

## Testing

- Domain unit tests: counters, reopen window (one failure reopens, counters reset), skipped ignored, fingerprint, availability hysteresis
- Command unit tests: apply_signals idempotence, older signal ignored
- Handler HTTP tests: full coverage on new `*_handler.go` (project ≥95% floor, patch 100% on handler changes)
- **At least one integration test against a real PostgreSQL** (not mocks) for `apply_signals`, because the partial unique index, the row lock and the reopen logic cannot be proven with mocks:
  - same run applied twice → counters unchanged
  - run N-1 applied after run N → ignored
  - worker and scheduler applying a signal on the same fingerprint concurrently → one incident, correct counters
  - failure within 30 minutes of resolution → same row reopened, `reminder_count` reset
- Scheduler tests: mass silence suppresses `no_run`, a single silent workflow opens one

## Success criteria

1. Empty workflow with defaults: no rows; list policies returns 7 virtual policies.
2. Three consecutive call failures open one `call_failed` incident; two successes resolve it.
3. Ack stops reminders; resolve still automatic on recovery.
4. Scheduler opens `no_run` when silent too long and closes it when runs resume; `availability_low` only if policy enabled, and it closes only at the recovery threshold.
5. TLS last threshold / expired opens `tls_expiry` incident without duplicating earlier threshold spam, and a renewed certificate closes it.
6. A run replayed out of order, or twice, never changes the counters.
7. A mass silence (platform outage) does not open one `no_run` incident per workflow.
8. A failure in incident evaluation never prevents the run from being persisted.
9. Handler tests green; coverage floor held; the PostgreSQL integration test passes.