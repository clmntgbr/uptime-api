# User & authentication

## Overview

Users are provisioned from **Clerk** via webhooks. The API authenticates requests with a **Clerk JWT** (Bearer token).

`ActiveProjectID` exists on the domain user / HTTP context for multi-tenant apps; this template does not persist projects yet — use `httpctx.GetActiveProjectID()` when you add project-scoped resources.

## Authentication

| Route group | Middleware |
|-------------|------------|
| `/api/*` (protected group) | `AuthenticateMiddleware` — validates JWT, loads/creates user |
| `/webhooks/clerk` | Svix signature verification |

## HTTP routes

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/users/me` | Current user profile |
| `GET` | `/api/realtime/connection` | Centrifugo connection credentials |

## Code map

| Layer | Location |
|-------|----------|
| HTTP | `internal/interfaces/http/handler/user_handler.go` |
| Commands | `internal/application/command/user/` |
| Queries | `internal/application/query/user/` |
| Domain | `internal/domain/user/` |
| Clerk adapter | `internal/infrastructure/clerk/` |
| Webhook | [Webhooks](webhooks.md) |

## Events

- `user.created.v1`, `user.updated.v1`, `user.deleted.v1` (Clerk webhooks → commands → outbox)

## Tests

`internal/interfaces/http/handler/test/user/`
