# FailSafe Integration Testing Guide

## Overview

This guide describes how to run the complete integration test suite for FailSafe's web execution engine. The suite tests the end-to-end flow from Playwright automation through the Go orchestrator.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Playwright Runner (Node.js)                                  │
│ ├─ Loads scenario from /configs/scenarios/web/*.json        │
│ ├─ Spawns browser (Chromium)                                │
│ ├─ Injects runtime (vitals, errors, network collectors)    │
│ ├─ Executes 3-phase lifecycle:                             │
│ │  • Baseline: clean metrics (5s default)                   │
│ │  • Injecting: apply chaos + collect metrics (10s default) │
│ │  • Recovery: remove chaos + observe stabilization (5s)    │
│ └─ Polls /experiments/frontend/fault-command for active    │
│    faults & posts metrics to /frontend/metrics               │
└──────────────────┬──────────────────────────────────────────┘
                   │ HTTP
                   ▼
┌─────────────────────────────────────────────────────────────┐
│ Go Controller (HTTP Server, Port 8000)                       │
│ ├─ POST /frontend/metrics ← receives metric batches         │
│ ├─ GET /experiments/frontend/start ← starts experiment      │
│ ├─ GET /experiments/frontend/status ← queries status        │
│ ├─ GET /experiments/frontend/metrics ← retrieves metrics    │
│ ├─ GET /experiments/frontend/fault-command ← serves faults  │
│ └─ GET /experiments/frontend/stop ← stops experiment        │
└──────────────────┬──────────────────────────────────────────┘
                   │ stores
                   ▼
         ┌──────────────────┐
         │ Orchestrator     │
         │ (in-memory maps) │
         ├─ experiments    │
         ├─ monitors       │
         ├─ frontendMetrics│
         └─ faultHistory   │
         └──────────────────┘
```

## Components

### 1. **Test Server** (`test/test-server.js`)
- Simple HTTP server serving test web pages (port 3001)
- Endpoints:
  - `GET /` → renders interactive HTML page
  - `GET /health` → health check
  - `GET /api/fast` → instant response
  - `GET /api/slow` → 1s delayed response

### 2. **Playwright Automation** (`internal/frontend/automation/playwright/runner.js`)
- Orchestrates browser automation
- Loads scenarios from `/configs/scenarios/web/`
- Phases:
  - **Baseline** (5s): capture clean metrics without faults
  - **Injecting** (10s): apply chaos while collecting metrics
  - **Recovery** (5s): remove chaos, verify stabilization
- Collects metrics via RUM (Real User Monitoring):
  - Performance: LCP, CLS, INP (Web Vitals API)
  - Errors: uncaught exceptions, unhandled rejections
  - Network: API call duration/status

### 3. **Chaos Modules** (`internal/frontend/chaos/`)
- **scenarios.js**: Loads and validates scenario JSON configs
- **interceptor.js**: Playwright route handler (network faults)
- **service_worker.js**: Service worker for offline mode

### 4. **Go Orchestrator** (`internal/orchestrator/`)
- Manages experiment lifecycle
- Stores metrics in memory (or PostgreSQL)
- Tracks fault injection history
- Monitors system state

### 5. **Integration Test Script** (`test/integration-test.ps1`)
- PowerShell orchestrator for full test suite
- Starts test server + Go controller
- Runs Playwright automation
- Runs Go tests
- Cleanup on completion

## Quick Start

### Prerequisites
```bash
# Go 1.22+
go version

# Node.js 18+
node --version

# Playwright (installed via npm)
npm install

# Build Go controller
go build -o controller.exe ./cmd/controller
```

### Run Full Integration Test
```powershell
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
.\test\integration-test.ps1
```

### Run Components Manually

**Terminal 1: Test Web Server**
```bash
node test/test-server.js
# Output: [test-server] listening on http://127.0.0.1:3001
```

**Terminal 2: Go Controller**
```bash
go run ./cmd/controller
# Output: [controller] listening on :8000
```

**Terminal 3: Playwright Test**
```bash
node test/playwright-integration.js
```

## Scenario Definitions

Scenarios are JSON files in `/configs/scenarios/web/`:

### latency.json
- **Network Delay**: 700ms
- **Failure Rate**: 10%
- **Duration**: 15s injecting phase
- **Pages**: "/" → "/products"

### cpu_throttle.json
- **CPU Slowdown**: 4x
- **Duration**: 12s injecting phase
- **Pages**: "/" → "/dashboard"

### offline.json
- **Offline Mode**: 100% failure
- **Duration**: 8s injecting phase
- **Pages**: "/" → "/cached"
- **Test**: Validates retry logic

### Create Custom Scenario
```json
{
  "name": "custom-latency",
  "description": "Custom network latency test",
  "pages": [
    {
      "path": "/search",
      "actions": ["wait:1500", "fill:input.search:test", "press:Enter", "wait:2000"]
    }
  ],
  "phases": {
    "baselineMs": 5000,
    "injectingMs": 20000,
    "recoveryMs": 10000
  },
  "chaos": {
    "enabled": true,
    "networkDelayMs": 500,
    "failureRate": 0.2,
    "cpuSlowdownRate": 2,
    "targetUrls": ["/api"]
  }
}
```

## Expected Behavior

### Successful Run
```
[integration-test] Starting test web server... ✓
[integration-test] Starting Go controller... ✓
[integration-test] Web server healthy ✓
[integration-test] Controller healthy ✓
[playwright-integration-test] Baseline phase (5000ms) ✓
[playwright-integration-test] Injecting phase (10000ms) ✓
[playwright-integration-test] Recovery phase (5000ms) ✓
[integration-test] Metrics collected: 150+ samples ✓
[integration-test] Integration Test Suite PASSED ✓
```

### Troubleshooting

| Issue | Solution |
|-------|----------|
| Port 3001 already in use | Change `TEST_SERVER_PORT` env var |
| Port 8000 already in use | Change `CONTROLLER_PORT` or kill existing process |
| "Browser not found" | Run `npm install` to download Chromium |
| Metrics not collected | Check `/frontend/metrics` endpoint in controller |
| Timeout on API calls | Test web server may not be responding; check `http://127.0.0.1:3001/health` |

## Metrics Flow

1. **Browser collects** (PerformanceObserver):
   ```javascript
   {
     lcp: 2500,      // ms
     cls: 0.05,      // unitless
     inp: 150,       // ms
     errors: 0,
     apiCalls: [{url, duration, status}]
   }
   ```

2. **Batched & sent** via fetch POST:
   ```
   POST /frontend/metrics
   Content-Type: application/json
   
   {
     "metrics": [{
       "experiment_id": "exp-123",
       "phase": "injecting",
       "metrics": {...},
       "timestamp": 1704067200000
     }]
   }
   ```

3. **Orchestrator stores**:
   ```go
   orch.AddFrontendMetrics(batch)
   // stored in frontendMetrics[experimentID][]FrontendMetrics
   ```

4. **Retrieved via API**:
   ```
   GET /experiments/frontend/metrics?id=exp-123
   
   {
     "metrics": [{...}, {...}, ...],
     "count": 150,
     "phase_summary": {
       "baseline": 50,
       "injecting": 75,
       "recovery": 25
     }
   }
   ```

## Testing Fault Injection

### Via Playwright Route Handler
```javascript
// Intercepts all fetch/XHR to apply faults
await page.route('**/*', async (route) => {
  if (chaos.enabled) {
    await delay(chaos.networkDelayMs);
    if (Math.random() < chaos.failureRate) {
      await route.abort('failed');
      return;
    }
  }
  await route.continue();
});
```

### Via Go Fault Command
1. Orchestrator stores fault in `WebInjector`
2. Browser polls `GET /experiments/frontend/fault-command`
3. Browser applies fault via route interception
4. Metrics capture impact

## Monitoring & Debugging

### Enable Verbose Logging
```bash
export DEBUG=failsafe:*
node test/test-server.js
```

### Monitor Playwright
```bash
# Run with headed browser to see automation
HEADED=1 node test/playwright-integration.js
```

### Inspect Metrics
```bash
# Query collected metrics
curl http://localhost:8000/experiments/frontend/metrics?id=exp-123
```

## Next Steps

1. **Performance Profiling**
   - Integrate Lighthouse runner for baseline/stress auditing
   - Compare metrics across fault scenarios

2. **Automated Test Suite**
   - Create CI/CD jobs for continuous integration
   - Generate test reports (HTML/JSON)

3. **Real Application Testing**
   - Migrate test server to actual web application
   - Test against production-like scenarios

4. **Advanced Chaos**
   - Service Worker offline mode
   - Browser Worker CPU throttling
   - Custom fault injection strategies

## References

- [Web Vitals](https://web.dev/vitals/)
- [Playwright API](https://playwright.dev/docs/api/class-page)
- [PerformanceObserver](https://developer.mozilla.org/en-US/docs/Web/API/PerformanceObserver)
- [Service Workers](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API)
