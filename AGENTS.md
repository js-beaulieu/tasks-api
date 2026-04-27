# tasks-api

Backend API for a task management application. Exposes a REST API and an MCP (Model Context Protocol) interface, backed by SQLite.

## Commands

```bash
task install                 # install Go tool deps and the pre-commit hook
task build                   # build all packages
task test                    # all tests (unit + integration)
task test:unit               # unit tests only (no DB)
task test:integration        # integration tests only (SQLite, real DB)
task format                  # gofmt -w .
task lint                    # golangci-lint via go tool (pinned in go.mod)
task check                   # format + lint + build + test:coverage
```

`lefthook` runs `task check` on `pre-commit`.

## Architecture

```
main.go
  └─ sqlite.Open("tasks.db") → sqlite.Store{Users, Projects, Tasks, Tags}
  └─ chi router
       ├─ httpserver.New(store)      → REST on /
       └─ mcpserver.Handler(store)   → MCP StreamableHTTP on /mcp
```

All layers talk through `internal/repo` interfaces — handlers never import `sqlite` directly (except `httpserver.New` / `mcpserver.Handler` which accept `*sqlite.Store` at the wiring point).

### Package map

| Package | Role |
|---------|------|
| `internal/model` | Domain structs: User, Project, ProjectMember, ProjectStatus, Task |
| `internal/repo` | Repository interfaces + sentinel errors (ErrNotFound, ErrNoAccess, ErrConflict) |
| `internal/store/sqlite` | Concrete SQLite implementations; goose migrations in `migrations/` |
| `internal/config` | Config struct loaded from env (PORT, LOG_FORMAT, LOG_LEVEL, LOG_DETAILED) |
| `internal/logger` | slog-based logger; `logger.FromCtx` / `logger.IntoCtx` context helpers |
| `internal/httpserver` | chi router wiring; sub-packages: middleware, projects, tasks, tags, users, render |
| `internal/mcpserver` | MCP server wiring + `withLogging` wrapper; sub-package: tools |
| `internal/testing` | Shared test helpers: `db` (in-memory SQLite), `mock` (repo mocks), `seed` (test fixtures) |

## Domain Model

```
User          id, name, email, created_at
Project       id, name, description*, due_date*, owner_id, assignee_id*, created_at, updated_at
ProjectMember project_id, user_id, role (read|modify|admin)
ProjectStatus project_id, status, position
Task          id, project_id, parent_id*, name, description*, status, due_date*, owner_id,
              assignee_id*, position, recurrence* (RFC 5545 RRULE), created_at, updated_at
Tags          stored as strings in task_tags(task_id, tag)
```

Default project statuses seeded on `CreateProject`: `todo`, `in_progress`, `done`, `cancelled`.
Task status is validated at the app layer against `project_statuses` (not a FK).

## Auth & Access Control

Auth: `X-User-ID` header (required on all routes except `POST /login`). `AuthMiddleware` looks up the user by ID and injects `*model.User` into context. `middleware.UserFromCtx(ctx)` panics if called without middleware — intentional.

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

## Recurring tasks

`POST /tasks/{id}/complete` (HTTP) / `complete_task` (MCP) is the only trigger for recurrence — plain status updates do not fire it. If the task has `recurrence` (RFC 5545 RRULE) **and** `due_date`, it marks the task done and creates the next occurrence with `due_date = nextOccurrence(due, rrule)` and `status = first project status (lowest position)`. Tags are copied. Returns `{completed, next}` (`next` is null for non-recurring tasks).

## MCP

All MCP tools accept `user_id` for access control. Tools are registered via a `withLogging` wrapper in `mcpserver/server.go` (adds `invocation_id`, logs entry/exit/duration, respects `LOG_DETAILED`). See `internal/mcpserver/tools/` for the full tool list.

## Key Patterns

**Context loading middleware:** both `projectCtx` and `taskCtx` middlewares load the entity + caller's role into the request context before the handler runs. Handlers call `projectFromCtx` / `taskFromCtx` and `roleFromCtx` / `taskRoleFromCtx`.

**`nullable[T]` type** (`internal/httpserver/tasks/handler.go`): distinguishes JSON field absent (`Set=false`) from present-but-null (`Set=true, Value=nil`). Used for `parent_id` in PATCH to allow explicit null (detach from parent).

**Store transactions:** all multi-step writes (Create task with position, Update task with sibling reordering, CompleteTask with next-occurrence creation) run in a single `sql.Tx`.

**Cycle guard:** `Update` uses a recursive CTE to detect if reparenting a task would create a cycle.

**Position management:** tasks within a sibling group are ordered by `position` (integer). Create appends (`MAX(position)+1`). Move compacts the old group and shifts the new group. Same-parent reorders shift only the affected range.

## Testing

- **Unit tests** (`internal/httpserver/...`): mock repos via `internal/testing/mock/`, `httptest.NewRecorder`, no DB
- **Integration tests** (`internal/store/sqlite/...`): build tag `//go:build integration`, real in-memory SQLite (`file::memory:?cache=shared&_pragma=foreign_keys(ON)`), migrations run via `db.Open`
- **Isolation pattern** when adding new store tests: run new tests with `-run TestNewFeature` separately from existing tests to avoid nil-pointer panics hiding regressions

## Structure

```
main.go
internal/
  config/           env-based config
  logger/           slog context helpers
  model/            domain types
  repo/             interfaces + sentinel errors
  store/sqlite/     concrete implementations + goose migrations
  httpserver/
    middleware/     auth (X-User-ID), logging (request ID, body)
    projects/       handler + access.go (RequireRole)
    tasks/          handler (includes nullable[T])
    tags/           handler
    users/          handler
    render/         JSON/error helpers
  mcpserver/
    tools/          one file per entity (projects, tasks, tags)
  testing/
    db/             in-memory SQLite helper
    mock/           repo mock implementations
    seed/           test fixture builders
```

## Branching

Trunk-based: `main` is protected — no direct pushes. All changes go through short-lived feature branches and PRs (delete branch after merge). No long-lived branches.

## Commits

Follow Conventional Commits: `type(scope): message`

Types: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`

Examples:
- `feat(httpserver): add POST /tasks endpoint`
- `fix(mcpserver): return proper error on missing param`
- `chore: update dependencies`
