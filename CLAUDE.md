# tasks

Backend API for a task management application. Exposes a REST API and an MCP (Model Context Protocol) interface, backed by a SQL database (Postgres/SQLite).

## Commands

```bash
go build ./...          # build
go test ./...           # test
go vet ./...            # vet
gofmt -w .              # format
golangci-lint run       # lint (requires golangci-lint installed)
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
