# tasks-api

Backend API for a task management application. Exposes a REST API and an MCP (Model Context Protocol) interface, backed by Postgres.

Additional long-form project documentation lives under [`docs/`](docs/README.md).

## Commands

```bash
task install                 # install Go tool deps and the pre-commit hook
task build                   # build all packages
task test                    # all tests (unit + integration)
task test:unit               # unit tests only (no DB)
task test:integration        # integration tests only (Postgres via Testcontainers)
task format                  # gofmt -w .
task lint                    # golangci-lint via go tool (pinned in go.mod)
task check                   # format + lint + build + test:coverage
```

`lefthook` runs `task check` on `pre-commit`.

## Architecture

See [`docs/architecture/overview.md`](docs/architecture/overview.md) for the current runtime wiring, package map, auth model, Huma route patterns, and recurring-task behavior.

### Package map

| Package | Role |
|---------|------|
| `internal/model` | Domain structs: User, Project, ProjectMember, ProjectStatus, Task |
| `internal/repo` | Repository interfaces + sentinel errors (ErrNotFound, ErrNoAccess, ErrConflict) |
| `internal/store/postgres` | Concrete Postgres implementations; goose migrations in `migrations/` |
| `internal/config` | Config struct loaded from env (PORT, PG_CONNECTION_STRING, LOG_FORMAT, LOG_LEVEL, LOG_DETAILED) |
| `internal/logger` | slog-based logger; `logger.FromCtx` / `logger.IntoCtx` context helpers |
| `internal/httpserver` | Huma REST API wiring; sub-packages: middleware, projects, tasks, tags, users, render |
| `internal/mcpserver` | MCP server wiring + `withLogging` wrapper; sub-package: tools |
| `internal/testing` | Shared test helpers: `db` (Testcontainers Postgres), `http`, `mcp`, `mock`, `seed` |

## Domain Model

See [`docs/architecture/overview.md`](docs/architecture/overview.md).

## Auth & Access Control

Auth: `X-User-ID` header (required on all protected REST routes and on `/mcp`). `AuthMiddleware` looks up the user by ID and injects `*model.User` into context. `middleware.UserFromCtx(ctx)` panics if called without middleware — intentional.

Roles: `read(1) < modify(2) < admin(3)`. `RequireRole(min, actual string) bool` in `internal/httpserver/projects/access.go`.

| Action | Min role |
|--------|----------|
| Read project / list tasks | read |
| Create / update / delete task, manage tags | modify |
| Update project fields | modify |
| Move task to a different project | modify on **both** source and target |
| Manage members / custom statuses | admin |
| Delete project | admin |

`GetMemberRole` returns `"admin"` implicitly when `userID == project.OwnerID`.

## Recurring Tasks

See [`docs/architecture/overview.md`](docs/architecture/overview.md).

## MCP

All MCP tools accept `user_id` for access control. Tools are registered via a `withLogging` wrapper in `mcpserver/server.go` (adds `invocation_id`, logs entry/exit/duration, respects `LOG_DETAILED`). See `internal/mcpserver/tools/` for the full tool list.

## Key Patterns

- Huma HTTP packages expose `RegisterRoutes(api, ...)`.
- Optional Huma request-body fields keep `json:",omitempty"` so Huma treats them as optional.
- Pointer fields still carry PATCH semantics (`nil` means omitted).
- `tasks.nullable[T]` is used where explicit JSON `null` must differ from omission.
- Multi-step writes run in a single `sql.Tx`.
- Task reparenting uses a recursive CTE cycle guard.
- Task ordering is maintained by integer `position` within a sibling group.

## Testing

- **Unit tests** (`internal/httpserver/...`): mock repos via `internal/testing/mock/`, `httptest.NewRecorder`, no DB
- **Integration tests** (`internal/store/postgres/...`): build tag `//go:build integration`, real Postgres via `testcontainers-go`, migrations run via `db.Open`
- **Standalone Huma test mux**: use `internal/testing/http` so production handlers only expose `RegisterRoutes(...)`
- **Isolation pattern** when adding new store tests: run new tests with `-run TestNewFeature` separately from existing tests to avoid nil-pointer panics hiding regressions

## Branching

Trunk-based: `main` is protected — no direct pushes. All changes go through short-lived feature branches and PRs (delete branch after merge). No long-lived branches.

## Commits

Follow Conventional Commits: `type(scope): message`

Types: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`

Examples:
- `feat(httpserver): add POST /tasks endpoint`
- `fix(mcpserver): return proper error on missing param`
- `chore: update dependencies`
