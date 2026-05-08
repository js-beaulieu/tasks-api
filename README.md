# tasks-api

Backend API for a task management application. Exposes a REST API and an [MCP](https://modelcontextprotocol.io) (Model Context Protocol) interface, backed by Postgres.

## Getting started

```bash
task install
docker compose up -d db
go run .
```

`task install` installs the pinned Go tools and the `lefthook` pre-commit hook.

Auth via `X-User-ID` header. See [AGENTS.md](AGENTS.md) for full architecture, domain model, access control, and dev commands.

## Local Postgres

```bash
docker compose up -d db
```

The database is exposed on `localhost:5432` and persisted in the `postgres_data` volume.

## Tests

Integration tests use [`testcontainers-go`](https://golang.testcontainers.org/modules/postgres/) to start a real Postgres instance automatically. Docker must be available when running `task test`, `task test:integration`, or coverage tasks that include integration tests.
