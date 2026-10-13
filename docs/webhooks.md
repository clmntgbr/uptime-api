# Webhooks

## Overview

External providers push lifecycle events to the API. Signatures are verified in middleware before handlers run.

| Provider | Path | Middleware |
|----------|------|------------|
| Clerk | `POST /webhooks/clerk` | Svix (`UserWebhookMiddleware`) |

No Clerk JWT on webhook routes.

## Clerk (user lifecycle)

Handler: `UserWebhookHandler.Execute`

Verified payload is set on `c.Locals("payload", dto.ClerkEvent)` before the handler runs.

| Event | Action |
|-------|--------|
| `user.created` | Create user if not exists (`201`) |
| `user.updated` | Update profile (`204`) |
| `user.deleted` | Delete by Clerk ID (`204`) |
| Unknown | `200` (ignored) |

### Idempotence

`user.created` skips creation if the Clerk ID already exists.

### Errors

| Case | HTTP |
|------|------|
| Invalid signature / payload (middleware) | `401` + `{ "message": "…" }` |
| Invalid JSON body | `400` |
| Validation failed | `400` + field `errors` |
| User not found on update | `404` |
| Handler failure | `500` (generic `message`, no internal leak) |

## Code map

| Layer | Location |
|-------|----------|
| HTTP | `user_webhook_handler.go` |
| Middleware | `internal/interfaces/http/middleware/user_webhook_middleware.go` |
| Commands | `internal/application/command/user/` |

## Tests

`internal/interfaces/http/handler/test/user_webhook/`

## Security

- Never trust raw webhook bodies — middleware validates signatures first.
- Handlers validate unmarshalled DTOs via `validation.Struct`.
