# FailSafe API Documentation

This file is the entrypoint for API docs.

## HTML Docs Site

- Site overview: `docs/site/index.html`
- Backend API contract: `docs/site/backend-api.html`
- Frontend integration guide: `docs/site/frontend-testing.html`
- Postman testing guide: `docs/site/postman-testing.html`

## Documentation Pages

- API overview and endpoint map: `docs/api/README.md`
- Backend API contract: `docs/api/backend-api.md`
- Frontend integration and polling flow: `docs/api/frontend-testing.md`
- Postman testing guide and request examples: `docs/api/postman-testing.md`

## Quick Start

1. Start infra and backend:

```bash
docker compose up -d postgres
go run cmd/controller/main.go
```

2. Open API docs:

```text
docs/site/index.html
```

3. Markdown source docs are kept under:

```text
docs/api/
```

## Build And Serve Pipeline

Quick launch commands:

```powershell
go run cmd/controller/main.go
powershell -ExecutionPolicy Bypass -File .\scripts\serve-docs.ps1 -Port 8090
```

Generate HTML pages from API markdown sources:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-docs.ps1
```

Serve locally with hosted-style relative link behavior:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\serve-docs.ps1 -Port 8090
```

The build script verifies API-only output and fails if architecture-oriented content appears in generated site pages.
