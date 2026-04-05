# FailSafe API Docs

This is the compact, consolidated API reference for the current platform-separated routes.

## Base URL

```text
http://localhost:8000
```

## Content Types

- JSON: `Content-Type: application/json`
- APK upload: `multipart/form-data`

## Route Map

| Method | Path | Auth | Request Body | Notes |
| --- | --- | --- | --- | --- |
| GET | `/health` | Public | none | Liveness check. |
| GET | `/scenarios/presets` | Public | none | Lists scenario presets and supported fault/trigger types. |
| POST | `/upload/apk` | Public | multipart form data (`file` or `apk`) | Upload APK and resolve package/activity metadata. |
| POST | `/frontend/metrics` | Public | JSON `FrontendMetricsBatch` | Browser collector ingestion endpoint. |
| POST | `/internal/api-keys/create` | Public | JSON `CreateAPIKeyRequest` | Key provisioning endpoint. |
| GET | `/environment/containers` | API key (`viewer+`) | none | Lists all containers in the docker environment (`docker ps -a`). |
| POST | `/environment/containers/start` | API key (`engineer+`) | query `name` or JSON `{"name":"..."}` | Starts/wakes selected container if present. |
| POST | `/experiments/backend/start` | API key (`engineer+`) | JSON `StartRequest` | Backend/Docker run; requires `target_type=docker`. |
| GET | `/experiments/backend/status?id={id}` | Public | query `id` | Returns backend lifecycle payload. |
| POST | `/experiments/backend/stop?id={id}` | API key (`engineer+`) | query `id` | Stops backend run. |
| GET | `/experiments/backend/metrics?id={id}` | API key (`viewer+`) | query `id` | Returns backend metrics summary. |
| POST | `/experiments/android/start` | API key (`engineer+`) | JSON `StartRequest` | Android run; requires `target_type=android`. |
| GET | `/experiments/android/status?id={id}` | Public | query `id` | Returns Android live status payload. |
| POST | `/experiments/android/stop?id={id}` | API key (`engineer+`) | query `id` | Stops Android run. |
| GET | `/experiments/android/metrics?id={id}` | API key (`viewer+`) | query `id` | Returns Android metrics summary. |
| POST | `/experiments/frontend/start` | API key (`engineer+`) | JSON `StartRequest` | Frontend run; requires `target_type=frontend` and `frontend_run.base_url`. |
| GET | `/experiments/frontend/status?id={id}` | Public | query `id` | Returns frontend lifecycle payload. |
| POST | `/experiments/frontend/stop?id={id}` | API key (`engineer+`) | query `id` | Stops frontend run. |
| GET | `/experiments/frontend/metrics?id={id}` | API key (`viewer+`) | query `id` | Returns frontend metrics and score payload. |
| GET | `/experiments/frontend/fault-command?id={id}` | Public | query `id` | Returns current active frontend fault command (if any). |

## Shared Experiment Start Body

All three `/experiments/<platform>/start` routes accept the same top-level JSON shape.
The handler accepts snake_case and camelCase aliases for most fields.

Required fields:

- `fault_type` or `faultType`
- `target_type` or `targetType`

`targets` is required for backend and Android starts. Frontend start can run without `targets`.

Common optional fields:

- `observed_endpoints` or `observedEndpoints`
- `observation_type` or `observationType`
- `duration_seconds` or `duration`
- `adaptive`
- `step_intensity` or `stepIntensity`
- `max_intensity` or `maxIntensity`
- `dependency_graph` or `dependencyGraph`
- `target_endpoint_map` or `targetEndpointMap`
- `scenario` or `scenarios`
- `scenario_preset` or `scenarioPreset`
- `expected`
- `android_run` or `androidRun`
- `apk`, `apk_id`, `uploadedApkId`, or `uploaded_apk_id`
- `frontend_run` or `frontendRun`

Protected routes require `x-api-key`:

- `/environment/containers`
- `/environment/containers/start`
- `/experiments/backend/start`, `/experiments/backend/stop`
- `/experiments/android/start`, `/experiments/android/stop`
- `/experiments/frontend/start`, `/experiments/frontend/stop`
- all `/experiments/*/metrics` routes

Public routes (no API key required):

- `/health`
- `/scenarios/presets`
- `/upload/apk`
- `/frontend/metrics`
- `/internal/api-keys/create`
- all `/experiments/*/status` routes
- `/experiments/frontend/fault-command`

Valid roles:

- `viewer` can read metrics only
- `engineer` can start and stop experiments
- `admin` can start, stop, and provision keys

Valid environments:

- `dev`
- `prod`

## Environment Container APIs

### List containers

`GET /environment/containers`

Headers:

- `x-api-key: <viewer|engineer|admin key>`

Response shape:

```json
{
  "timestamp": "2026-04-05T12:34:56Z",
  "count": 2,
  "containers": [
    {
      "id": "5f7c...",
      "name": "failsafe-postgres",
      "image": "postgres:15",
      "state": "running",
      "status": "Up 4 minutes",
      "ports": "0.0.0.0:5432->5432/tcp",
      "running": true
    }
  ]
}
```

### Start/wake a container

`POST /environment/containers/start`

Headers:

- `x-api-key: <engineer|admin key>`

Supported request forms:

1) query parameter

```text
POST /environment/containers/start?name=failsafe-postgres
```

2) JSON body

```json
{
  "name": "failsafe-postgres"
}
```

Response shape:

```json
{
  "name": "failsafe-postgres",
  "status": "started_or_already_running",
  "started_at": "2026-04-05T12:35:22Z"
}
```

## Platform-Specific Start Requirements

### Backend

```json
{
  "fault_type": "network_delay",
  "targets": ["order-service"],
  "target_type": "docker",
  "observation_type": "http",
  "observed_endpoints": [
    "http://api-gateway:8080/api/users/1",
    "http://api-gateway:8080/api/orders/10",
    "http://user-service:8081/users/1",
    "http://order-service:8082/orders/10",
    "http://payment-service:8083/payments/10",
    "http://inventory-service:8084/inventory/A1",
    "http://shipping-service:8085/shipping/10",
    "http://notification-service:8086/notifications/10",
    "http://recommendation-service:8087/recommendations/1"
  ],
  "duration_seconds": 60,
  "scenarios": [
    { "type": "network_delay", "at": 5, "duration_seconds": 20 },
    { "type": "packet_loss", "at": 30, "duration_seconds": 15 }
  ],
  "dependency_graph": {
    "http://api-gateway:8080/api/users/1": ["http://user-service:8081/users/1"],
    "http://api-gateway:8080/api/orders/10": ["http://order-service:8082/orders/10"],
    "http://user-service:8081/users/1": ["http://recommendation-service:8087/recommendations/1"],
    "http://order-service:8082/orders/10": [
      "http://payment-service:8083/payments/10",
      "http://inventory-service:8084/inventory/A1",
      "http://shipping-service:8085/shipping/10"
    ],
    "http://shipping-service:8085/shipping/10": ["http://notification-service:8086/notifications/10"]
  },
  "target_endpoint_map": {
    "api-gateway": [
      "http://api-gateway:8080/api/users/1",
      "http://api-gateway:8080/api/orders/10"
    ],
    "user-service": ["http://user-service:8081/users/1"],
    "order-service": ["http://order-service:8082/orders/10"],
    "payment-service": ["http://payment-service:8083/payments/10"],
    "inventory-service": ["http://inventory-service:8084/inventory/A1"],
    "shipping-service": ["http://shipping-service:8085/shipping/10"],
    "notification-service": ["http://notification-service:8086/notifications/10"],
    "recommendation-service": ["http://recommendation-service:8087/recommendations/1"]
  }
}
```

### Android

```json
{
  "fault_type": "kill_app",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 70,
  "apk": "{{apkId}}",
  "android_run": {
    "avd_name": "Pixel_8a",
    "headless": true,
    "reset_app_state": true
  },
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

Android start notes:

- Use `android_run.headless`; `ui_mode` is not part of the current API and is ignored.
- `observed_endpoints` may be `null` on the start response for Android runs; that is expected.
- `scenario` is accepted as a single list field and is reflected back in the experiment payload.
- Android tooling preflight is performed at server startup and cached. If unavailable, Android start fails fast with `400` and a message beginning with `android preflight failed` instead of creating a partially initialized run.

Android status fields to verify:

- `state` should move through `running` to `completed` or `failed`.
- `health.status` reports the live runtime view (`healthy`, `degraded`, or `down`).
- `faults.applied` should match the number of executed scenario steps.
- `faults.events[].in_phase` is captured at injection time (for example, `injecting`) and is no longer overwritten by the final phase.
- `timeline.first_impact` and `timeline.recovery` are backfilled from observed samples when the live maps are empty.
- `timeline_status` provides explicit booleans for `impact_observed`, `recovery_observed`, and `impact_pending`.
- `server_time` provides backend-side clock time for polling consistency.
- `is_terminal` indicates whether the experiment has reached a terminal lifecycle state.
- `next_fault_eta_ms` and `next_fault` show the upcoming scheduled fault step, or `-1`/`null` when no pending step remains.
- `progress.completed_percent_of_plan` is forced to `100` after terminal completion/failure.

Android metrics fields to verify:

- `summary.result` should be `PASS` when the configured expectations are satisfied.
- `validation.passed` should be `true` when the expectations are configured and met.
- `recovery.recovered` and `recovery.recovery_time_ms` should now be aligned for normal Android runs; if recovery is detected, the timestamp is backfilled from the sampled state transitions.
- Planned `foreground_app` scenario steps are treated as orchestrated recovery actions (not external/manual intervention), so `recovery.auto_recovered` remains `true` for successful planned recoveries.
- `replay_hints` and `state_transitions` should be populated once the app records transitions; empty arrays mean the run did not produce a visible state change.
- `blast_radius_percent` is derived from Android availability impact (`100 - uptime_percent`) with severity floors for kill/crash/anr events.
- `cascade_depth` is Android disruption depth: `0` none, `1` degraded-only, `2` hard failure (`not_running`/`crash`/`anr`), `3` includes restart transitions.

### Frontend

```json
{
  "fault_type": "latency",
  "targets": ["frontend-app"],
  "target_type": "frontend",
  "duration_seconds": 20,
  "frontend_run": {
    "base_url": "http://127.0.0.1:3001",
    "metrics_endpoint": "http://localhost:8000/frontend/metrics",
    "target_urls": ["/api/fast", "/api/slow"]
  }
}
```

### API Key Creation

`POST /internal/api-keys/create`

Headers:

- `Content-Type: application/json`

Authorization rules:

- no auth restriction on creation right now (any caller can create keys)

Request body:

```json
{
  "environment": "dev",
  "role": "engineer",
  "name": "team-a"
}
```

Response:

```json
{
  "api_key": "fs_..."
}
```

## Frontend Metrics Ingestion Body

`POST /frontend/metrics` accepts:

```json
{
  "metrics": [
    {
      "experiment_id": "exp-123",
      "phase": "baseline",
      "page": "/",
      "metrics": {
        "lcp": 1200,
        "cls": 0.04,
        "inp": 90,
        "long_tasks": 1,
        "errors": 0,
        "unhandled_rejections": 0
      },
      "api_calls": [
        { "url": "/api/users", "duration": 120, "status": 200 }
      ],
      "timestamp": 1712000000000
    }
  ]
}
```

## Response Notes

- Start endpoints return the created experiment object with `id`, `state`, `phase`, and platform fields.
- Status endpoints return the live experiment payload for the matching platform.
- Metrics endpoints return platform-specific summaries and do not rely on the old shared `/experiment/*` routes.
- Stop endpoints return `experiment stopped`.
- Android status and metrics backfill impact/recovery from observed samples when the live maps are empty; check the `state`, `health`, `validation`, and `summary` fields together instead of relying on one block alone.

## Detailed Docs

- Validation workflow: `docs/testing/README.md`
- API index: `docs/api/README.md`
- Backend contract details: `docs/api/backend-api.md`
- Frontend contract details: `docs/api/frontend-testing.md`
- Postman guide: `docs/api/postman-testing.md`

## Copy-Paste Runbook (Windows PowerShell)

### 1) Start dependencies

Start mock microservices:

```powershell
docker compose -f docker-compose.test.yml up -d --build
```

Start controller prerequisites and controller:

```powershell
docker compose up -d postgres
go run cmd/controller/main.go
```

### 2) Backend test run (mock microservices)

Start backend experiment:

```powershell
$backendStartBody = @'
{
  "fault_type": "network_delay",
  "targets": ["order-service"],
  "target_type": "docker",
  "observation_type": "http",
  "observed_endpoints": [
    "http://api-gateway:8080/api/users/1",
    "http://api-gateway:8080/api/orders/10",
    "http://user-service:8081/users/1",
    "http://order-service:8082/orders/10",
    "http://payment-service:8083/payments/10",
    "http://inventory-service:8084/inventory/A1",
    "http://shipping-service:8085/shipping/10",
    "http://notification-service:8086/notifications/10",
    "http://recommendation-service:8087/recommendations/1"
  ],
  "duration_seconds": 60,
  "scenarios": [
    { "type": "network_delay", "at": 5, "duration_seconds": 20 },
    { "type": "packet_loss", "at": 30, "duration_seconds": 15 }
  ]
}
'@

$backendExp = Invoke-RestMethod -Method Post -Uri "http://localhost:8000/experiments/backend/start" -ContentType "application/json" -Body $backendStartBody
$backendExp.id
```

Get status, metrics, stop:

```powershell
$id = $backendExp.id
Invoke-RestMethod "http://localhost:8000/experiments/backend/status?id=$id"
Invoke-RestMethod "http://localhost:8000/experiments/backend/metrics?id=$id"
Invoke-RestMethod -Method Post "http://localhost:8000/experiments/backend/stop?id=$id"
```

### 3) Android test run

Preflight note:

- If `adb` or emulator tooling is missing at server startup, `/experiments/android/start` returns `400` with `android preflight failed`.
- In that case, skip Android for that environment and continue backend/frontend API validation.

Upload APK first:

```powershell
$apk = Invoke-RestMethod -Method Post -Uri "http://localhost:8000/upload/apk" -Form @{ file = Get-Item "D:\FailSafe\uploads\apks\app.apk" }
$apk.apk
```

Start Android experiment:

```powershell
$androidStartBody = @"
{
  "fault_type": "kill_app",
  "targets": ["$($apk.package)"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 70,
  "apk": "$($apk.apk)",
  "android_run": {
    "avd_name": "Pixel_8a",
    "headless": true,
    "reset_app_state": true
  },
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
"@

$androidExp = Invoke-RestMethod -Method Post -Uri "http://localhost:8000/experiments/android/start" -ContentType "application/json" -Body $androidStartBody
$androidExp.id
```

Get status, metrics, stop:

```powershell
$id = $androidExp.id
Invoke-RestMethod "http://localhost:8000/experiments/android/status?id=$id"
Invoke-RestMethod "http://localhost:8000/experiments/android/metrics?id=$id"
Invoke-RestMethod -Method Post "http://localhost:8000/experiments/android/stop?id=$id"
```

### 4) Frontend + Playwright run

Install Playwright once:

```powershell
npm install
```

Use the local test server as the frontend target:

- App URL: `http://127.0.0.1:3001`
- API endpoints under test: `/api/fast`, `/api/slow`
- Metrics ingestion: `http://localhost:8000/frontend/metrics`

Create frontend experiment:

```powershell
$frontendStartBody = @'
{
  "fault_type": "latency",
  "targets": ["frontend-app"],
  "target_type": "frontend",
  "duration_seconds": 30,
  "frontend_run": {
    "base_url": "http://127.0.0.1:3001",
    "metrics_endpoint": "http://localhost:8000/frontend/metrics",
    "target_urls": ["/api/fast", "/api/slow"]
  }
}
'@

$frontendExp = Invoke-RestMethod -Method Post -Uri "http://localhost:8000/experiments/frontend/start" -ContentType "application/json" -Body $frontendStartBody
$frontendExp.id
```

Run the Playwright integration harness:

```powershell
$env:EXPERIMENT_ID = $frontendExp.id
$env:BASE_URL = "http://127.0.0.1:3001"
$env:FAILSAFE_FRONTEND_ENDPOINT = "http://localhost:8000/frontend/metrics"
$env:FAILSAFE_CONTROLLER_URL = "http://localhost:8000"
node internal/frontend/automation/playwright/runner.js
```

Fetch frontend status and metrics:

```powershell
$id = $frontendExp.id
Invoke-RestMethod "http://localhost:8000/experiments/frontend/status?id=$id"
Invoke-RestMethod "http://localhost:8000/experiments/frontend/metrics?id=$id"
Invoke-RestMethod -Method Post "http://localhost:8000/experiments/frontend/stop?id=$id"
```