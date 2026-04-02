#!/usr/bin/env pwsh
<#
.SYNOPSIS
  FailSafe Integration Test - Step-by-Step Verification
  
  This script guides you through testing each layer:
  1. Component validation (syntax, imports)
  2. Server connectivity
  3. Chaos injection
  4. Metrics collection
  5. End-to-end flow
#>

$ErrorActionPreference = "Continue"

# Colors for output
function Success { Write-Host $args -ForegroundColor Green }
function Warning { Write-Host $args -ForegroundColor Yellow }
function Error { Write-Host $args -ForegroundColor Red }
function Info { Write-Host $args -ForegroundColor Cyan }

Info "╔════════════════════════════════════════════════════════════╗"
Info "║ FailSafe Integration Test - Verification Checklist        ║"
Info "╚════════════════════════════════════════════════════════════╝"
Info ""

# Track result
$testsPassed = 0
$testsFailed = 0

function Test-Component {
    param([string]$Name, [scriptblock]$Test)
    
    Write-Host "Testing: $Name..." -NoNewline
    try {
        & $Test
        Success " ✓"
        $script:testsPassed++
    } catch {
        Error " ✗ FAILED"
        Error "  Error: $_"
        $script:testsFailed++
    }
}

# ─────────────────────────────────────────────────────────────
# Phase 1: Component Validation
# ─────────────────────────────────────────────────────────────

Info ""
Info "Phase 1: Component Validation"
Info "─────────────────────────────────────────────────────────────"

Test-Component "Node.js vitals.js syntax" {
    & node --check internal/frontend/runtime/vitals.js
}

Test-Component "Node.js errors.js syntax" {
    & node --check internal/frontend/runtime/errors.js
}

Test-Component "Node.js network.js syntax" {
    & node --check internal/frontend/runtime/network.js
}

Test-Component "Node.js bootstrap.js syntax" {
    & node --check internal/frontend/runtime/bootstrap.js
}

Test-Component "Node.js collector.js syntax" {
    & node --check internal/frontend/runtime/collector.js
}

Test-Component "Node.js sender.js syntax" {
    & node --check internal/frontend/transport/sender.js
}

Test-Component "Chaos scenarios.js syntax" {
    & node --check internal/frontend/chaos/scenarios.js
}

Test-Component "Chaos interceptor.js syntax" {
    & node --check internal/frontend/chaos/interceptor.js
}

Test-Component "Lighthouse config.js syntax" {
    & node --check internal/frontend/automation/lighthouse/config.js
}

Test-Component "Lighthouse runner.js syntax" {
    & node --check internal/frontend/automation/lighthouse/runner.js
}

# ─────────────────────────────────────────────────────────────
# Phase 2: JSON Validation
# ─────────────────────────────────────────────────────────────

Info ""
Info "Phase 2: JSON Scenarios Validation"
Info "─────────────────────────────────────────────────────────────"

Test-Component "Scenario: latency.json" {
    Get-Content configs/scenarios/web/latency.json | ConvertFrom-Json | Out-Null
}

Test-Component "Scenario: cpu_throttle.json" {
    Get-Content configs/scenarios/web/cpu_throttle.json | ConvertFrom-Json | Out-Null
}

Test-Component "Scenario: offline.json" {
    Get-Content configs/scenarios/web/offline.json | ConvertFrom-Json | Out-Null
}

# ─────────────────────────────────────────────────────────────
# Phase 3: Module Imports
# ─────────────────────────────────────────────────────────────

Info ""
Info "Phase 3: Module Imports & Exports"
Info "─────────────────────────────────────────────────────────────"

Test-Component "ScenarioLoader import" {
    $code = @"
        const ScenarioLoader = require('./internal/frontend/chaos/scenarios');
        if (typeof ScenarioLoader !== 'function') throw new Error('Not a class');
"@
    & node -e $code
}

Test-Component "NetworkInterceptor import" {
    $code = @"
        const NetworkInterceptor = require('./internal/frontend/chaos/interceptor');
        if (typeof NetworkInterceptor !== 'function') throw new Error('Not a class');
"@
    & node -e $code
}

Test-Component "Lighthouse config import" {
    $code = @"
        const cfg = require('./internal/frontend/automation/lighthouse/config');
        if (!cfg.LIGHTHOUSE_PRESETS) throw new Error('Missing PRESETS');
        if (!cfg.buildLighthouseConfig) throw new Error('Missing factory');
"@
    & node -e $code
}

# ─────────────────────────────────────────────────────────────
# Phase 4: Scenario Loading
# ─────────────────────────────────────────────────────────────

Info ""
Info "Phase 4: Scenario Loading & Validation"
Info "─────────────────────────────────────────────────────────────"

Test-Component "Load latency scenario" {
    $code = @"
        const ScenarioLoader = require('./internal/frontend/chaos/scenarios');
        const loader = new ScenarioLoader();
        const scenario = loader.load('latency');
        scenario.then(s => {
            if (!s.chaos) throw new Error('No chaos config');
            if (s.chaos.networkDelayMs !== 700) throw new Error('Wrong delay');
            console.log('Loaded:', s.name);
        });
"@
    & node -e $code
}

Test-Component "Load cpu_throttle scenario" {
    $code = @"
        const ScenarioLoader = require('./internal/frontend/chaos/scenarios');
        const loader = new ScenarioLoader();
        const scenario = loader.load('cpu_throttle');
        scenario.then(s => {
            if (s.chaos.cpuSlowdownRate !== 4) throw new Error('Wrong CPU rate');
            console.log('Loaded:', s.name);
        });
"@
    & node -e $code
}

Test-Component "Load offline scenario" {
    $code = @"
        const ScenarioLoader = require('./internal/frontend/chaos/scenarios');
        const loader = new ScenarioLoader();
        const scenario = loader.load('offline');
        scenario.then(s => {
            if (!s.chaos.offline) throw new Error('Not offline');
            console.log('Loaded:', s.name);
        });
"@
    & node -e $code
}

# ─────────────────────────────────────────────────────────────
# Phase 5: Go Tests
# ─────────────────────────────────────────────────────────────

Info ""
Info "Phase 5: Go Unit Tests"
Info "─────────────────────────────────────────────────────────────"

Test-Component "Go: internal/execution tests" {
    & go test -v -short ./internal/execution 2>&1 | findstr /R "PASS|FAIL"
}

Test-Component "Go: internal/monitoring tests" {
    & go test -v -short ./internal/monitoring 2>&1 | findstr /R "PASS|FAIL"
}

Test-Component "Go: internal/fault tests" {
    & go test -v -short ./internal/fault 2>&1 | findstr /R "PASS|FAIL"
}

Test-Component "Go: internal/orchestrator tests" {
    & go test -v -short ./internal/orchestrator 2>&1 | findstr /R "PASS|FAIL"
}

# ─────────────────────────────────────────────────────────────
# Phase 6: Server Health
# ─────────────────────────────────────────────────────────────

Info ""
Info "Phase 6: Server Health Checks (Start servers first!)"
Info "─────────────────────────────────────────────────────────────"

# Start test server in background
Info ""
Info "Starting test web server (port 3001)..."
$server = Start-Process -FilePath node -ArgumentList "test/test-server.js" -PassThru -NoNewWindow
Start-Sleep -Seconds 2

Test-Component "Test server /health endpoint" {
    $resp = Invoke-WebRequest -Uri "http://127.0.0.1:3001/health" -TimeoutSec 5 -ErrorAction Stop
    $data = $resp.Content | ConvertFrom-Json
    if ($data.status -ne "ok") { throw "Status not ok" }
}

Test-Component "Test server /api/fast endpoint" {
    $resp = Invoke-WebRequest -Uri "http://127.0.0.1:3001/api/fast" -TimeoutSec 5 -ErrorAction Stop
    $data = $resp.Content | ConvertFrom-Json
    if (-not $data.message) { throw "No message in response" }
}

Test-Component "Test server main page load" {
    $resp = Invoke-WebRequest -Uri "http://127.0.0.1:3001/" -TimeoutSec 5 -ErrorAction Stop
    if ($resp.StatusCode -ne 200) { throw "Page not available" }
}

# Stop test server
Stop-Process -Id $server.Id -Force

# ─────────────────────────────────────────────────────────────
# Phase 7: Summary
# ─────────────────────────────────────────────────────────────

Info ""
Info "╔════════════════════════════════════════════════════════════╗"

if ($testsFailed -eq 0) {
    Success "║ ALL TESTS PASSED ✓"
    Success "║ Total: $($testsPassed + $testsFailed) tests"
} else {
    Error "║ SOME TESTS FAILED ✗"
    Error "║ Passed: $testsPassed | Failed: $testsFailed"
}

Info "╚════════════════════════════════════════════════════════════╝"

if ($testsFailed -gt 0) {
    exit 1
}
