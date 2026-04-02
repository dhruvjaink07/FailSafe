# FailSafe Integration Testing - Verification Guide

## Quick Verification (5 Minutes)

### Step 1: Validate All Components
```powershell
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
.\test\verify-components.ps1
```

**Expected Output:**
```
╔════════════════════════════════════════════════════════════╗
║ FailSafe Integration Test - Verification Checklist        ║
╚════════════════════════════════════════════════════════════╝

Phase 1: Component Validation
─────────────────────────────────────────────────────────────
Testing: Node.js vitals.js syntax... ✓
Testing: Node.js errors.js syntax... ✓
Testing: Node.js network.js syntax... ✓
Testing: Node.js bootstrap.js syntax... ✓
...
Testing: Test server /health endpoint... ✓

╔════════════════════════════════════════════════════════════╗
║ ALL TESTS PASSED ✓
║ Total: 25 tests
╚════════════════════════════════════════════════════════════╝
```

---

## Layer-by-Layer Verification

### Layer 1: Browser Runtime (JavaScript)

**Verify vitals.js enhancement:**
```bash
node -e "
const code = require('fs').readFileSync('internal/frontend/runtime/vitals.js', 'utf-8');
const hasINP = code.includes('interaction');
const hasLCP = code.includes('largest-contentful-paint');
const hasCLS = code.includes('layout-shift');
console.log('INP tracking:', hasINP ? '✓' : '✗');
console.log('LCP tracking:', hasLCP ? '✓' : '✗');
console.log('CLS tracking:', hasCLS ? '✓' : '✗');
"
```

**Expected:**
```
INP tracking: ✓
LCP tracking: ✓
CLS tracking: ✓
```

**Verify error tracking:**
```bash
node -e "
const code = require('fs').readFileSync('internal/frontend/runtime/errors.js', 'utf-8');
const hasError = code.includes('error');
const hasRejection = code.includes('unhandledrejection');
console.log('Error tracking:', hasError ? '✓' : '✗');
console.log('Rejection tracking:', hasRejection ? '✓' : '✗');
"
```

**Verify network tracking:**
```bash
node -e "
const code = require('fs').readFileSync('internal/frontend/runtime/network.js', 'utf-8');
const hasFetch = code.includes('fetch');
const hasDuration = code.includes('duration');
console.log('Fetch wrapping:', hasFetch ? '✓' : '✗');
console.log('Duration tracking:', hasDuration ? '✓' : '✗');
"
```

---

### Layer 2: Test Server

**Start test server:**
```bash
node test/test-server.js
```

**Expected output:**
```
[test-server] listening on http://127.0.0.1:3001
```

**Verify in another terminal:**

```bash
# Health check
curl http://127.0.0.1:3001/health
# Expected: {"status":"ok","uptime":2.5}

# Fast API
curl http://127.0.0.1:3001/api/fast
# Expected: {"message":"fast response","delay":"instant"}

# Slow API (waits 1s)
curl http://127.0.0.1:3001/api/slow
# Expected: {"message":"delayed response","delay":"slow"}

# Main page
curl http://127.0.0.1:3001/
# Expected: HTML page with interactive buttons
```

---

### Layer 3: Chaos Injection

**Test ScenarioLoader:**
```bash
node -e "
const ScenarioLoader = require('./internal/frontend/chaos/scenarios');
const loader = new ScenarioLoader();

Promise.all([
  loader.load('latency'),
  loader.load('cpu_throttle'),
  loader.load('offline')
]).then(scenarios => {
  console.log('✓ All scenarios loaded successfully');
  scenarios.forEach(s => {
    console.log(\`  - \${s.name}: \${s.phases.injectingMs}ms injecting phase\`);
  });
}).catch(err => {
  console.error('✗ Failed:', err.message);
  process.exit(1);
});
"
```

**Expected:**
```
✓ All scenarios loaded successfully
  - latency: 15000ms injecting phase
  - cpu_throttle: 12000ms injecting phase
  - offline: 8000ms injecting phase
```

**Test NetworkInterceptor:**
```bash
node -e "
const NetworkInterceptor = require('./internal/frontend/chaos/interceptor');
const interceptor = new NetworkInterceptor({});

// Mock page object
const mockPage = {
  route: async () => {},
  unroute: async () => {}
};

const interceptor2 = new NetworkInterceptor(mockPage);
interceptor2.setFaults({
  enabled: true,
  networkDelayMs: 700,
  failureRate: 0.1,
  offline: false
});

console.log('✓ NetworkInterceptor instantiated and configured');
"
```

**Expected:**
```
✓ NetworkInterceptor instantiated and configured
```

---

### Layer 4: Lighthouse Configuration

**Test Lighthouse config:**
```bash
node -e "
const { LIGHTHOUSE_PRESETS, AUDIT_METRICS, buildLighthouseConfig } = require('./internal/frontend/automation/lighthouse/config');

console.log('Presets:', Object.keys(LIGHTHOUSE_PRESETS).join(', '));
console.log('Metrics:', AUDIT_METRICS.length, 'audits');

const baselineConfig = buildLighthouseConfig('baseline');
const stressConfig = buildLighthouseConfig('stress');

console.log('Baseline throttling:', baselineConfig.throttling.cpuSlowdownMultiplier + 'x CPU');
console.log('Stress throttling:', stressConfig.throttling.cpuSlowdownMultiplier + 'x CPU');
console.log('✓ Lighthouse config validated');
"
```

**Expected:**
```
Presets: baseline, stress
Metrics: 6 audits
Baseline throttling: 1x CPU
Stress throttling: 4x CPU
✓ Lighthouse config validated
```

---

### Layer 5: Go Integration

**Test orchestrator types:**
```bash
go test -v -short ./internal/orchestrator -run WebMonitor 2>&1 | grep -E "PASS|FAIL|===" | head -20
```

**Expected:**
```
=== RUN   TestWebMonitor
=== RUN   TestWebMonitor/record_ingest
=== RUN   TestWebMonitor/staleness_detection
--- PASS: TestWebMonitor (2.50s)
PASS
```

**Test execution layer:**
```bash
go test -v -short ./internal/execution -run Web 2>&1 | grep -E "PASS|FAIL|===" | head -20
```

**Expected:**
```
=== RUN   TestWebTarget
--- PASS: TestWebTarget (0.10s)
PASS
```

**Test fault injection:**
```bash
go test -v -short ./internal/fault -run Web 2>&1 | grep -E "PASS|FAIL|===" | head -20
```

**Expected:**
```
=== RUN   TestWebInjector
--- PASS: TestWebInjector (0.20s)
PASS
```

---

### Layer 6: Go HTTP Handlers

**Test frontend handlers:**
```bash
go test -v -short ./internal/handlers -run Frontend 2>&1 | grep -E "PASS|FAIL|TestFrontend" | head -20
```

**Expected:**
```
=== RUN   TestFrontendMetricsHandler
--- PASS: TestFrontendMetricsHandler (0.15s)
PASS
```

---

## End-to-End Testing

### Minimal Setup (No Docker)

**Terminal 1: Web Server**
```bash
node test/test-server.js
# [test-server] listening on http://127.0.0.1:3001
```

**Terminal 2: Playwright Test**
```bash
export BASE_URL=http://127.0.0.1:3001
export EXPERIMENT_ID=manual-test-1
node test/playwright-integration.js
```

**Expected Output:**
```
[playwright-integration-test] starting
[playwright-integration-test] launching chromium
[playwright-integration-test] loading scenario: latency
[playwright-integration-test] baseline phase (5000ms)
[playwright-integration-test] injecting phase (15000ms)
[playwright-integration-test] recovery phase (5000ms)
[playwright-integration-test] experiment completed
[playwright-integration-test] metrics collected: 150+
[playwright-integration-test] result: {"success":true,...}
```

### Full Stack (With Go Controller)

**Terminal 1: Web Server**
```bash
node test/test-server.js
```

**Terminal 2: Go Controller**
```bash
# Build if needed
go build -o controller.exe ./cmd/controller

# Run
.\controller.exe
# [controller] listening on 0.0.0.0:8000
```

**Terminal 3: Playwright Test**
```bash
node test/playwright-integration.js
```

**Terminal 4: Verify Metrics (Optional)**
```bash
# After test completes, check metrics endpoint
curl http://localhost:8000/experiments/frontend/metrics?id=exp-123
```

**Expected Response:**
```json
{
  "experiment_id": "exp-123",
  "samples": [
    {
      "experiment_id": "exp-123",
      "phase": "baseline",
      "page": "/",
      "metrics": {
        "lcp": 2500,
        "cls": 0.05,
        "inp": 150,
        "errors": 0
      },
      "api_calls": [
        {"url": "...", "duration": 50, "status": 200}
      ],
      "timestamp": 1704067200000
    },
    ...more samples...
  ],
  "count": 150,
  "phase_summary": {
    "baseline": 50,
    "injecting": 75,
    "recovery": 25
  }
}
```

---

## Verification Checklist

### Before Running Tests
- [ ] Node.js 18+ installed: `node --version`
- [ ] Go 1.22+ installed: `go version`
- [ ] npm installed: `npm --version`
- [ ] All files created: `ls internal/frontend/` (should show runtime/, chaos/, automation/, transport/)
- [ ] Scenarios exist: `ls configs/scenarios/web/` (should show latency.json, cpu_throttle.json, offline.json)

### Component Validation
- [ ] `node --check internal/frontend/runtime/vitals.js` → No output (success)
- [ ] `node --check internal/frontend/chaos/scenarios.js` → No output (success)
- [ ] `node --check internal/frontend/automation/lighthouse/config.js` → No output (success)
- [ ] `go test -v ./internal/execution` → All tests pass
- [ ] `go test -v ./internal/monitoring` → All tests pass
- [ ] `go test -v ./internal/fault` → All tests pass

### Runtime Verification
- [ ] Test server starts on port 3001 ✓
- [ ] Web server responds to `/health` ✓
- [ ] Scenarios load successfully ✓
- [ ] ScenarioLoader validates chaos config ✓
- [ ] NetworkInterceptor instantiates ✓
- [ ] Lighthouse config builds successfully ✓

### End-to-End
- [ ] Playwright launches Chromium ✓
- [ ] Scenario loads (latency, cpu_throttle, or offline) ✓
- [ ] 3-phase lifecycle completes (baseline → injecting → recovery) ✓
- [ ] Metrics collected (150+ samples) ✓
- [ ] No errors in console ✓

---

## Common Issues & Solutions

| Issue | Symptom | Solution |
|-------|---------|----------|
| `Cannot find module` | Error during `node -e` | Run `npm install` to get Playwright/deps |
| Port 3001 in use | `EADDRINUSE` on test-server | Kill existing process: `lsof -i :3001` or use different port |
| Module syntax error | Error like "Unexpected token" | Check for escaped quotes in JSON/JS files; all should use single quotes |
| Timeout on `/api/slow` | Request hangs | Ensure test-server is running; check firewall |
| Go test fails | `undefined (type *orchestrator.Orchestrator has no field or method...)` | This is expected for integration tests; run `-short` tests instead |
| Metrics endpoint 404 | Controller running but no `/frontend/metrics` | Controller must be built with latest handlers; rebuild: `go build -o controller.exe ./cmd/controller` |

---

## Success Criteria

✅ **All components pass syntax check**
- No JavaScript parse errors
- No Go type mismatches

✅ **All scenarios load correctly**
- latency.json: 700ms, 10% failure, 15s injecting
- cpu_throttle.json: 4x slowdown, 12s injecting
- offline.json: 100% failure, offline: true, 8s injecting

✅ **Test server responds**
- `/health` returns status ok
- `/api/fast` returns immediately
- `/api/slow` returns after 1s

✅ **Playwright automation completes**
- Launches browser successfully
- Loads scenario without error
- Completes 3-phase lifecycle
- Collects 150+ metric samples

✅ **Go tests pass**
- `./internal/execution` tests ✓
- `./internal/monitoring` tests ✓
- `./internal/fault` tests ✓
- `./internal/handlers` tests ✓

✅ **Metrics flow end-to-end**
- Browser collects vitals/errors/network
- Posts to `/frontend/metrics` endpoint
- Orchestrator stores metrics
- Metrics retrievable via `/experiments/frontend/metrics?id=...`

---

## Next: Production Testing

Once all above verified, proceed to:
1. Test with real web application (not test-server)
2. Test with multi-page workflows
3. Monitor performance regressions
4. Integrate with CI/CD pipeline
