#!/usr/bin/env pwsh
<#
.SYNOPSIS
  FailSafe Integration Test Script
  
  Tests the complete pipeline:
  1. Start test web server
  2. Start Go controller
  3. Run Playwright automation with chaos injection
  4. Verify metrics collection
  5. Cleanup
#>

$ErrorActionPreference = "Stop"

$TEST_SERVER_PORT = 3001
$CONTROLLER_PORT = 8000
$BASE_URL = "http://127.0.0.1:$TEST_SERVER_PORT"
$METRICS_ENDPOINT = "http://127.0.0.1:$CONTROLLER_PORT/frontend/metrics"
$EXPERIMENT_ID = "integration-test-$(Get-Date -Format 'yyyyMMddHHmmss')"

Write-Host "[integration-test] FailSafe Integration Test Suite" -ForegroundColor Green
Write-Host "[integration-test] Experiment ID: $EXPERIMENT_ID"
Write-Host "[integration-test] Base URL: $BASE_URL"
Write-Host "[integration-test] Metrics Endpoint: $METRICS_ENDPOINT"
Write-Host ""

# Cleanup function
function Cleanup {
    Write-Host "[integration-test] Cleaning up..."
    
    # Kill processes
    Get-Process node -ErrorAction SilentlyContinue | Where-Object { $_.Handle -gt 0 } | Stop-Process -Force -ErrorAction SilentlyContinue
    Get-Process controller -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    
    Start-Sleep -Milliseconds 500
}

try {
    # 1. Start test web server
    Write-Host "[integration-test] Starting test web server..." -ForegroundColor Cyan
    $server = Start-Process -FilePath node -ArgumentList "test/test-server.js" `
        -EnvironmentVariables @{TEST_SERVER_PORT=$TEST_SERVER_PORT} `
        -PassThru -NoNewWindow
    
    Write-Host "[integration-test] Test server started (PID: $($server.Id))"
    Start-Sleep -Seconds 2
    
    # 2. Start Go controller
    Write-Host "[integration-test] Starting Go controller..." -ForegroundColor Cyan
    # Note: Controller must be built first: go build -o controller.exe ./cmd/controller
    if (-not (Test-Path ".\controller.exe")) {
        Write-Host "[integration-test] Building controller..." -ForegroundColor Yellow
        & go build -o controller.exe ./cmd/controller
    }
    
    $controller = Start-Process -FilePath ".\controller.exe" `
        -EnvironmentVariables @{PORT=$CONTROLLER_PORT} `
        -PassThru -NoNewWindow
    
    Write-Host "[integration-test] Controller started (PID: $($controller.Id))"
    Start-Sleep -Seconds 3
    
    # 3. Health checks
    Write-Host "[integration-test] Running health checks..." -ForegroundColor Cyan
    
    # Check web server
    try {
        $webHealth = Invoke-WebRequest -Uri "$BASE_URL/health" -TimeoutSec 5 -ErrorAction Stop
        Write-Host "[integration-test] ✓ Web server healthy"
    } catch {
        Write-Host "[integration-test] ✗ Web server health check failed: $_" -ForegroundColor Red
        exit 1
    }
    
    # Check controller
    try {
        $ctrlHealth = Invoke-WebRequest -Uri "http://127.0.0.1:$CONTROLLER_PORT/health" -TimeoutSec 5 -ErrorAction Stop
        Write-Host "[integration-test] ✓ Controller healthy"
    } catch {
        Write-Host "[integration-test] ✗ Controller health check failed: $_" -ForegroundColor Red
        exit 1
    }
    
    # 4. Run Playwright integration test
    Write-Host "[integration-test] Running Playwright integration test..." -ForegroundColor Cyan
    $env:BASE_URL = $BASE_URL
    $env:METRICS_ENDPOINT = $METRICS_ENDPOINT
    $env:EXPERIMENT_ID = $EXPERIMENT_ID
    
    & node test/playwright-integration.js
    $playwrightExitCode = $LASTEXITCODE
    
    if ($playwrightExitCode -ne 0) {
        Write-Host "[integration-test] ✗ Playwright test failed (exit code: $playwrightExitCode)" -ForegroundColor Red
        exit 1
    }
    
    Write-Host "[integration-test] ✓ Playwright test passed" -ForegroundColor Green
    
    # 5. Run Go tests
    Write-Host "[integration-test] Running Go tests..." -ForegroundColor Cyan
    & go test -v -short ./cmd/controller ./internal/orchestrator ./internal/monitoring ./internal/fault
    $goExitCode = $LASTEXITCODE
    
    if ($goExitCode -ne 0) {
        Write-Host "[integration-test] ✗ Go tests failed (exit code: $goExitCode)" -ForegroundColor Red
        exit 1
    }
    
    Write-Host "[integration-test] ✓ Go tests passed" -ForegroundColor Green
    
    # 6. Summary
    Write-Host ""
    Write-Host "[integration-test] ╔════════════════════════════════════════╗" -ForegroundColor Green
    Write-Host "[integration-test] ║ Integration Test Suite PASSED ✓ ║" -ForegroundColor Green
    Write-Host "[integration-test] ╚════════════════════════════════════════╝" -ForegroundColor Green
    Write-Host ""
    Write-Host "[integration-test] Test Results:"
    Write-Host "  • Web Server:        ✓ Running"
    Write-Host "  • Go Controller:     ✓ Running"
    Write-Host "  • Playwright:        ✓ Passed"
    Write-Host "  • Go Orchestrator:   ✓ Passed"
    Write-Host "  • Experiments:       ✓ Lifecycle verified"
    Write-Host ""
    
} catch {
    Write-Host "[integration-test] ✗ Fatal error: $_" -ForegroundColor Red
    exit 1
} finally {
    Cleanup
}
