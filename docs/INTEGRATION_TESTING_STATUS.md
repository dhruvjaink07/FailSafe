# Integration Testing Checklist & Status

## ✓ Completed Components

### Browser Runtime Layer (JavaScript)
- [x] **vitals.js** - Web Vitals tracking (LCP, CLS, INP)
  - PerformanceObserver for LCP/CLS
  - INP support with feature detection
  - Clean, modular API: `__FAILSAFE_INSTALL_VITALS__(state)`

- [x] **errors.js** - Error & exception tracking
  - Catches `window.error` events
  - Catches `unhandledrejection` events
  - Counter-based reporting (privacy-first)

- [x] **network.js** - Network call tracking
  - Wraps native fetch() API
  - Tracks duration, status, URL per request
  - Transparent pass-through behavior

- [x] **bootstrap.js** - Runtime orchestration
  - Installs all three trackers into shared state
  - Called by Playwright with `state` object
  - Exports `__FAILSAFE_BOOTSTRAP_RUNTIME__` factory

- [x] **collector.js** - Metric batching
  - PerformanceObserver integration
  - Fetch interception for API tracking
  - Payload normalization

- [x] **sender.js** - Metric transport
  - Batch queue with auto-flush
  - Exponential backoff retry
  - sendBeacon + fetch POST to backend

### Chaos Control Layer (Node.js)
- [x] **chaos/scenarios.js** - Scenario loader & validator
  - Load scenarios by name: `await loader.load('latency')`
  - Deep merge with defaults
  - Full schema validation (pages, phases, chaos config)
  - Exported: `ScenarioLoader` class

- [x] **chaos/interceptor.js** - Network fault injection
  - Playwright route handler pattern
  - Supports: networkDelayMs, failureRate, offline, targetUrls
  - Methods: `setFaults()`, `install()`, `uninstall()`
  - Exported: `NetworkInterceptor` class

- [x] **chaos/service_worker.js** - Offline mode
  - Service worker for browser context
  - Intercepts all fetch requests
  - IndexedDB fallback for offline config
  - Exported: Service Worker script (injectable)

### Profiling Layer (Node.js)
- [x] **lighthouse/config.js** - Lighthouse configuration
  - LIGHTHOUSE_PRESETS for baseline/stress
  - AUDIT_METRICS array (fcp, lcp, cls, tbt, tti, speed-index)
  - Factory: `buildLighthouseConfig(phase)` → audit config object
  - Exported: presets, metrics, factory

- [x] **lighthouse/runner.js** - Lighthouse executor
  - LighthouseProfiler class wraps lighthouse module
  - Methods: `runAudit(pageUrl, phase)` → normalized metrics
  - Handles errors gracefully
  - Returns: {phase, pageUrl, success, metrics, score}

### Scenario Configurations (JSON)
- [x] **latency.json** - Network latency scenario
  - 700ms delay, 10% failure rate
  - 2 pages: "/" + "/products"
  - 15s injecting phase

- [x] **cpu_throttle.json** - CPU throttling scenario
  - 4x CPU slowdown
  - 2 pages: "/" + "/dashboard"
  - 12s injecting phase

- [x] **offline.json** - Offline fault scenario
  - 100% failure rate, offline: true
  - 2 pages: "/" + "/cached"
  - 8s injecting phase

### Go Infrastructure (Already Existing)
- [x] **web_target.go** - First-class web execution adapter
  - Implements Target interface (same as docker/android)
  - Factory: `NewWebTarget(injector, monitor)`
  - Delegates to BaseTarget

- [x] **web_monitor.go** - Passive frontend monitoring
  - Implements MonitorInterface
  - `RecordIngest(FrontendMetrics)` for push-based collection
  - Staleness detection (6s threshold)
  - Fires callbacks on metric ingestion

- [x] **web_injector.go** - Browser fault control
  - Implements Injector interface
  - `Inject(config)` stores WebCommand with TTL
  - Supports: network_delay, packet_loss, cpu_throttle, offline, js_chaos
  - `LatestCommand(expID)` checks expiration

- [x] **frontend_handlers.go** - HTTP lifecycle handlers
  - POST /frontend/metrics → metrics ingestion
  - GET /experiments/frontend/start → experiment startup
  - GET /experiments/frontend/status → query status
  - GET /experiments/frontend/stop → experiment shutdown
  - GET /experiments/frontend/fault-command → serve active faults
  - GET /experiments/frontend/metrics → retrieve collected metrics

### Testing Infrastructure
- [x] **test-server.js** - Test web application
  - Simple HTTP server (port 3001)
  - Endpoints: /, /health, /api/fast, /api/slow
  - Interactive HTML with logging UI
  - CORS-enabled

- [x] **playwright-integration.js** - Playwright test harness
  - Tests scenario loading
  - Tests network interceptor installation
  - Tests 3-phase lifecycle
  - Logs collected metrics

- [x] **integration-test.ps1** - Full suite orchestrator
  - Starts test server
  - Starts Go controller
  - Health checks
  - Runs Playwright tests
  - Runs Go tests
  - Cleanup

### Documentation
- [x] **integration-testing.md** - Complete testing guide
  - Architecture overview
  - Component descriptions
  - Quick start instructions
  - Scenario definitions
  - Troubleshooting guide
  - Metrics flow diagram

---

## 🚀 Ready to Test

### Prerequisites Check
```bash
# Required tools
node --version          # v18+
go version             # v1.22+
npm --version          # v9+

# Build controller
go build -o controller.exe ./cmd/controller

# Install dependencies
npm install
```

### Run Integration Test
```powershell
# Set execution policy (one-time)
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process

# Run full suite
.\test\integration-test.ps1
```

### Expected Timeline
- **Server startup**: 2-3s
- **Playwright baseline**: 5s
- **Playwright injecting**: 10s
- **Playwright recovery**: 5s
- **Go tests**: 10-15s
- **Total**: ~40s

### Expected Output
```
[integration-test] FailSafe Integration Test Suite
[integration-test] Experiment ID: integration-test-20260402143022
[integration-test] Base URL: http://127.0.0.1:3001
[integration-test] Metrics Endpoint: http://127.0.0.1:8000/frontend/metrics

[integration-test] Starting test web server... 
[integration-test] Test server started (PID: 12345)

[integration-test] Starting Go controller...
[integration-test] Controller started (PID: 12346)

[integration-test] Running health checks...
[integration-test] ✓ Web server healthy
[integration-test] ✓ Controller healthy

[integration-test] Running Playwright integration test...
[playwright-integration-test] baseline phase (5000ms)
[playwright-integration-test] injecting phase (10000ms)
[playwright-integration-test] recovery phase (5000ms)
[playwright-integration-test] result: {"success":true,"scenario":{...},"metricsCollected":150}
[integration-test] ✓ Playwright test passed

[integration-test] Running Go tests...
[integration-test] ✓ Go tests passed

[integration-test] ╔════════════════════════════════════════╗
[integration-test] ║ Integration Test Suite PASSED ✓ ║
[integration-test] ╚════════════════════════════════════════╝
```

---

## 📋 Verification Steps

After running integration tests, verify:

### 1. Metrics Flow
```bash
# Check orchestrator received metrics
curl -s http://localhost:8000/experiments/frontend/metrics?id=integration-test-* | jq .
```

### 2. Test Server Response
```bash
# Verify test server is functional
curl -s http://localhost:3001/health | jq .
curl -s http://localhost:3001/api/fast | jq .
curl -s http://localhost:3001/api/slow | jq .
```

### 3. Scenario Loading
```bash
# Test scenario loader
node -e "
const Loader = require('./internal/frontend/chaos/scenarios');
const l = new Loader();
l.load('latency').then(s => console.log(JSON.stringify(s, null, 2)));
"
```

### 4. Chaos Injection
```bash
# Verify interceptor can be instantiated
node -e "
const Interceptor = require('./internal/frontend/chaos/interceptor');
const i = new Interceptor({});
i.setFaults({enabled: true, networkDelayMs: 500});
console.log('Interceptor ready');
"
```

---

## 🎯 Next Phase: Advanced Testing

### Performance Profiling
- [ ] Run Lighthouse audits (baseline vs stress)
- [ ] Compare metrics across scenarios
- [ ] Generate performance reports

### Extended Scenarios
- [ ] Test with real web applications
- [ ] Multi-page workflows
- [ ] Concurrent experiments

### CI/CD Integration
- [ ] GitHub Actions workflow
- [ ] Automated test reporting
- [ ] Performance regression detection

### Monitoring & Analytics
- [ ] Visualization dashboard
- [ ] Metrics aggregation
- [ ] Historical trend analysis

---

## 📝 Notes

- All modules are decoupled: chaos/ and runtime/ are independent of runner.js
- Playwright runner already integrates applyChaos() logic
- Metrics are collected via RUM (Real User Monitoring)
- No modifications to docker/android subsystems
- Storage is in-memory for tests (PostgreSQL optional for production)

---

## ✅ Sign-Off

**Integration Testing Architecture**: COMPLETE ✓
- Browser runtime: ✓ vitals, errors, network tracking
- Chaos control: ✓ scenario loading, network interception, offline mode
- Profiling: ✓ Lighthouse config & runner
- Go handlers: ✓ lifecycle, metrics, fault command
- Test suite: ✓ server, automation, orchestration
- Documentation: ✓ guide, troubleshooting, metrics flow

**Ready for**: Full end-to-end testing with Playwright + Go orchestrator
