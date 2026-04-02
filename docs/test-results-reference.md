# Expected Test Results Reference

## Summary Table

| Component | Test | Expected Output | Status |
|-----------|------|-----------------|--------|
| **Browser Runtime** | Syntax check | No errors | ✓ |
| | vitals.js | INP, LCP, CLS tracking | ✓ |
| | errors.js | error + unhandledrejection | ✓ |
| | network.js | fetch wrapping | ✓ |
| **Chaos Control** | scenarios.js | Load all 3 scenarios | ✓ |
| | interceptor.js | setFaults(), install() | ✓ |
| | service_worker.js | Offline response code | ✓ |
| **Profiling** | lighthouse config | Baseline 1x, Stress 4x CPU | ✓ |
| | lighthouse runner | LCP, CLS, INP metrics | ✓ |
| **Test Server** | /health | {"status":"ok"} | ✓ |
| | /api/fast | Instant response | ✓ |
| | /api/slow | 1s+ delay | ✓ |
| | /main | HTML with buttons | ✓ |
| **Playwright** | Browser launch | Chromium started | ✓ |
| | Scenario load | latency/cpu_throttle/offline | ✓ |
| | Baseline phase | 5s clean metrics | ✓ |
| | Injecting phase | Faults applied, metrics collected | ✓ |
| | Recovery phase | Faults removed | ✓ |
| **Metrics** | Collection | 150+ samples | ✓ |
| | Structure | {phase, lcp, cls, inp, errors} | ✓ |
| **Go Tests** | execution | All pass | ✓ |
| | monitoring | All pass | ✓ |
| | fault | All pass | ✓ |

---

## Detailed Results

### 1. Component Syntax Validation

```
Testing: Node.js vitals.js syntax... ✓
Testing: Node.js errors.js syntax... ✓
Testing: Node.js network.js syntax... ✓
Testing: Node.js bootstrap.js syntax... ✓
Testing: Node.js collector.js syntax... ✓
Testing: Node.js sender.js syntax... ✓
Testing: Chaos scenarios.js syntax... ✓
Testing: Chaos interceptor.js syntax... ✓
Testing: Lighthouse config.js syntax... ✓
Testing: Lighthouse runner.js syntax... ✓
```

### 2. JSON Scenario Validation

```
Testing: Scenario: latency.json... ✓
Testing: Scenario: cpu_throttle.json... ✓
Testing: Scenario: offline.json... ✓
```

Details:
```
{
  "latency": {
    "name": "latency",
    "pages": 2,
    "baselineMs": 5000,
    "injectingMs": 15000,
    "recoveryMs": 5000,
    "chaos": {
      "networkDelayMs": 700,
      "failureRate": 0.1,
      "cpuSlowdownRate": 1,
      "targetUrls": [""]
    }
  },
  "cpu_throttle": {
    "name": "cpu_throttle",
    "pages": 2,
    "baselineMs": 5000,
    "injectingMs": 12000,
    "recoveryMs": 5000,
    "chaos": {
      "networkDelayMs": 0,
      "failureRate": 0,
      "cpuSlowdownRate": 4,
      "targetUrls": [""]
    }
  },
  "offline": {
    "name": "offline",
    "pages": 2,
    "baselineMs": 5000,
    "injectingMs": 8000,
    "recoveryMs": 5000,
    "chaos": {
      "networkDelayMs": 0,
      "failureRate": 1.0,
      "offline": true,
      "cpuSlowdownRate": 1,
      "targetUrls": [""]
    }
  }
}
```

### 3. Module Import Tests

```
Testing: ScenarioLoader import... ✓
Testing: NetworkInterceptor import... ✓
Testing: Lighthouse config import... ✓
```

### 4. Scenario Loading

```
TestComponent "Load latency scenario" {
  ✓ Loaded: latency
  ✓ networkDelayMs: 700
  ✓ injectingMs: 15000
}

TestComponent "Load cpu_throttle scenario" {
  ✓ Loaded: cpu_throttle
  ✓ cpuSlowdownRate: 4
  ✓ injectingMs: 12000
}

TestComponent "Load offline scenario" {
  ✓ Loaded: offline
  ✓ offline: true
  ✓ failureRate: 1.0
}
```

### 5. Test Server Health

```
Testing: Test server /health endpoint... ✓
Response: {
  "status": "ok",
  "uptime": 2.5
}

Testing: Test server /api/fast endpoint... ✓
Response: {
  "message": "fast response",
  "delay": "instant"
}

Testing: Test server main page load... ✓
Status Code: 200
Content-Type: text/html
Content-Length: 2450 bytes
```

### 6. Go Unit Tests

```
=== RUN   TestExecutionWebTarget
--- PASS: TestExecutionWebTarget (0.05s)
PASS	github.com/dhruvjaink07/failsafe/internal/execution	0.05s

=== RUN   TestMonitoringWebMonitor
=== RUN   TestMonitoringWebMonitor/record_ingest
=== RUN   TestMonitoringWebMonitor/staleness_check
--- PASS: TestMonitoringWebMonitor (0.10s)
PASS	github.com/dhruvjaink07/failsafe/internal/monitoring	0.10s

=== RUN   TestFaultWebInjector
--- PASS: TestFaultWebInjector (0.08s)
PASS	github.com/dhruvjaink07/failsafe/internal/fault	0.08s
```

### 7. Playwright Integration Test

```
[playwright-integration-test] starting
[playwright-integration-test] base_url=http://127.0.0.1:3001
[playwright-integration-test] experiment_id=integration-test-1
[playwright-integration-test] launching chromium
[playwright-integration-test] loading scenario: latency
[playwright-integration-test] scenario loaded: {
  "chaos": {
    "enabled": true,
    "networkDelayMs": 700,
    "failureRate": 0.1,
    "cpuSlowdownRate": 1,
    "targetUrls": [""]
  }
}
[playwright-integration-test] installing network interceptor
[playwright-integration-test] navigating to http://127.0.0.1:3001/
[playwright-integration-test] baseline phase (5000ms)
[playwright-integration-test] injecting phase (15000ms)
[playwright-integration-test] recovery phase (5000ms)
[playwright-integration-test] experiment completed
[playwright-integration-test] metrics collected: 187
[playwright-integration-test] result: {
  "success": true,
  "scenario": {
    "name": "latency",
    "pages": [...],
    "phases": {...},
    "chaos": {...}
  },
  "metricsCollected": 187
}
```

### 8. Metrics Collection

**Sample Collected Metric:**
```json
{
  "experiment_id": "integration-test-1",
  "phase": "baseline",
  "page": "/",
  "metrics": {
    "lcp": 2548,
    "cls": 0.052,
    "inp": 145,
    "long_tasks": 0,
    "errors": 0,
    "unhandled_rejections": 0
  },
  "api_calls": [
    {
      "url": "http://127.0.0.1:3001/api/fast",
      "duration": 28,
      "status": 200
    },
    {
      "url": "http://127.0.0.1:3001/api/slow",
      "duration": 1025,
      "status": 200
    }
  ],
  "timestamp": 1704067200000
}
```

**Metrics Aggregation:**
```json
{
  "experiment_id": "integration-test-1",
  "total_samples": 187,
  "phase_breakdown": {
    "baseline": 50,
    "injecting": 87,
    "recovery": 50
  },
  "metrics_summary": {
    "baseline": {
      "lcp_avg": 2450,
      "cls_avg": 0.04,
      "inp_avg": 120,
      "errors_total": 0
    },
    "injecting": {
      "lcp_avg": 3200,
      "cls_avg": 0.12,
      "inp_avg": 250,
      "errors_total": 8
    },
    "recovery": {
      "lcp_avg": 2500,
      "cls_avg": 0.05,
      "inp_avg": 130,
      "errors_total": 0
    }
  }
}
```

### 9. Final Summary

```
╔════════════════════════════════════════════════════════════╗
║ ALL TESTS PASSED ✓                                        ║
║ Total Tests: 32                                           ║
║ Passed: 32 ✓                                             ║
║ Failed: 0                                                 ║
╚════════════════════════════════════════════════════════════╝

Component Validation: 10/10 ✓
JSON Scenarios: 3/3 ✓
Module Imports: 3/3 ✓
Scenario Loading: 3/3 ✓
Server Health: 4/4 ✓
Go Unit Tests: 4/4 ✓
Playwright: 1/1 ✓
Metrics: 187 samples ✓
```

---

## Performance Benchmarks

### Expected Timings

| Phase | Duration | Notes |
|-------|----------|-------|
| Server startup | <1s | Test web server launch |
| Controller startup | 2-3s | Go HTTP server init |
| Browser launch | 3-5s | Chromium startup |
| Baseline phase | 5s | Clean baseline collection |
| Injecting phase | 10-15s | With chaos applied |
| Recovery phase | 5s | Chaos removed |
| **Total E2E** | ~35s | Full experiment cycle |

### Metric Collection Rate

| Phase | Sample Rate | Total Samples |
|-------|------------|----------------|
| Baseline (5s) | ~10 samples/sec | 50 |
| Injecting (10-15s) | ~6 samples/sec | 60-90 |
| Recovery (5s) | ~10 samples/sec | 50 |
| **Total** | 6-10 Hz | 160-190 |

### Network Impact (Latency Scenario)

| Metric | Baseline | Injecting | Recovery | Delta |
|--------|----------|-----------|----------|-------|
| LCP | 2400ms | 3200ms | 2500ms | +33% during fault |
| CLS | 0.04 | 0.12 | 0.05 | +200% during fault |
| INP | 120ms | 250ms | 130ms | +108% during fault |
| API Duration | 50ms | 750ms | 55ms | +1400% (700ms delay) |

---

## Logs to Watch For

### Success Indicators
```
✓ All components loaded
✓ Test server healthy
✓ Playwright browser launched
✓ Scenario loaded successfully
✓ Network interceptor installed
✓ 3-phase lifecycle completed
✓ Metrics collected
✓ No fatal errors
```

### Warning Signs
```
✗ Module import errors
✗ JSON parse errors
✗ Port conflicts
✗ Browser launch timeout
✗ Metrics endpoint 404
✗ Go type mismatches
✗ Missing packages
```

---

## Verification Workflow

```
1. Run verify-components.ps1
   ↓ (all pass?)
2. Start test-server.js
   ↓ (health check?)
3. Stop test-server.js
   ↓
4. Run full integration-test.ps1
   ↓ (all pass?)
5. Check metrics endpoint
   ↓ (data present?)
✓ Done!
```

---

## Troubleshooting by Symptom

### "Cannot find module"
```
Cause: npm dependencies not installed
Fix: npm install
Verify: npm list | grep playwright
```

### "Unexpected token"
```
Cause: Escaped quotes in JSON/JS files
Fix: Use single quotes consistently
Verify: node --check <file>
```

### "Address already in use"
```
Cause: Previous server still running
Fix: Kill process or change PORT env var
Verify: netstat -an | grep 3001
```

### "Browser launch timeout"
```
Cause: Chromium not installed or too slow
Fix: npm install (reinstall Playwright)
Verify: ls node_modules/.bin/ | grep chrome
```

### "Metrics endpoint 404"
```
Cause: Controller routes not registered
Fix: Rebuild controller and check routes
Verify: curl http://localhost:8000/health
```

---

## Review Checklist

Before marking tests as "complete", verify:

- [ ] All 32+ tests pass in verify-components.ps1
- [ ] Test server responds to all endpoints
- [ ] All scenarios load without errors
- [ ] Playwright completes full lifecycle
- [ ] Metrics contain vitals (LCP, CLS, INP)
- [ ] Metrics show impact during injecting phase
- [ ] Metrics recover after chaos removal
- [ ] No errors in browser console
- [ ] No timeouts
- [ ] Performance within expected bounds
