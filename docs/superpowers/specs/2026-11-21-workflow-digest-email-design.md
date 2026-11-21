# Workflow digest email — design

**Date:** 2026-11-21  
**Status:** approved for implementation  
**Scope:** API + worker-scheduler + mail template + UI (email channel only)

## Goal

Per-workflow **summary** emails the owner can opt into independently at three cadences: daily, weekly, and monthly. Content stays light (run counters + incident counters). Empty periods are not mailed.

## Decisions

| Topic | Choice |
| --- | --- |
| Scope | Per workflow (not account aggregate) |
| Cadences | Independent toggles: daily / weekly / monthly (all three allowed) |
| Default | All off |
| Content | Light: runs + success rate + incidents opened/resolved |
| Schedule | Fixed UTC 08:00 via `worker-scheduler` (same hour as TLS scan) |
| Recipients | Existing `workflow_notification_settings.emails` |
| Empty period | Skip send (no runs and no incident open/resolve in window) |

## Settings model

Extend `workflow_notification_settings`:

```sql
ALTER TABLE workflow_notification_settings
  ADD COLUMN digest_daily   BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN digest_weekly  BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN digest_monthly BOOLEAN NOT NULL DEFAULT FALSE;
```

API `GET/PUT /workflows/:id/notification-settings` exposes camelCase:
`digestDaily`, `digestWeekly`, `digestMonthly`.

UI: section **Summary** with three switches under the existing notification card.

## Period windows (UTC)

Send hour: **08:00 UTC**. Each digest covers the period that just ended, then is sent the morning after that period closes.

| Cadence | Covers | Sent |
| --- | --- | --- |
| daily | Previous calendar day `[D 00:00, D+1 00:00)` | Every day at 08:00 UTC (the morning after that day) |
| weekly | Previous Mon→Sun `[Mon 00:00, next Mon 00:00)` | Following **Monday** at 08:00 UTC |
| monthly | Previous calendar month `[1st 00:00, next 1st 00:00)` | **1st** of the following month at 08:00 UTC |

Examples (UTC):

- Daily for Tuesday → Wednesday 08:00
- Weekly for Mon 10 Nov → Sun 16 Nov → Monday 17 Nov 08:00
- Monthly for November → 1 December 08:00

On a Monday that is also the 1st, a workflow with all three flags may receive up to three digests in the same tick (different windows).

## Mail contract

- Kind: `workflow.digest`
- `type`: `daily` | `weekly` | `monthly`
- `subject`: workflow name
- `message`: short human summary (period label + key numbers)
- Detail rows: period start/end, run totals, success rate, incidents opened/resolved
- Template file: `internal/infrastructure/mail/workflow_digest.go`

Subject line example: `[Uptime] Summary · weekly · Checkout`

## Stats to aggregate

For workflow `W` over `[from, to)`:

| Metric | Source |
| --- | --- |
| runs total / success / failed / error | `runs` finished in window (`finished_at`) |
| success rate | `success / (success+failed+error)`; `0` if denominator 0 |
| incidents opened | `incident_events` type `opened` (or incident `opened_at`) in window |
| incidents resolved | `incident_events` type `resolved` (or `resolved_at`) in window |

Skipped/running runs are excluded from the rate denominator (and may be omitted from the mail).

## Producer

`worker-scheduler` tick at **08:00 UTC**:

1. Load settings rows where the relevant digest flag is true and `emails` is non-empty.
2. For each due cadence today, compute window, aggregate stats.
3. If empty (0 runs and 0 opened and 0 resolved) → skip.
4. Idempotency claim: `processed_events` key  
   `event_id = workflow.digest:{cadence}:{periodStartRFC3339}:{workflowID}`  
   `handler_name = send_workflow_digest`  
   Already claimed → skip.
5. Publish `contract.NotificationRequest` with `recipients` from settings.

No separate Kafka topic; reuse `notifications.requests`.

## Gate

Send only when:

- matching digest flag is true
- `len(emails) > 0`
- period is non-empty (see above)
- idempotency claim succeeds

System mails (`workflow.disabled`, `one_shot.done`) remain unaffected.

## Docs

Update `docs/emails.md` catalogue + settings table.

## Out of scope (v1)

- User timezone / preferred send hour
- Account-level digest
- TLS / top failure causes in the summary
- Digest of feed lines
- Staggered send times per workflow
- Slack channel

## Acceptance

- [ ] Migration + API + UI expose three independent digest toggles (default off)
- [ ] Scheduler sends daily/weekly/monthly at 08:00 UTC with correct windows
- [ ] Mail renders via `workflow.digest` template
- [ ] Empty periods and duplicate ticks do not send
- [ ] `docs/emails.md` updated
