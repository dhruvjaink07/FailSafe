# Android Executor Quick Start

This guide helps you get the FailSafe system running with the host-based Android executor architecture.

## 5-Minute Setup

### Step 1: Start Android Executor Service (Host)

**Terminal 1 - Host Machine:**
```powershell
# Navigate to FailSafe root
cd d:\FailSafe

# Build and run executor service
go build -o cmd\android-executor\android-executor.exe ./cmd/android-executor
cmd\android-executor\android-executor.exe
```

Expected output:
```
Android executor listening on :9090
```

### Step 2: Start Backend in Docker

**Terminal 2:**
```bash
# From FailSafe root
docker-compose up
```

Expected output:
```
failsafe-backend-1       | Server running on :8080
failsafe-postgres-1      | database system is ready to accept connections
```

### Step 3: Verify It Works

**Terminal 3 - Test the system:**

```bash
# Check health
curl http://localhost:8080/health

# Check executor is reachable
curl http://localhost:9090/health

# Upload a test APK
curl -X POST -F "file=@path/to/app.apk" http://localhost:8080/upload/apk
```

## Configuration

### 1. Executor URL

By default, backend looks for executor at `http://host.docker.internal:9090`

To override:
```bash
# Before running docker-compose
export ANDROID_EXECUTOR_URL=http://your-host:9090
docker-compose up
```

### 2. APK Upload Directory

```bash
# Store uploads in custom location
export APK_UPLOAD_DIR=/data/apks
```

### 3. AAPT Tool (Fallback)

Ensure Android SDK is on your PATH or set explicitly:

```powershell
# Windows - set before running executor
$env:ANDROID_SDK_ROOT = "C:\Android\SDK"
cmd\android-executor\android-executor.exe

# Or set AAPT_PATH directly
$env:AAPT_PATH = "C:\Android\SDK\build-tools\34.0.0\aapt.exe"
cmd\android-executor\android-executor.exe
```

## Testing Workflow

### 1. Unit Tests
```bash
go test ./internal/handlers ./internal/orchestrator ./internal/storage -v
```

### 2. Integration Test - APK Upload

Once system is running:

```bash
# Create a minimal test APK or use an existing one
# Then upload it:

curl -X POST \
  -F "file=@E:\MyApp.apk" \
  http://localhost:8080/upload/apk \
  -s | jq

# Expected response:
# {
#   "id": "550e8400-e29b-41d4-a716-446655440000",
#   "apk": "550e8400-e29b-41d4-a716-446655440000",
#   "package": "com.example.myapp",
#   "activity": "com.example.MainActivity",
#   "path": "uploads/apks/550e8400-e29b-41d4-a716-446655440000.apk"
# }
```

### 3. Create Android Experiment

Using the uploaded APK ID:

```bash
curl -X POST http://localhost:8080/experiments/android \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "name": "Test Android Experiment",
    "target": "emulator",
    "platform": "android",
    "apk": "550e8400-e29b-41d4-a716-446655440000",
    "faults": [
      {
        "type": "network_disable",
        "trigger": "request"
      }
    ],
    "duration": 60
  }' | jq
```

## Troubleshooting

### Executor not responding

```bash
# Check if service is running
curl http://localhost:9090/health

# If not found, start it:
cmd\android-executor\android-executor.exe

# Check for port conflicts
netstat -ano | findstr :9090
```

### APK upload fails with AAPT error

```bash
# Verify AAPT tool exists
where aapt
# or
$env:ANDROID_SDK_ROOT\build-tools\<version>\aapt.exe dump badging "c:\path\to\app.apk"

# If no Android SDK, install it:
# https://developer.android.com/studio/command-line-tools

# After installation, set path:
$env:ANDROID_HOME = "C:\Users\YourName\AppData\Local\Android\Sdk"
```

### Docker can't reach executor

```bash
# Verify host.docker.internal works from container:
docker run -it golang:latest curl http://host.docker.internal:9090/health

# If that fails, use machine IP instead:
$env:ANDROID_EXECUTOR_URL = "http://192.168.1.100:9090"
docker-compose up
```

### Database connection issues

```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Reset database
docker-compose down -v  # removes volumes
docker-compose up
```

## Common Commands

### Build Everything
```bash
# Executor
go build -o cmd\android-executor\android-executor.exe ./cmd/android-executor

# Controller
go build -o cmd\controller\controller.exe ./cmd/controller

# All tests
go test ./...
```

### Run Tests with Coverage
```bash
go test -cover ./internal/handlers ./internal/orchestrator ./internal/storage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Clean Up
```bash
# Stop containers and remove data
docker-compose down -v

# Remove uploaded APKs
Remove-Item -Path "uploads/*" -Recurse -Force -ErrorAction SilentlyContinue

# Kill executor if still running
# (Close terminal or use Ctrl+C)
```

### View Logs

```bash
# Backend container logs
docker-compose logs -f failsafe-backend

# Database logs
docker-compose logs -f failsafe-postgres

# Both
docker-compose logs -f

# Executor logs (in its terminal)
# Just look at Terminal 1
```

## Next Steps

1. **Create API Keys** - Set up authentication for your experiments
2. **Upload Test APKs** - Use the APK upload endpoint
3. **Create Experiments** - Define fault injection scenarios
4. **Run Smoke Tests** - Verify metrics collection and restart resilience
5. **Production Deployment** - See [DEPLOYMENT.md](./DEPLOYMENT.md)

## Docker Utilities

Manage containers easily with the Docker helper script:

```powershell
# Check Docker status and list containers
./scripts/docker-helper.ps1

# Ensure Docker is running (starts if needed)
./scripts/docker-helper.ps1 -Start

# Interactive container management
./scripts/docker-helper.ps1 -Interactive
```

See [BACKEND_TESTING_GUIDE.md](./BACKEND_TESTING_GUIDE.md) for comprehensive testing workflows and container management.

## Architecture Reference

For detailed architecture information, see:
- [BACKEND_TESTING_GUIDE.md](./BACKEND_TESTING_GUIDE.md) - Complete testing workflows
- [ANDROID_EXECUTOR_ARCHITECTURE.md](./ANDROID_EXECUTOR_ARCHITECTURE.md) - System design
- [ANDROID_EXECUTOR_TEST_PLAN.md](./ANDROID_EXECUTOR_TEST_PLAN.md) - Test procedures
- [API_DOCS.md](../API_DOCS.md) - API reference
- [INTEGRATION_TESTING_STATUS.md](../INTEGRATION_TESTING_STATUS.md) - Integration status
