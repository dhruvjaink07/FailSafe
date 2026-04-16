# Android Executor Integration - Test Plan

## Objective
Validate that the host-based Android executor architecture works correctly with:
1. Remote APK metadata extraction via HTTP
2. Fallback to local aapt when executor unavailable
3. Full Docker integration
4. APK upload and experiment creation workflows

## Test Environment Setup

### Prerequisites
- Windows machine with:
  - Go 1.21+
  - Docker Desktop
  - Android SDK (with aapt available)
  - A valid APK file for testing
- Network connectivity verified (Docker Desktop running)

### Initial Setup
```powershell
# Set working directory
cd d:\FailSafe

# Verify Go toolchain
go version

# Verify Docker
docker --version
docker-compose --version

# Verify Android SDK
$env:ANDROID_SDK_ROOT = "C:\Android\SDK"  # adjust to your path
aapt version
```

## Test Suite

### Test 1: Executor Service Standalone

**Objective:** Verify executor service runs and responds to requests

**Steps:**
```powershell
# Terminal 1: Build and run executor
cd d:\FailSafe
go build -o cmd\android-executor\android-executor.exe ./cmd/android-executor
cmd\android-executor\android-executor.exe
# Expected: "Android executor listening on :9090"

# Terminal 2: Test health endpoint
curl http://localhost:9090/health
# Expected: {"status": "ok"}

# Terminal 2: Test with valid APK (adjust path)
curl "http://localhost:9090/aapt?apk=C:\path\to\test.apk"
# Expected: {"package": "com.example.app", "activity": "com.example.MainActivity", "output": "...", "success": true}

# Terminal 2: Test with invalid path
curl "http://localhost:9090/aapt?apk=C:\nonexistent\app.apk"
# Expected: {"success": false, "error": "apk file not found: C:\\nonexistent\\app.apk"}
```

**Success Criteria:**
- ✅ Service starts on port 9090
- ✅ Health endpoint returns 200 + JSON
- ✅ Valid APK returns package/activity
- ✅ Invalid APK returns error with appropriate message

### Test 2: Executor Client Standalone

**Objective:** Verify Go client can connect to and call executor

**Steps:**
```powershell
# Terminal 1: Keep executor running from Test 1

# Terminal 2: Run a minimal Go test
cat > test_executor_client.go << 'EOF'
package main

import (
	"fmt"
	"d:\FailSafe\internal\handlers"
)

func main() {
	client := handlers.NewAndroidExecutorClient("http://localhost:9090")
	
	// Test health check
	if client.IsAvailable() {
		fmt.Println("✓ Executor is available")
	} else {
		fmt.Println("✗ Executor is NOT available")
		return
	}
	
	// Test AAPT call (adjust path for your test APK)
	pkg, activity, err := client.CallAAPT("C:\\path\\to\\test.apk")
	if err != nil {
		fmt.Printf("✗ AAPT call failed: %v\n", err)
		return
	}
	
	fmt.Printf("✓ Package: %s\n", pkg)
	fmt.Printf("✓ Activity: %s\n", activity)
}
EOF

go run test_executor_client.go
# Expected: ✓ messages with package and activity
```

**Success Criteria:**
- ✅ Client successfully connects to executor
- ✅ Health check returns true
- ✅ AAPT call returns correct package/activity
- ✅ Errors are properly propagated

### Test 3: APK Upload to Local Backend

**Objective:** Verify APK upload handler can extract metadata

**Steps:**
```powershell
# Terminal 1: Keep executor running

# Terminal 2: Start local backend (NOT in Docker)
cd d:\FailSafe
$env:ANDROID_EXECUTOR_URL = "http://localhost:9090"
go run ./cmd/controller/main.go
# Expected: "Server running on :8080"

# Terminal 3: Upload APK (adjust file path)
$ApkPath = "C:\path\to\test.apk"
curl -X POST `
  -F "file=@$ApkPath" `
  http://localhost:8080/upload/apk | ConvertFrom-Json | ConvertTo-Json -Depth 10

# Expected:
# {
#   "id": "550e8400-e29b-41d4-a716-446655440000",
#   "apk": "550e8400-e29b-41d4-a716-446655440000",
#   "package": "com.example.myapp",
#   "activity": "com.example.MainActivity",
#   "path": "uploads/apks/550e8400-e29b-41d4-a716-446655440000.apk"
# }
```

**Success Criteria:**
- ✅ APK file is saved to disk
- ✅ Upload endpoint returns 200 + metadata JSON
- ✅ Returned ID is a valid UUID
- ✅ Package and activity are extracted correctly
- ✅ Package and activity match APK contents

**Save APK ID for later tests:**
```powershell
$ApkId = "550e8400-e29b-41d4-a716-446655440000"
```

### Test 4: APK Upload with Executor Unavailable (Fallback)

**Objective:** Verify fallback to local aapt works when executor down

**Steps:**
```powershell
# Terminal 1: Stop executor (Ctrl+C or close terminal)

# Terminal 2: Keep backend running

# Terminal 3: Try uploading APK again
$ApkPath = "C:\path\to\test.apk"
curl -X POST `
  -F "file=@$ApkPath" `
  http://localhost:8080/upload/apk | ConvertFrom-Json | ConvertTo-Json -Depth 10

# Expected: Same success as Test 3, but using fallback local aapt
# In backend console, should see:
# "remote executor failed, falling back to local aapt: ..."
```

**Success Criteria:**
- ✅ Upload still succeeds despite executor being down
- ✅ Metadata is correctly extracted from local aapt
- ✅ Response has same structure as Test 3
- ✅ Error message appears in backend logs

### Test 5: Docker Integration

**Objective:** Verify APK upload works when backend is in Docker

**Steps:**
```powershell
# Terminal 1: Start executor on host (if not already running)
cd d:\FailSafe
cmd\android-executor\android-executor.exe

# Terminal 2: Start backend in Docker
cd d:\FailSafe
docker compose -f deployments/docker/docker-compose.yml up
# Expected: "failsafe-backend-1 | Server running on :8080"

# Terminal 3: Verify Docker can reach executor
docker run -it --rm golang:latest curl http://host.docker.internal:9090/health
# Expected: {"status":"ok"}

# Terminal 3: Upload APK to Docker backend
$ApkPath = "C:\path\to\test.apk"
curl -X POST `
  -F "file=@$ApkPath" `
  http://localhost:8080/upload/apk | ConvertFrom-Json | ConvertTo-Json -Depth 10

# Expected: Same success as Test 3
# Check Docker logs - should show executor communication
```

**Success Criteria:**
- ✅ Docker container can reach executor at host.docker.internal:9090
- ✅ APK upload to Docker backend succeeds
- ✅ Metadata is extracted correctly via remote executor
- ✅ Response structure matches previous tests
- ✅ Docker logs show successful remote calls (no local fallback)

### Test 6: Experiment Creation with Uploaded APK

**Objective:** Verify full workflow from APK upload to experiment creation

**Prerequisites:**
- APK ID from Test 5: `$ApkId = "..."`
- Backend running in Docker (docker-compose up)
- Executor running on host

**Steps:**
```powershell
# Terminal 3: Create API key (or reuse existing one)
$ApiKey = "test-key-" + [guid]::NewGuid().ToString().Substring(0,8)

# Create experiment using uploaded APK ID
$ExperimentPayload = @{
    name = "Test Android Fault Injection"
    platform = "android"
    target = "emulator"
    apk = $ApkId
    faults = @(
        @{
            type = "network_disable"
            trigger = "request"
        }
    )
    duration = 30
} | ConvertTo-Json

curl -X POST `
  -H "Content-Type: application/json" `
  -H "X-API-Key: $ApiKey" `
  -d $ExperimentPayload `
  http://localhost:8080/experiments/android | ConvertFrom-Json | ConvertTo-Json -Depth 10

# Expected:
# {
#   "id": "experiment-uuid",
#   "name": "Test Android Fault Injection",
#   "platform": "android",
#   "apk": {
#     "id": "550e8400-..., 
#     "package": "com.example.myapp",
#     "activity": "com.example.MainActivity",
#     ...
#   },
#   "status": "created",
#   ...
# }
```

**Success Criteria:**
- ✅ POST /experiments/android succeeds with 201/200
- ✅ Experiment includes full APK metadata
- ✅ Experiment includes all faults
- ✅ Experiment receives unique ID
- ✅ Experiment status is "created"

### Test 7: Environment Variable Override

**Objective:** Verify ANDROID_EXECUTOR_URL environment variable works

**Steps:**
```powershell
# Start executor on non-default port (9091 instead of 9090)
# This requires modifying cmd/android-executor/main.go temporarily
# OR use a proxy/port forward for this test

# For now, test with explicit URL:
$env:ANDROID_EXECUTOR_URL = "http://nonexistent:9999"

# Start backend (will try to use override)
go run ./cmd/controller/main.go

# Upload APK - should fail trying to reach nonexistent executor
# Then try fallback
# Close backend
```

**Success Criteria:**
- ✅ Environment variable is read correctly
- ✅ Backend attempts to use custom URL
- ✅ Falls back to local aapt when custom URL fails

### Test 8: Restart Resilience

**Objective:** Verify DB persistence survives restart with Android executor

**Steps:**
```powershell
# From Test 6: Note the experiment ID
$ExperimentId = "experiment-uuid-from-test-6"

# Terminal 3: Get experiment status before restart
curl http://localhost:8080/experiments/$ExperimentId/status | ConvertFrom-Json | ConvertTo-Json -Depth 10

# Terminal 2: Restart backend (Ctrl+C)
docker compose -f deployments/docker/docker-compose.yml down

# Wait 10 seconds
Start-Sleep -Seconds 10

# Terminal 2: Restart backend
docker compose -f deployments/docker/docker-compose.yml up

# Wait for backend to start (20 seconds)
Start-Sleep -Seconds 20

# Terminal 3: Get experiment status after restart
curl http://localhost:8080/experiments/$ExperimentId/status | ConvertFrom-Json | ConvertTo-Json -Depth 10

# Expected: Same experiment data before and after restart
# Status should be retrievable from database
```

**Success Criteria:**
- ✅ Experiment persists in database after restart
- ✅ Status endpoint returns same data pre/post restart
- ✅ APK metadata is available from DB reads
- ✅ No executor calls needed if data in DB

## Failure Scenarios

### Scenario A: Executor Crashes During Upload

**Setup:**
1. Executor running
2. Start large APK upload (create slow network)
3. Kill executor during upload

**Expected:** Backend falls back to local aapt, upload completes

### Scenario B: Corrupt APK File

**Setup:**
1. Create empty file with .apk extension
2. Upload to backend

**Expected:** 413/400 error with message about aapt failure

### Scenario C: Docker Network Isolation

**Setup:**
1. Docker-compose up
2. Firewall block port 9090
3. Upload APK

**Expected:** Falls back to local aapt (within container)

## Regression Tests

Run these to ensure nothing broke:

```powershell
# All unit tests
go test ./internal/handlers ./internal/orchestrator ./internal/storage -v

# Specific test suite
go test -run TestUploadAPK ./internal/handlers -v

# Platform endpoints
go test -run PlatformEndpoints ./internal/handlers -v

# API key auth
go test -run Auth ./internal/handlers -v
```

## Performance Benchmarks

**Baseline measurements for optimization:**

```powershell
# APK upload time (local)
Measure-Command { curl -X POST -F "file=@C:\test.apk" http://localhost:8080/upload/apk } | select TotalMilliseconds

# APK upload time (Docker)
Measure-Command { curl -X POST -F "file=@C:\test.apk" http://localhost:8080/upload/apk } | select TotalMilliseconds

# Executor latency
Measure-Command { curl "http://localhost:9090/aapt?apk=C:\path\to\test.apk" } | select TotalMilliseconds

# Should be:
# - Local: < 1000ms
# - Docker: < 1500ms (includes network overhead)
# - Executor: < 500ms
```

## Cleanup

After all tests:

```powershell
# Stop containers
docker compose -f deployments/docker/docker-compose.yml down -v

# Remove test artifacts
rm -r uploads/

# Remove test go file
rm test_executor_client.go

# Stop any remaining executor processes
# (should auto-stop if you closed the terminal)
```

## Test Report Template

Use this to document results:

```markdown
# Android Executor Integration Test Report

**Date:** [YYYY-MM-DD]
**Environment:** Windows | Docker Desktop | Go 1.21+

| Test | Status | Notes |
|------|--------|-------|
| Test 1: Executor Service Standalone | ✓ PASS / ✗ FAIL | |
| Test 2: Executor Client Standalone | ✓ PASS / ✗ FAIL | |
| Test 3: APK Upload to Local Backend | ✓ PASS / ✗ FAIL | |
| Test 4: APK Upload Fallback | ✓ PASS / ✗ FAIL | |
| Test 5: Docker Integration | ✓ PASS / ✗ FAIL | |
| Test 6: Experiment Creation | ✓ PASS / ✗ FAIL | |
| Test 7: Environment Variable Override | ✓ PASS / ✗ FAIL | |
| Test 8: Restart Resilience | ✓ PASS / ✗ FAIL | |

**Regression Tests:** All passed / Some failed

**Performance Metrics:**
- Local upload: XXXms
- Docker upload: XXXms
- Executor latency: XXXms

**Issues Encountered:** [none / list]

**Conclusion:** Ready for production / Further testing required
```

## Next Steps Upon Completion

1. ✓ If all tests pass:
   - Merge to main branch
   - Update deployment documentation
   - Update README with architecture diagram
   - Tag release version

2. ✗ If tests fail:
   - Document failure in GitHub issue
   - Debug according to troubleshooting section
   - Re-run specific test after fix
   - Repeat until all pass
