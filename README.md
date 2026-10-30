# Go API Template

Go/Fiber API template with **Clean Architecture**, **CQRS**, **Outbox pattern**, **Kafka**, **Centrifugo**, **PostgreSQL**, and **Clerk** authentication.

## Features

- CQRS: command / query / event handlers under `internal/application`
- Domain events (`user.created.v1`) + transactional outbox
- Kafka worker (outbox relay + consumer + handler registry) on the `events` topic
- API workflow monitoring: `worker-scheduler` (leases + jobs on `workflow.jobs`), `worker-calls` (HTTP execution, no database access), `worker` (records results from `workflow.results`)
- Centrifugo realtime (outbox → worker → publish to `users:{id}`)
- Offset pagination + query validation helpers
- Clerk JWT auth + webhook sync (Svix)
- Docker Compose local stack (API, worker, worker-scheduler, worker-calls, Postgres, Kafka, Centrifugo, ngrok)

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
  api/          HTTP server
  worker/       Outbox relay + Kafka consumers (events, workflow results)
  worker-scheduler/  Leases due workflows at hh:mm:00, publishes jobs, expires old runs
  worker-calls/ Executes jobs (HTTP calls, rules), publishes results
  cli/          Migrations, workflow:import, variable:set, keys:generate
internal/
  contract/               Versioned Kafka messages (jobs, results)
  engine/                  Workflow execution engine (variables, extraction, rules, masking)
  domain/                 Aggregates, events, ports, paginate
  application/
    command/              Write-side handlers
    query/                Read-side handlers
    event/                Async event handlers (+ realtime publish)
    realtime/             WS type helpers (entity.action)
    registry/             Event handler registry
  infrastructure/
    persistence/write|read|outbox|processed
    messaging/kafka
    httpclient    SSRF-safe HTTP executor
    crypto        AES-GCM at rest, sealed job secrets
    centrifugo
    clerk|config
  interfaces/http/        Handlers, middleware, DTO, presenter, validation
```

### Sync command + async event flow

```
HTTP / webhook
  → CreateUser command (same DB transaction)
      → save User aggregate
      → StoreEvents(outbox)
  → 201 response

worker (outbox relay)
  → poll unpublished outbox rows
  → publish to the Kafka topic events (key = aggregate id)
  → mark published_at

worker (consumer)
  → HandlerRegistry.Dispatch(event type)
  → log / notify handlers
  → publish_user_*_realtime → Centrifugo users:{userId}
```

Supported domain events: `user.created.v1`, `user.updated.v1`, `user.deleted.v1` (all written to outbox on the matching command).

### Idempotence + retry / DLQ

- Each domain event has a stable `eventId` (set when recorded on the aggregate).
- Handlers are wrapped with dedup on `(event_id, handler_name)` via `processed_events`.
- Kafka topology per topic: `<topic>` → `<topic>.retry` (`not_before` header, separate consumer group) → `<topic>.dlq` after 5 attempts or on non-retryable errors.
- Offsets are committed up to the oldest record still in flight, so parallel processing never loses a record.
- Topics are created by `kc` (`deploy/kafka/topics.sh`); auto-creation is disabled.

## Getting Started

```bash
cp .env.dist .env
# fill Clerk keys

make dev
make migrate
```

| Service | Host port | Notes |
|---|---|---|
| API | `4000` | Air hot reload |
| Worker | — | relay + consumer |
| Postgres | `9543` | |
| Scheduler | — | single active instance (advisory lock) |
| Worker-calls | — | executes jobs, no database access |
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

Handler HTTP tests live under `internal/interfaces/http/handler/test/`. See [docs/handler-tests.md](docs/handler-tests.md).

```bash
go test ./internal/interfaces/http/handler/test/... -race \
  -coverpkg=go-api/internal/interfaces/http/handler
```

CI enforces ≥95% handler statement coverage (Codecov flag `handler`, patch 100%).

## API

### Health

- `GET /livez`, `/readyz`, `/startupz`

### Protected

- `GET /api/users/me` — Bearer Clerk JWT (query side / read repository)
- `GET /api/realtime/connection` — Centrifugo JWT + channel + `wsUrl`

### Webhooks

- `POST /webhooks/clerk` — Svix-verified; `user.created` / `updated` / `deleted`

## CQRS notes

- Commands write through aggregates (`NewUser` records `UserCreated`) and persist events in `outbox_events` in the **same** GORM transaction.
- Queries use `UserReadRepository` (SQL projection / `UserView`), not domain mutation.
- Event types are versioned (`user.created.v1`).
- One worker process runs outbox relay + consumer; scale the worker horizontally as needed.
- Start with a single `events` topic; consumers dispatch by event type through the registry.

## Environment

See `.env.dist`. Messaging / realtime:

- `KAFKA_BROKERS`, `KAFKA_SASL_MECHANISM`, `KAFKA_USERNAME`, `KAFKA_PASSWORD`, `KAFKA_TLS`
- `SECRETS_ENCRYPTION_KEY`, `JOB_SEAL_PUBLIC_KEY`, `JOB_SEAL_PRIVATE_KEY` (generate with `make keys-generate`)
- `SSRF_ALLOW_PRIVATE` (dev only), `HEARTBEAT_URL`, `RESULTS_RETENTION_DAYS`
- `OUTBOX_POLL_INTERVAL` (default `2s`)
- `WORKER_CONCURRENCY` (default `4`)
- `CENTRIFUGO_URL`, `CENTRIFUGO_API_KEY`, `CENTRIFUGO_TOKEN_SECRET`, `CENTRIFUGO_PUBLIC_WS_URL`
- `RATE_LIMIT_MAX` (requests per minute per IP; `0` disables)

## Extending

1. Add aggregate under `internal/domain/<name>/` (`entity.go`, `events.go`, `repository.go`)
2. Add command/query handlers under `internal/application/...`
3. Implement write/read repos under `internal/infrastructure/persistence/`
4. Register event handlers in `cmd/worker/di` (including `publish_*_realtime` if needed)
5. Wire HTTP in `cmd/api/di` + `routes.go`
6. Add a SQL migration in `migrations/` and a persistence model under `internal/infrastructure/persistence/`
7. Run `make migrate` (applies SQL + fails on model/DB drift)

## Architecture rules

Conventions live in [`.cursor/rules/architecture.mdc`](.cursor/rules/architecture.mdc).

- User / auth: [docs/user.md](docs/user.md)
- Webhooks: [docs/webhooks.md](docs/webhooks.md)
- Realtime: [docs/realtime.md](docs/realtime.md)
- Handler tests: [docs/handler-tests.md](docs/handler-tests.md)
