# HTTP handler tests

Guide for tests under `internal/interfaces/http/handler/test/`.

Architecture rules: [`.cursor/rules/architecture.mdc`](../.cursor/rules/architecture.mdc).

## Principles

- **Unit tests at the HTTP boundary** — Fiber `app.Test(req)` with mocked command/query handlers.
- **No infrastructure** — no real Postgres, RabbitMQ, Clerk, Centrifugo, or Docker.
- **One test package per resource** — never beside production handler files.
- **Ports for mocking** — handlers depend on unexported interfaces in `*_ports.go`.
- **Target full coverage** — aim for 100% statement coverage on each `*_handler.go` when adding or changing an endpoint.

## When tests are required

| Change | Required test work |
|--------|-------------------|
| New `*_handler.go` | Create `test/<resource>/`: mocks, ports, scenarios for every public HTTP method |
| New HTTP method | Add `Test<Resource>Handler_<Method>_<Scenario>` (success + error paths) |
| Changed behaviour | Update tests; keep affected handler fully covered |

Tests are part of the definition of done — not a follow-up task.

## Directory layout

```
internal/interfaces/http/handler/
  <resource>_handler.go
  <resource>_ports.go
  <resource>_helpers.go        # optional
  <resource>_test_exports.go   # optional
  test/
    <resource>/                # package: <resource>test
      handler_test.go
      helpers_test.go
internal/interfaces/http/testutil/
  http.go
  fixtures.go
```

## Shared utilities

| Helper | Usage |
|--------|--------|
| `NewTestApp()` | Fiber + `validation.FiberErrorHandler` |
| `WithActiveProject(userID, projectID)` | User with active project |
| `WithUserWithoutProject(userID)` | Authenticated user, no project |
| `WithLocal(key, value)` | Webhook payload in `c.Locals` |
| `JSONRequest` | Build JSON requests |
| `DecodeJSON`, `DecodeJSONMap` | Parse responses |

## Scenarios (per HTTP method, when applicable)

1. **Success**
2. **Unauthorized** / **MissingActiveProject**
3. **Invalid input** (`400` + field errors)
4. **Handler error (business)** — not found, forbidden, …
5. **Handler error (internal)** → `500` without leaking `err.Error()`

## Coverage

```bash
go test ./internal/interfaces/http/handler/test/... -race \
  -coverpkg=./internal/interfaces/http/handler/... \
  -coverprofile=coverage.out
go tool cover -func=coverage.out
```

- CI floor: total handler statement coverage ≥ **95%**
- Codecov patch: **100%** on `internal/interfaces/http/handler/`

Local: `make tests`, `make coverage`, `make coverage-html`.
