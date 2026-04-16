# FailSafe

FailSafe is an integration and chaos-testing harness for validating resilience of backend, frontend, and Android targets. It provides APIs and orchestration for running controlled fault-injection experiments, collecting metrics, and producing post-run summaries.

**Key capabilities**
- Start/stop experiments for backend, frontend, and Android targets
- Upload and manage APKs for Android experiments
- Ingest frontend performance metrics and aggregate results
- Orchestrate Docker- and emulator-based test targets
- Provide reproducible run artifacts and metrics for analysis

## Quick start (Docker)

Prerequisites: Docker (and Docker Compose), Go (for local dev), Node/npm for frontend tooling.

1) Start core services (uses `deployments/docker/docker-compose.yml`):

```powershell
docker compose -f deployments/docker/docker-compose.yml up -d --build
```

2) Run the controller (local development):

```powershell
go run cmd/controller/main.go
```

3) Open API docs locally (optional):

```powershell
.
# or use the provided script to build and serve
powershell -ExecutionPolicy Bypass -File .\scripts\serve-docs.ps1 -Port 8090
```

## Local Postgres (quick)

If you prefer to run Postgres in a single container for local dev:

```powershell
docker run -d --name failsafe-postgres -e POSTGRES_USER=failsafe -e POSTGRES_PASSWORD=failsafe -e POSTGRES_DB=failsafe -p 5432:5432 postgres:15
docker exec -it failsafe-postgres psql -U failsafe -d failsafe
```

## Repository layout (high level)

- `cmd/` — entrypoint programs (controller, test servers)
- `internal/` — core application logic and private packages
- `internal/storage/` — database migrations and schema
- `orchestrator/` — experiment lifecycle orchestration
- `handlers/` — HTTP handlers and API surface
- `monitoring/` — metric collection and exporters
- `docs/` — human-readable docs and generated site
- `deployments/docker/` — Dockerfiles and compose manifests
- `data/metrics/` — CSV/JSON metric exports and examples
- `data/payloads/` — sample request payloads and fixtures
- `uploads/apks/` — APK uploads (large binaries kept here)

Files moved during reorganization: many top-level docs, metrics, and payload files were consolidated under `docs/`, `data/metrics`, and `data/payloads` to keep the root cleaner.

## API overview

The service exposes a simple JSON HTTP API for experiment control. See the full API reference in `docs/API_DOCS.md` and the concise endpoint list in `docs/EXACT_API_ENDPOINTS.md`.

Common endpoints:

- `POST /experiments/<platform>/start` — start an experiment (`platform` = `backend|frontend|android`)
- `GET /experiments/<platform>/status?id={id}` — fetch live status
- `POST /experiments/<platform>/stop?id={id}` — stop a running experiment
- `POST /frontend/metrics` — ingest frontend metrics
- `POST /upload/apk` — upload an APK and return metadata

## Tests & integration

- Integration tests and test harnesses live under `test/` and `internal/`.
- Playwright-based frontend integration lives under `test/playwright` (see `package.json` scripts).
- Use `scripts/start-mock-services.ps1` to run the Python mock services if you need a full stack without Docker.

## Scripts and helpers

- `scripts/build-docs.ps1` — generate HTML site from `docs/` sources
- `scripts/serve-docs.ps1` — local doc server
- `scripts/docker-helper.ps1` — helper for starting/stopping Docker in Windows dev

## Contributing

If you want to contribute:

1. Create an issue describing the problem or feature.
2. Open a branch named `feature/` or `fix/` and submit a PR with tests or a clear reproduction.

## Troubleshooting

- If Android experiments fail, ensure `adb` and emulator tooling are available on PATH. See `docs/ANDROID_EXECUTOR_QUICKSTART.md`.
- If Postgres migrations fail, confirm DB connection string in env and run `internal/storage/schema.sql` against the DB.

## Where things moved

- Metrics and exports: `data/metrics/`
- Payload fixtures: `data/payloads/`
- Dockerfiles / compose: `deployments/docker/`
- Docs and site sources: `docs/`

---

If you'd like, I can also update any scripts that reference files moved during reorganization, or commit these changes for you. Which would you prefer next?