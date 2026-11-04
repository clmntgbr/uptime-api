# Workflow notification settings — design

**Date:** 2026-11-04  
**Status:** approved for implementation  
**Scope:** API + worker + UI (email channel only)

## Goal

Per-workflow notification config: one recipient email list, separate toggles for **incidents**, **TLS expiry**, and **runs**. Slack later as another channel with its own recipients.

## Defaults

| Family | Events | Default |
| --- | --- | --- |
| Incidents | opened, reminder, resolved | enabled |
| TLS | threshold / expired | enabled |
| Runs | success, failed, error, skipped | **disabled** |

When runs are enabled: `run_mode` defaults to `transition` (mail only when status changes). Alternative: `every` (each matching run).

## Model

```sql
CREATE TABLE workflow_notification_settings (
  workflow_id UUID PRIMARY KEY REFERENCES workflows(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  emails TEXT[] NOT NULL DEFAULT '{}',
  incidents_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  tls_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  runs_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  run_statuses TEXT[] NOT NULL DEFAULT '{}',
  run_mode TEXT NOT NULL DEFAULT 'transition'
    CHECK (run_mode IN ('every', 'transition')),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- Emails: lowercase, deduped, max 20, empty allowed (no mail).
- On workflow create: insert row with `emails = [creator.email]` when non-empty.
- Missing row → same virtual defaults as above (creator email only when creating).

## Delivery

- Publishers (incident / expiry / new run notify) resolve settings + recipients.
- Empty emails → skip. Family disabled → skip.
- `worker-notify` sends to each address in the request (extend contract with `recipients []string` or one message per recipient). Prefer **recipients on the Kafka request** so notify stays egress-only without DB for routing (still may keep DB for fallback).

## API

- `GET /workflows/:id/notification-settings`
- `PUT /workflows/:id/notification-settings`
- Create workflow may accept optional `notificationSettings`.

## UI

Workflow settings drawer: recipients chips, three family sections, empty-list warning.

## Out of scope

External email confirmation, unsubscribe links (1.5), Slack, maintenance windows.
