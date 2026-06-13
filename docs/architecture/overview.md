# Architecture Overview

## Runtime Wiring

```
main.go
  └─ postgres.Open(PG_CONNECTION_STRING) → postgres.Store{Users, Projects, Tasks, Tags}
  └─ http.ServeMux
       ├─ httpserver.New(store)      → Huma REST API on /
       │    ├─ generated OpenAPI     → /openapi.json, /openapi.yaml
       │    └─ generated docs        → /docs
       └─ mcpserver.Handler(store)   → MCP StreamableHTTP on /mcp
```

The top-level mux applies request logging around both the REST API and the MCP endpoint. REST routes are registered as typed Huma operations. MCP uses its own handler and is protected by the same `X-User-ID` auth middleware.

## Package Map

| Package | Role |
|---------|------|
| `internal/model` | Domain structs: User, Project, ProjectMember, ProjectStatus, Task |
| `internal/repo` | Repository interfaces + sentinel errors (`ErrNotFound`, `ErrNoAccess`, `ErrConflict`) |
| `internal/store/postgres` | Concrete Postgres implementations; goose migrations in `migrations/` |
| `internal/config` | Config struct loaded from env (`PORT`, `PG_CONNECTION_STRING`, `LOG_FORMAT`, `LOG_LEVEL`, `LOG_DETAILED`) |
| `internal/logger` | slog-based logger; `logger.FromCtx` / `logger.IntoCtx` context helpers |
| `internal/httpserver` | Huma REST API wiring; sub-packages: `middleware`, `projects`, `tasks`, `tags`, `users`, `render` |
| `internal/mcpserver` | MCP server wiring + `withLogging` wrapper; sub-package: `tools` |
| `internal/testing` | Shared test helpers: `db`, `http`, `mcp`, `mock`, `seed` |

All layers talk through `internal/repo` interfaces. Handlers never import `postgres` directly except at the wiring points in `httpserver.New` and `mcpserver.Handler`.

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

Task status is validated at the app layer against `project_statuses` and is not a foreign key.

## Auth And Access Control

Auth uses the `X-User-ID` header. `AuthMiddleware` looks up the user by ID and injects `*model.User` into context. `middleware.UserFromCtx(ctx)` intentionally panics if called without the middleware having run first.

Roles rank as `read(1) < modify(2) < admin(3)`. `projects.RequireRole(min, actual)` implements the comparison.

| Action | Min role |
|--------|----------|
| Read project / list tasks | read |
| Create / update / delete task, manage tags | modify |
| Update project fields | modify |
| Move task to a different project | modify on both source and target |
| Manage members / custom statuses | admin |
| Delete project | admin |

`GetMemberRole` returns `admin` implicitly when `userID == project.OwnerID`.

## REST API Patterns

Each HTTP package exposes `RegisterRoutes(api, ...)` and registers typed Huma operations.

Optional request-body fields keep `json:",omitempty"` tags so Huma treats them as optional in generated schema and request validation. Pointer fields still carry PATCH semantics:

- `nil` means omitted
- non-`nil` means provided

`internal/httpserver/tasks/handler.go` defines `nullable[T]` for `parent_id` updates. That distinguishes field omitted from explicit JSON `null`, which allows detaching a task from its parent.

`internal/httpserver/render` still backs the plain `/health` handler and auth/logging middleware responses.

## Recurring Tasks

`POST /tasks/{id}/complete` (HTTP) and `complete_task` (MCP) are the only recurrence triggers. Plain task status updates do not create the next occurrence.

If a task has both `recurrence` (RFC 5545 RRULE) and `due_date`, completion:

- marks the current task done
- creates the next occurrence with `due_date = nextOccurrence(due, rrule)`
- sets the next task status to the first project status (lowest position)
- copies tags

The response shape is `{completed, next}` where `next` is `null` for non-recurring tasks.

## Store Behavior

Multi-step writes run in a single `sql.Tx`, including:

- create task with position assignment
- update task with sibling reordering
- complete recurring task with next-occurrence creation

Task reparenting uses a recursive CTE cycle guard. Task ordering is maintained by integer `position` within a sibling group.

## Testing

- Unit tests under `internal/httpserver/...` use repo mocks and `httptest` only.
- HTTP integration tests run against the full Huma server with Postgres via Testcontainers.
- Standalone Huma test mux setup lives in `internal/testing/http` so production handlers only expose `RegisterRoutes(...)`.
