# tasks-api

Backend API for a task management application. Exposes a REST API and an [MCP](https://modelcontextprotocol.io) (Model Context Protocol) interface, backed by SQLite.

## Getting started

```bash
go run .
```

Auth via `X-User-ID` header. See [CLAUDE.md](CLAUDE.md) for full architecture, domain model, access control, and dev commands.

## Docker

```bash
docker build -t tasks-api .
docker run -p 8080:8080 tasks-api
```




