# tasks-api

Backend API for a task management application. Exposes a REST API and an [MCP](https://modelcontextprotocol.io) (Model Context Protocol) interface, backed by Postgres.

## Documentation

- [Documentation index](docs/README.md)
- [Architecture overview](docs/architecture/overview.md)
- [Local development](docs/development/local.md)

## Getting Started

```bash
task install
docker compose up -d db
go run .
```

`task install` installs the pinned Go tools and the `lefthook` pre-commit hook.

Auth for protected routes uses `X-User-ID`.

Generated REST API docs are exposed at `/docs`, with OpenAPI available at `/openapi.json` and `/openapi.yaml`.
