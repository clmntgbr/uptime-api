# Uptime API

Go/Fiber API for **API workflow monitoring**: ordered HTTP calls, variables, rules, runs, incidents, certificate expiry, and email notifications.

Built with **Clean Architecture**, **CQRS**, **Outbox**, **Kafka**, **Centrifugo**, **PostgreSQL**, and **Clerk** authentication.

Product details (concepts, rules, alerts): [docs/main.md](docs/main.md). Security notes: [docs/security.md](docs/security.md).

## Features

- Workflows of ordered HTTP calls (variables `global` / `static` / call-extracted, readable alert rules, TLS certificate checks)
- Progressive run recording: `call.started` / `call.finished` while a run is open, then `workflow.finished`
- Dashboard statistics (24h counters, runs hourly + daily, call duration series)
- Incidents + per-kind policies, acknowledgement, realtime updates
- Certificate expiry policies and items
- Per-workflow notification settings (email for incidents / TLS)
- Header/query key & value suggestions across the user’s calls
- CQRS + domain events + transactional outbox
- Kafka workers: outbox relay, scheduler leases, call execution, result persistence
- Centrifugo realtime on `users:{id}` (`workflow.*`, `call.*`, `incident.*`, …)
- Clerk JWT auth + Svix webhook user sync
- Docker Compose local stack

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP | Fiber v3 |
| ORM | GORM + PostgreSQL |
| Auth | Clerk (JWKS + webhooks) |
| Messaging | Kafka (KRaft, franz-go) |
| Realtime | Centrifugo v5 |
| CLI | Cobra |
| Dev | Docker, Air, golangci-lint |

## Architecture

```
cmd/
  api/               HTTP server
  worker/            Outbox relay + Kafka consumers (events, workflow.results)
  worker-scheduler/  Leases due workflows, publishes jobs, expires stale leases
  worker-calls/      Executes jobs (HTTP + rules), publishes results / call progress
  cli/               Migrations, workflow:import, variable:set, keys:generate
internal/
  contract/          Versioned Kafka messages (jobs, results, call progress)
  engine/            Pure execution (variables, extraction, rules) via ports
  domain/            Aggregates, events, ports, paginate
  application/
    command/         Sync write use-cases
    query/           Read models
    event/           Async handlers (+ realtime publish)
    realtime/        WS type helpers (entity.action)
    registry/        Event handler registry
  infrastructure/
    persistence/write|read|outbox|processed
    messaging/kafka
    httpclient       SSRF-safe HTTP executor
    crypto           AES-GCM at rest, sealed job secrets
    centrifugo
    clerk|config
  interfaces/http/   Handlers, middleware, DTO, presenter, validation
migrations/          Goose SQL (schema source of truth)
```

### Sync command + async event flow

```
HTTP / webhook
  → command handler (same DB transaction)
      → save aggregate
      → StoreEvents(outbox)
  → 2xx response

worker (outbox relay)
  → poll unpublished outbox rows
  → publish to Kafka topic `events` (key = aggregate id)
  → mark published_at

worker (consumer)
  → HandlerRegistry.Dispatch(event type)
  → side effects + publish_*_realtime → Centrifugo users:{userId}
```

### Workflow execution path

```
worker-scheduler  → Kafka workflow.jobs
worker-calls      → HTTP calls, call progress + final result on workflow.results
worker            → EnsureRunning / UpsertCallRun / finalize run
                  → outbox: call.started|finished, workflow.started|finished
                  → Centrifugo + optional notification requests
```

### Idempotence + retry / DLQ

- Stable `eventId` on domain events (set when recorded on the aggregate).
- Dedup on `(event_id, handler_name)` via `processed_events`.
- Per topic: `<topic>` → `<topic>.retry` (`not_before`, separate group) → `<topic>.dlq`.
- Offsets commit up to the oldest in-flight record.
- Topics are created by `kc` (`deploy/kafka/topics.sh`); auto-creation is disabled.

## Getting Started

```bash
cp .env.dist .env
# fill Clerk keys + crypto keys (make keys-generate)

make dev
make migrate
```

| Service | Host port | Notes |
|---|---|---|
| API | `4000` | Air hot reload |
| Worker | — | outbox relay + consumers |
| Scheduler | — | single active instance (advisory lock) |
| Worker-calls | — | executes jobs, no database access |
| Postgres | `9543` | |
| Kafka | `9094` | external listener (`localhost:9094`) |
| Centrifugo | `8000` | WS + HTTP API |
| ngrok | `4040` | webhook tunnel |

## Makefile

| Command | Description |
|---|---|
| `make dev` | Start compose.dev stack |
| `make build` | Start stack with `--build` |
| `make dev-down` | Stop stack |
| `make dev-logs` | Follow API + worker logs |
| `make worker-logs` | Follow worker logs only |
| `make tests` | Handler HTTP tests (in api container) |
| `make unit-tests` | Domain, engine, crypto, execution and CLI unit tests |
| `make integration-tests` | Repository and run-flow tests against the dev Postgres |
| `make workflow-import FILE=… USER=…` | Import a workflow YAML for a user (UUID or Clerk ID) |
| `make keys-generate` | Print new encryption and job sealing keys |
| `make purge` | Empty every table except `users` and `goose_db_version` |
| `make coverage` | Handler coverage profile |
| `make coverage-html` | HTML coverage report |
| `make migrate` | Apply SQL migrations + schema drift check |
| `make migrate-check` | Fail if persistence models ≠ DB columns |
| `make migrate-status` | Goose migration status |
| `make lint` | golangci-lint --fix |
| `make shell` | Shell into API container |

## Testing

Handler HTTP tests live under `internal/interfaces/http/handler/test/` (ports in `*_ports.go`, Fiber `app.Test` with mocks). Architecture conventions: [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc).

```bash
go test ./internal/interfaces/http/handler/test/... -race \
  -coverpkg=go-api/internal/interfaces/http/handler
```

CI enforces ≥95% handler statement coverage (Codecov flag `handler`, patch 100%).

## API (overview)

### Health

- `GET /livez`, `/readyz`, `/startupz`

### Auth / realtime

- `GET /api/users/me` — Bearer Clerk JWT
- `GET /api/realtime/connection` — Centrifugo JWT + channel + `wsUrl`
- `POST /webhooks/clerk` — Svix-verified user sync

### Core product (protected)

- `GET /api/statistics` — dashboard counters + runs/calls series (hourly + daily)
- Workflows CRUD, run/stop, context (latest run, variables)
- Calls CRUD, reorder, copy; call variables & response keys
- Suggestions: `…/suggestions/header-keys|header-values|query-keys|query-values`
- Runs list/detail
- Incidents + policies; notification settings
- Expiry policies + items
- Global / static variables

JSON uses camelCase. Errors: `{ "message", "errors?", "code?" }`.

## Environment

See `.env.dist`. Messaging / realtime / crypto:

- `KAFKA_BROKERS`, `KAFKA_SASL_MECHANISM`, `KAFKA_USERNAME`, `KAFKA_PASSWORD`, `KAFKA_TLS`
- `SECRETS_ENCRYPTION_KEY`, `JOB_SEAL_PUBLIC_KEY`, `JOB_SEAL_PRIVATE_KEY` (`make keys-generate`)
- `SSRF_ALLOW_PRIVATE` (dev only), `HEARTBEAT_URL`, `RESULTS_RETENTION_DAYS`
- `OUTBOX_POLL_INTERVAL` (default `2s`)
- `WORKER_CONCURRENCY` (default `4`)
- `CENTRIFUGO_URL`, `CENTRIFUGO_API_KEY`, `CENTRIFUGO_TOKEN_SECRET`, `CENTRIFUGO_PUBLIC_WS_URL`
- `RATE_LIMIT_MAX` (requests per minute per IP; `0` disables)

## Extending

1. Add aggregate under `internal/domain/<name>/`
2. Add command/query handlers under `internal/application/...`
3. Implement write/read repos under `internal/infrastructure/persistence/`
4. Register event handlers in `cmd/worker/di` (including realtime publishers if needed)
5. Wire HTTP in `cmd/api/di` + `routes.go` with `*_ports.go` + handler tests
6. Add a SQL migration in `migrations/` and update persistence models
7. Run `make migrate` (applies SQL + fails on model/DB drift)
