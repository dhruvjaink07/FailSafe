#!/usr/bin/env bash
# Quick Integration Test Guide
# Copy & paste commands below to test FailSafe

echo "═══════════════════════════════════════════════════════════"
echo "FailSafe Integration Testing - Quick Reference"
echo "═══════════════════════════════════════════════════════════"
echo ""

# 1. Prerequisites
echo "Step 1: Verify Prerequisites"
echo "─────────────────────────────"
echo "$ node --version          # should be v18+"
echo "$ go version              # should be v1.22+"
echo "$ npm --version           # should be v9+"
echo ""

# 2. Install dependencies
echo "Step 2: Install Dependencies"
echo "─────────────────────────────"
echo "$ npm install"
echo ""

# 3. Build controller
echo "Step 3: Build Controller"
echo "─────────────────────────"
echo "$ go build -o controller.exe ./cmd/controller"
echo ""

# 4. Run full suite
echo "Step 4: Run Full Integration Test Suite (PowerShell)"
echo "─────────────────────────────────────────────────────"
echo "$ Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process"
echo "$ .\test\integration-test.ps1"
echo ""

# 5. Or run manually
echo "Step 5 (Alternative): Run Manually in Separate Terminals"
echo "──────────────────────────────────────────────────────────"
echo ""
echo "Terminal 1 - Test Web Server:"
echo "$ node test/test-server.js"
echo ""
echo "Terminal 2 - Go Controller:"
echo "$ go run ./cmd/controller"
echo ""
echo "Terminal 3 - Playwright Automation:"
echo "$ node test/playwright-integration.js"
echo ""

# 6. Run just Go tests
echo "Step 6: Run Go Unit Tests Only"
echo "───────────────────────────────"
echo "$ go test -v -short ./cmd/controller ./internal/orchestrator"
echo ""

# 7. Run just Playwright test
echo "Step 7: Run Playwright Test Only"
echo "─────────────────────────────────"
echo "$ node test/playwright-integration.js"
echo ""

# 8. Verify endpoints
echo "Step 8: Verify Endpoints (after servers running)"
echo "────────────────────────────────────────────────"
echo "$ curl http://127.0.0.1:3001/health           # test server"
echo "$ curl http://127.0.0.1:8000/health           # go controller"
echo ""

# 9. Load test scenarios
echo "Step 9: Test Scenario Loading (Node.js REPL)"
echo "──────────────────────────────────────────────"
echo "$ node"
echo "> const Loader = require('./internal/frontend/chaos/scenarios');"
echo "> const l = new Loader();"
echo "> await l.load('latency').then(s => console.log(s.chaos));"
echo ""

# 10. Cleanup
echo "Step 10: Cleanup"
echo "───────────────"
echo "$ Get-Process node | Stop-Process -Force"
echo "$ Get-Process controller | Stop-Process -Force"
echo ""

echo "═══════════════════════════════════════════════════════════"
echo "For detailed documentation, see:"
echo "  • docs/integration-testing.md"
echo "  • INTEGRATION_TESTING_STATUS.md"
echo "═══════════════════════════════════════════════════════════"
