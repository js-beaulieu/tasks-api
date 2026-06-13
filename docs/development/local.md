# Local Development

## Getting Started

```bash
task install
docker compose up -d db
go run .
```

`task install` downloads pinned Go tool dependencies and installs the `lefthook` pre-commit hook.

## Local Postgres

```bash
docker compose up -d db
```

The local database listens on `localhost:5432` and persists data in the `postgres_data` volume.

## Useful Commands

```bash
task build
task format
task lint
task test
task test:unit
task test:integration
task check
```

`lefthook` runs `task check` on pre-commit.

## Generated API Docs

With the server running locally, the REST API exposes:

- `/docs`
- `/openapi.json`
- `/openapi.yaml`
- `/health`

Auth for protected routes uses `X-User-ID`.

## Testing Notes

Integration tests use [`testcontainers-go`](https://golang.testcontainers.org/modules/postgres/) to start a real Postgres instance automatically. Docker must be available when running integration or coverage tasks.

When adding new store integration tests, run focused cases with `-run TestName` when needed so unrelated failures do not mask the feature under development.
