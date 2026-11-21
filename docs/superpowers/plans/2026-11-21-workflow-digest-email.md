# Workflow Digest Email Implementation Plan

> **For agentic workers:** Implement task-by-task. Steps use checkbox syntax.

**Goal:** Per-workflow daily/weekly/monthly summary emails (opt-in, UTC 08:00).

**Architecture:** Settings flags on `workflow_notification_settings`; `worker-scheduler` aggregates run/incident counters and publishes `workflow.digest` to `notifications.requests`; shared mail layout.

**Tech Stack:** Go, Postgres, Kafka, Fiber API, Next.js UI.

## Global Constraints

- Cadences independent (all three allowed); default all off
- Windows UTC; send 08:00 the morning after the period ends
- Skip empty periods; idempotent via `processed_events`
- English code/comments

### Task 1: Settings (migration + domain + API + UI)
### Task 2: Mail template `workflow.digest`
### Task 3: Digest command + scheduler wire
### Task 4: Docs + verify tests
