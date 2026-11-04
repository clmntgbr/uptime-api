# Workflow incidents — frontend (card)

**Date:** 2026-11-04  
**Status:** approved for planning  
**Scope:** Compact active-incidents card on the workflow Overview page + Acknowledge. No Incidents tab, no policy settings UI.

Companion to `2026-11-04-workflow-incidents-design.md` (API).

## Goal

On `/workflows/[id]` Overview, after the charts, show a **single-column** card listing active incidents for that workflow, with an Acknowledge action on open ones.

## Placement

- Parent: Overview tab, **below** `WorkflowCharts`.
- Layout: `grid md:grid-cols-3` (or equivalent); the card occupies **one cell** (~1/3 width on desktop, full width on mobile). Empty cells are unused for now.
- If there are **no** active incidents (`open` or `acknowledged`), **do not render** the card (and do not reserve the grid).

## Card content

- Title: `Incidents`
- One row per active incident (from `GET /workflows/:id/incidents?state=open,acknowledged`, reasonable page size e.g. `limit=20`):
  - Kind label (humanized from API kind, e.g. `call_failed` → “Call failed”)
  - Severity badge (`info` / `warning` / `critical`)
  - Relative `openedAt` (e.g. “2h ago”)
  - State: show “Acknowledged” muted when `state === acknowledged`
- **Acknowledge** button only when `state === open`; disabled while the mutation is in flight for that id.
- On success: toast optional; invalidate the incidents query (realtime will also refresh).
- On error: toast with API `message`.

No expand/detail drawer in this phase. No resolved history.

## Data layer (`uptime-app`)

Mirror the expiry module:

| Piece | Path |
| --- | --- |
| Types | `lib/incident/types.ts` |
| Client fetch | `lib/incident/api.ts` |
| Hooks | `lib/incident/hooks.ts` |
| Query keys | `lib/query/keys.ts` → `incidents` |
| BFF | `app/api/workflows/[id]/incidents/route.ts` (GET proxy) |
| BFF | `app/api/incidents/[id]/acknowledge/route.ts` (POST proxy) |
| UI | `components/incident/workflow-incidents-card.tsx` |
| Mount | `app/(private)/workflows/[id]/page.tsx` Overview |

Proxy auth/headers the same way as existing workflow/expiry BFF routes.

List query filters: `state=open,acknowledged` (CSV as API already supports). Sort is server default (active first, severity, `openedAt` desc).

## Realtime

Extend Centrifugo user-stream handling:

- Event types: `incident.opened`, `incident.acknowledged`, `incident.resolved`, `incident.reopened`
- Payload: ids only (`incidentId`, `userId`, `workflowId`) — invalidate `queryKeys.incidents` for that `workflowId` (debounced like expiry if useful).

## Out of scope

- Incidents tab / full timeline
- Incident policy settings UI
- Dashboard-wide incident list
- Creating/editing policies from the card

## Success criteria

1. Workflow with no active incidents: Overview unchanged (no empty card).
2. Open incident appears in the bottom one-column card after charts.
3. Acknowledge moves the row to acknowledged (button gone) without full page reload; Centrifugo events refresh another open tab.
4. Mobile: card stacks full-width under charts.
