# FailSafe Testing - Copy-Paste Commands

## 🚀 QUICK START (Choose One)

### Option 1: Automated Full Suite (Recommended)
```powershell
cd d:\FailSafe
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
.\test\integration-test.ps1
```

**What happens:**
- Starts test web server ✓
- Starts Go controller ✓
- Health checks pass ✓
- Runs Playwright automation ✓
- Runs Go tests ✓
- Prints summary ✓

**Expected time:** ~40 seconds

---

### Option 2: Component Validation Only (5 minutes)
```powershell
cd d:\FailSafe
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
.\test\verify-components.ps1
```

**What happens:**
- Validates all JavaScript syntax ✓
- Validates all JSON files ✓
- Tests module imports ✓
- Tests scenario loading ✓
- Tests Go compilation ✓
- Prints pass/fail count ✓

**Expected output:**
```
╔════════════════════════════════════════════════════════════╗
║ ALL TESTS PASSED ✓
║ Total: 32 tests
╚════════════════════════════════════════════════════════════╝
```

**Expected time:** ~5 minutes

---

### Option 3: Manual Step-by-Step (Debugging)

#### Terminal 1: Start Test Web Server
```bash
cd d:\FailSafe
node test/test-server.js
```

**Expect:**
```
[test-server] listening on http://127.0.0.1:3001
```

#### Terminal 2: Start Go Controller
```bash
cd d:\FailSafe
go build -o controller.exe ./cmd/controller
.\controller.exe
```

**Expect:**
```
[controller] listening on 0.0.0.0:8000
```

#### Terminal 3: Run Playwright Test
```bash
cd d:\FailSafe
node test/playwright-integration.js
```

**Expect:**
```
[playwright-integration-test] starting
[playwright-integration-test] baseline phase (5000ms)
[playwright-integration-test] injecting phase (15000ms)
[playwright-integration-test] recovery phase (5000ms)
[playwright-integration-test] metrics collected: 150+
[playwright-integration-test] result: {"success":true,...}
```

#### Terminal 4: Check Metrics (After test completes)
```bash
curl http://localhost:8000/experiments/frontend/metrics?id=exp-1 | jq .
```

**Expect:**
```json
{
  "metrics": [
    {
      "experiment_id": "...",
      "phase": "baseline",
      "metrics": {
        "lcp": 2500,
        "cls": 0.05,
        "inp": 150,
        "errors": 0
      }
    },
    ...150+ samples...
  ]
}
```

---

## 📋 VERIFICATION COMMANDS

### 1. Validate Syntax (No Errors Expected)
```bash
cd d:\FailSafe

# JavaScript files
node --check internal/frontend/runtime/vitals.js
node --check internal/frontend/runtime/errors.js
node --check internal/frontend/runtime/network.js
node --check internal/frontend/chaos/scenarios.js
node --check internal/frontend/chaos/interceptor.js
node --check internal/frontend/automation/lighthouse/config.js

# All at once
for ($file in Get-ChildItem -Recurse internal/frontend -Include '*.js' | Select-Object -ExpandProperty FullName) {
    node --check $file && echo "✓ $file"
}
```

### 2. Validate JSON (Silent Success)
```bash
cd d:\FailSafe

# Individual
Get-Content configs/scenarios/web/latency.json | ConvertFrom-Json > $null && echo "✓ latency.json"
Get-Content configs/scenarios/web/cpu_throttle.json | ConvertFrom-Json > $null && echo "✓ cpu_throttle.json"
Get-Content configs/scenarios/web/offline.json | ConvertFrom-Json > $null && echo "✓ offline.json"

# All at once
Get-ChildItem configs/scenarios/web/*.json | ForEach-Object {
    Get-Content $_ | ConvertFrom-Json > $null && Write-Host "✓ $($_.Name)"
}
```

### 3. Test Module Imports
```bash
cd d:\FailSafe

# ScenarioLoader
node -e "
const ScenarioLoader = require('./internal/frontend/chaos/scenarios');
console.log(typeof ScenarioLoader === 'function' ? '✓ ScenarioLoader' : '✗ Failed');
"

# NetworkInterceptor
node -e "
const NetworkInterceptor = require('./internal/frontend/chaos/interceptor');
console.log(typeof NetworkInterceptor === 'function' ? '✓ NetworkInterceptor' : '✗ Failed');
"

# Lighthouse config
node -e "
const cfg = require('./internal/frontend/automation/lighthouse/config');
console.log(cfg.LIGHTHOUSE_PRESETS ? '✓ Lighthouse config' : '✗ Failed');
"
```

### 4. Test Scenario Loading
```bash
cd d:\FailSafe

node -e "
const Loader = require('./internal/frontend/chaos/scenarios');
const l = new Loader();

Promise.all([
  l.load('latency'),
  l.load('cpu_throttle'),
  l.load('offline')
]).then(scenarios => {
  console.log('✓ All scenarios loaded');
  scenarios.forEach(s => console.log('  -', s.name, s.chaos.networkDelayMs || s.chaos.cpuSlowdownRate, 'config'));
}).catch(e => { console.error('✗', e.message); process.exit(1); });
"
```

**Expect:**
```
✓ All scenarios loaded
  - latency 700 config
  - cpu_throttle 4 config
  - offline 100 config
```

### 5. Test Server Health
```bash
cd d:\FailSafe

# Start server
$server = Start-Process -FilePath node -ArgumentList "test/test-server.js" -PassThru

# Wait for startup
Start-Sleep -Seconds 2

# Health check
$health = Invoke-WebRequest http://127.0.0.1:3001/health -TimeoutSec 5
$health.Content | ConvertFrom-Json | Select-Object status

# API test
$fast = Invoke-WebRequest http://127.0.0.1:3001/api/fast -TimeoutSec 5
$fast.Content | ConvertFrom-Json | Select-Object message

# Cleanup
Stop-Process -Id $server.Id -Force
```

**Expect:**
```
status
------
ok

message
-------
fast response
```

### 6. Go Unit Tests
```bash
cd d:\FailSafe

# Data structures & models
go test -v -short ./internal/models

# Execution layer (web_target)
go test -v -short ./internal/execution

# Monitoring layer (web_monitor)
go test -v -short ./internal/monitoring

# Fault injection layer (web_injector)
go test -v -short ./internal/fault

# HTTP handlers
go test -v -short ./internal/handlers

# All together
go test -v -short ./cmd/controller ./internal/execution ./internal/monitoring ./internal/fault ./internal/handlers
```

**Expect:**
```
ok  	github.com/dhruvjaink07/failsafe/internal/execution	0.05s
ok  	github.com/dhruvjaink07/failsafe/internal/monitoring	0.10s
ok  	github.com/dhruvjaink07/failsafe/internal/fault	0.08s
ok  	github.com/dhruvjaink07/failsafe/internal/handlers	0.15s
```

---

## 🎯 SUCCESS CRITERIA

### All Tests Pass If:
```
✓ verify-components.ps1 shows 32 tests passed
✓ All *.js files pass node --check
✓ All *.json files parse without error
✓ ScenarioLoader loads all 3 scenarios
✓ Test server responds to /health, /api/fast, /api/slow
✓ Playwright completes 3-phase lifecycle
✓ Metrics collected: 150+ samples
✓ Metrics contain vitals (lcp, cls, inp)
✓ All Go tests pass
✓ No timeouts or fatal errors
```

---

## 📊 EXPECTED OUTPUTS BY PHASE

### Phase 1: Baseline (5 seconds)
```
[playwright-integration-test] baseline phase (5000ms)
Metrics: page loaded, no faults applied
Expected: LCP ~2500ms, CLS ~0.05, Errors 0
```

### Phase 2: Injecting (10-15 seconds)
```
[playwright-integration-test] injecting phase (15000ms)
Metrics: network delay + failures applied
Expected: LCP ~3200ms (+30%), CLS ~0.12 (+200%), API duration 750ms (+700ms)
```

### Phase 3: Recovery (5 seconds)
```
[playwright-integration-test] recovery phase (5000ms)
Metrics: faults removed, stabilization
Expected: LCP ~2500ms (back to baseline), CLS ~0.05, Errors return to 0
```

---

## ❌ FAILURE DIAGNOSIS

### If Component Validation Fails:
```powershell
# Check each file individually
node --check internal/frontend/runtime/vitals.js
# If error, inspect file:
cat internal/frontend/runtime/vitals.js | head -20
```

### If Test Server Won't Start:
```powershell
# Check port in use
netstat -ano | findstr :3001
# Kill process if needed
Get-Process node | Stop-Process -Force
# Try again
node test/test-server.js
```

### If Playwright Fails:
```bash
# Check Chromium installed
npm list @playwright/test
# Reinstall if needed
npm install

# Test in headed mode (see browser)
set HEADED=1
node test/playwright-integration.js
```

### If Go Tests Fail:
```bash
# Check Go version
go version

# Clean build
go clean -testcache
go test -v ./internal/execution

# Run with verbose error
go test -v ./internal/execution 2>&1 | tail -50
```

---

## 📝 LOGGING CHECKLIST

After tests complete, verify:

✓ **No errors in console**
```bash
# Grep for errors in latest output
# Should return nothing
```

✓ **All phases completed**
```bash
# Should see all three lines:
# [playwright-integration-test] baseline phase
# [playwright-integration-test] injecting phase
# [playwright-integration-test] recovery phase
```

✓ **Metrics collected**
```bash
# Should see:
# [playwright-integration-test] metrics collected: 150+
```

✓ **Go tests passed**
```bash
# Should see:
# ok	github.com/dhruvjaink07/failsafe/internal/...
# (no FAIL lines)
```

---

## 🎓 LEARNING RESOURCES

If tests fail, check:
1. **Syntax**: `docs/verification-guide.md` - Expected outputs for each layer
2. **Results**: `docs/test-results-reference.md` - Sample metric structures
3. **Architecture**: `docs/integration-testing.md` - How components interact
4. **Troubleshooting**: `INTEGRATION_TESTING_STATUS.md` - Common issues & fixes

---

## ⏱️ TIME BREAKDOWN

| Command | Time | Notes |
|---------|------|-------|
| `verify-components.ps1` | 5 min | No servers, just syntax/imports |
| `integration-test.ps1` | 40 sec | Full automated suite |
| Manual step-by-step | 2-3 min | Useful for debugging |
| Go tests only | 1 min | `go test -v -short ./...` |

---

## 📦 CLEANUP

If servers hang:
```powershell
# Kill all Node processes
Get-Process node | Stop-Process -Force

# Kill all Go processes
Get-Process controller | Stop-Process -Force

# Verify clean
Get-Process | grep -E "node|controller"
# Should return nothing
```

---

## ✅ SIGN-OFF

Run this to confirm everything works:
```powershell
cd d:\FailSafe
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
.\test\integration-test.ps1
```

**If you see:**
```
╔════════════════════════════════════════════════════════════╗
║ Integration Test Suite PASSED ✓
╚════════════════════════════════════════════════════════════╝
```

**✅ You're done!** All layers verified and working.
