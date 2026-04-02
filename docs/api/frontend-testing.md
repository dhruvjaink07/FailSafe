# Frontend Web Testing Integration Guide

## Goal

Test resilience and performance of target web pages (external site or app URL) under controlled frontend faults.

This flow is for web page testing only. It does not require APK upload, Android emulator fields, or dashboard-specific steps.

## End-to-End Flow

1. Start a frontend experiment with web runtime config.
2. Run the Playwright collector runner for the same experiment id.
3. Poll frontend status until terminal state.
4. Fetch frontend metrics report and render results.

## Step 1: Start Frontend Experiment

Endpoint:

```http
POST /experiments/frontend/start
```

Minimum payload:

```json
{
  "fault_type": "latency",
  "target_type": "frontend",
  "duration_seconds": 30,
  "frontend_run": {
    "base_url": "https://example.com",
    "metrics_endpoint": "http://localhost:8000/frontend/metrics",
    "target_urls": ["/api/"]
  }
}
```

Required web fields:

- `target_type`: `frontend`
- `frontend_run.base_url`: target site URL for browser run

## Step 2: Run Browser Collector

From repo root:

```powershell
$env:EXPERIMENT_ID="<experiment-id-from-start-response>"
node internal/frontend/automation/playwright/runner.js
```

Legacy compatibility command:

```powershell
node playwright/runner.js
```

Optional overrides:

- `BASE_URL`
- `FAILSAFE_FRONTEND_ENDPOINT`
- `FAILSAFE_CONTROLLER_URL`

If `BASE_URL` and `FAILSAFE_FRONTEND_ENDPOINT` are not set, the runner resolves runtime config from:

```http
GET /experiments/frontend/status?id={experiment_id}
```

## Step 3: Poll Live Status

Endpoint:

```http
GET /experiments/frontend/status?id={experiment_id}
```

Recommended interval:

- 2 seconds

Stop polling when `state` is `completed` or `failed`.

## Step 4: Fetch Metrics Report

Endpoint:

```http
GET /experiments/frontend/metrics?id={experiment_id}
```

Use this payload for charts and summary cards.

## Collector Ingest Contract

Browser collector posts batches to:

```http
POST /frontend/metrics
```

Batch body shape:

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
        { "url": "/api/users", "duration": 140, "status": 200 }
      ],
      "timestamp": 1712000000000
    }
  ]
}
```

## UI Rendering Suggestions

- Run header: experiment id, state, phase.
- Vitals: LCP, CLS, INP by phase.
- Stability: long tasks, JS errors, unhandled rejections.
- API quality: success/error rates and latency percentiles.
- Recovery: baseline vs injecting vs recovery deltas.

## Notes

- Keep frontend testing scoped to target page behavior under faults.
- Android and APK workflows are separate and should not be mixed into this flow.
