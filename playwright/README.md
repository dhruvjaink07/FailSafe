# Playwright Live-Site Frontend Metrics Runner

This folder gives you a no-frontend-required runner.

## What it does

- Launches Chromium with Playwright.
- Injects `sender.js` and `collector.js` automatically.
- Visits the target URL.
- Applies network and CPU chaos.
- Sends frontend metrics to the Go controller.

## Requirements

- Go controller running (default expected endpoint: `http://localhost:8000/frontend/metrics`)
- Node.js installed
- Playwright installed in this repo

## Install

From repo root:

```powershell
npm init -y
npm install playwright
```

## Run

Default run (uses `https://example.com`):

```powershell
node playwright/runner.js
```

Run against a live site with explicit backend endpoint:

```powershell
$env:BASE_URL="https://jsonplaceholder.typicode.com"
$env:EXPERIMENT_ID="exp-live-1"
$env:FAILSAFE_FRONTEND_ENDPOINT="http://localhost:8000/frontend/metrics"
node playwright/runner.js
```

## Chaos targeting

By default the runner applies chaos to all requests:

```js
targetUrls: [""]
```

You can narrow this in `runner.js`, for example:

```js
targetUrls: ["/api/", "graphql"]
```

## Verify data arrived

```powershell
Invoke-RestMethod "http://localhost:8000/experiments/frontend/metrics?id=exp-live-1"
```

If your controller is on another port, adjust `FAILSAFE_FRONTEND_ENDPOINT` and the metrics URL.

## Configure from experiment/start

You can now set frontend runtime config in the experiment creation payload.

Example:

```json
{
	"faultType": "latency",
	"targets": ["api-gateway"],
	"targetType": "docker",
	"observationType": "http",
	"duration": 30,
	"frontendRun": {
		"baseUrl": "https://jsonplaceholder.typicode.com",
		"metricsEndpoint": "http://localhost:8000/frontend/metrics",
		"targetUrls": ["/posts", "/users"]
	}
}
```

Then run:

```powershell
$env:EXPERIMENT_ID="<experiment-id>"
node playwright/runner.js
```

`runner.js` will call `/experiments/frontend/status?id=<experiment-id>` and use `frontend_run` automatically.

Backward compatibility:

- If `frontendRun` is not provided, existing defaults are used.
- Env vars (`BASE_URL`, `FAILSAFE_FRONTEND_ENDPOINT`) still override experiment config.
