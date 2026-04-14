# tasks

Backend API for a task management application. Exposes a REST API and an MCP (Model Context Protocol) interface, backed by a SQL database (Postgres/SQLite).

## Commands

```bash
task build              # build all packages
task test               # all tests (unit + integration)
task test:unit          # unit tests only (no DB)
task test:integration   # integration tests only (SQLite, real DB)
task fmt                # gofmt -w .
task lint               # golangci-lint via go tool (pinned in go.mod)
task check              # fmt + lint + build + test:unit (pre-commit)
```

## Structure

- `main.go` — entry point, wires HTTP + MCP routers on `:8080`
- `internal/httpserver/` — chi-based REST handlers
- `internal/mcpserver/` — MCP tools via go-sdk
- `.github/workflows/` — CI pipelines (GitHub Actions)
- `.golangci.yml` — linter config

## Conventions

- All business logic lives in `internal/`
- New HTTP routes go in `httpserver`, new MCP tools in `mcpserver`
- Table-driven tests using `t.Run`
- SQL queries go in a dedicated `internal/store/` package

## Branching

Trunk-based: commit to `main` directly or via short-lived feature branches (delete after merge). No long-lived branches.

## Commits

Follow Conventional Commits: `type(scope): message`

Types: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`

Examples:
- `feat(httpserver): add POST /tasks endpoint`
- `fix(mcpserver): return proper error on missing param`
- `chore: update dependencies`
