# FailSafe Backend Testing Guide

## Quick Start

### 1. Ensure Backend is Running

```powershell
curl.exe http://localhost:8000/health
```

### 2. Create API Key (one-time for testing)

```powershell
$keyPayload = @{
    environment = "dev"
    role = "admin"
    name = "test-key"
}

Invoke-WebRequest -Uri "http://localhost:8000/internal/api-keys/create" `
  -Method Post `
  -ContentType "application/json" `
  -Body ($keyPayload | ConvertTo-Json) `
  -UseBasicParsing
```

### 3. List Available Containers via API

```powershell
$apiKey = "<YOUR_API_KEY>"

Invoke-WebRequest -Uri "http://localhost:8000/environment/containers" `
  -Method Get `
  -Headers @{ "X-API-Key" = $apiKey } `
  -UseBasicParsing | Select-Object -ExpandProperty Content
```

### 4. Wake a Selected Container via API

```powershell
$apiKey = "<YOUR_API_KEY>"

Invoke-WebRequest -Uri "http://localhost:8000/environment/containers/start?name=failsafe-postgres" `
  -Method Post `
  -Headers @{ "X-API-Key" = $apiKey } `
  -UseBasicParsing
```

---

## Environment Setup

### Start Services

```powershell
# Terminal 1: Android Executor (optional but recommended)
cmd\android-executor\android-executor.exe

# Terminal 2: Docker Compose
docker-compose up

# Terminal 3: Testing
curl.exe http://localhost:8000/health
```

### Verify Everything is Running

```powershell
# Check Docker
docker ps

# Check backend health
curl.exe http://localhost:8000/health

# Check executor (if running)
curl.exe http://localhost:9090/health
```

---

## Testing Workflows

### Workflow 1: APK Upload Test

```powershell
# 1. Ensure services are running
curl.exe http://localhost:8000/health

# 2. List containers available in environment
$apiKey = "<YOUR_API_KEY>"
Invoke-WebRequest -Uri "http://localhost:8000/environment/containers" `
  -Method Get `
  -Headers @{ "X-API-Key" = $apiKey } `
  -UseBasicParsing | Select-Object -ExpandProperty Content

# 3. Upload APK
curl.exe -X POST -F "file=@app.apk" http://localhost:8000/upload/apk

# Expected: Returns APK metadata with package/activity
```

### Workflow 2: Create Experiment

```powershell
# 1. Create API key
$keyPayload = @{
    environment = "dev"
    role = "admin"
    name = "test-key"
}
$keyPayload | Out-File -FilePath "key-payload.json" -Encoding UTF8 -NoBOM

curl.exe -X POST `
  -H "Content-Type: application/json" `
  -d "@key-payload.json" `
  http://localhost:8000/internal/api-keys/create

# Save the returned API key
$apiKey = "fs_dev_admin_..."

# 2. Upload APK (to get APK ID)
curl.exe -X POST -F "file=@app.apk" http://localhost:8000/upload/apk
# Save the returned ID

# 3. Create experiment
$payload = @{
    fault_type = "kill_app"
    targets = @("com.example.code")
    target_type = "android"
    observation_type = "android"
    duration_seconds = 70
    apk = "YOUR-APK-ID"
    android_run = @{
        avd_name = "Pixel_8a"
        headless = $true
        reset_app_state = $false
    }
    scenarios = @(
        @{ type = "network_disable"; at = 10; duration_seconds = 6 }
        @{ type = "network_enable"; at = 8; duration_seconds = 1 }
        @{ type = "kill_app"; at = 20; duration_seconds = 1 }
        @{ type = "foreground_app"; at = 20; duration_seconds = 2 }
    )
    expected = @{
        running = $true
        not_crash = $true
        not_anr = $true
        should_recover = $true
    }
}

Invoke-WebRequest -Uri "http://localhost:8000/experiments/android/start" `
  -Method Post `
  -ContentType "application/json" `
  -Headers @{ "X-API-Key" = $apiKey } `
  -Body ($payload | ConvertTo-Json -Depth 10) `
  -UseBasicParsing
```

### Workflow 3: View Experiment Status

```powershell
$apiKey = "fs_dev_admin_..."
$experimentId = "YOUR-EXPERIMENT-ID"

Invoke-WebRequest -Uri "http://localhost:8000/experiments/$experimentId/status" `
  -Method Get `
  -Headers @{ "X-API-Key" = $apiKey } `
  -UseBasicParsing | Select-Object -ExpandProperty Content | ConvertFrom-Json | ConvertTo-Json
```

---

## Container Inspection

### View Backend Logs

```powershell
# Real-time logs
docker logs -f failsafe-backend

# Last 100 lines
docker logs --tail 100 failsafe-backend

# Logs from last 5 minutes
docker logs --since 5m failsafe-backend
```

### View PostgreSQL Logs

```powershell
# Real-time logs
docker logs -f failsafe-postgres

# Check if database is accessible
docker exec failsafe-postgres psql -U failsafe -d failsafe -c "SELECT 1"
```

### Connect to PostgreSQL

```powershell
# Interactive psql shell
docker exec -it failsafe-postgres psql -U failsafe -d failsafe

# Common queries
# List tables: \dt
# List databases: \l
# Exit: \q
```

### Inspect Container

```powershell
# Environment variables
docker inspect failsafe-backend --format='{{json .Config.Env}}' | ConvertFrom-Json

# Port mappings
docker inspect failsafe-backend --format='{{json .NetworkSettings.Ports}}'

# Full config
docker inspect failsafe-backend
```

---

## Troubleshooting

### Docker Won't Start

```powershell
# Check if Docker Desktop service is enabled
Get-Service Docker

# Try to start Docker Desktop manually
& "C:\Program Files\Docker\Docker\Docker Desktop.exe"

# Wait 30 seconds and check again
./scripts/docker-helper.ps1 -Status
```

### Container Fails to Start

```powershell
# View full error logs
docker logs failsafe-backend

# Check resource limits
docker stats failsafe-backend

# Restart container
docker restart failsafe-backend

# Or rebuild
docker-compose down
docker-compose up --build
```

### Backend Returns 500 Error

```powershell
# View detailed logs
docker logs failsafe-backend | Select-String -Pattern "error|Error|ERROR" -Context 5

# Check database connection
docker exec failsafe-backend curl http://localhost:8000/health

# Verify environment variables
docker inspect failsafe-backend --format='{{json .Config.Env}}'
```

### APK Upload Fails

```powershell
# Check if executor is needed
curl.exe http://localhost:9090/health

# If executor not running, backend should fall back to local aapt
# Check backend logs for fallback messages
docker logs failsafe-backend | Select-String -Pattern "fallback|executor"

# Verify APK file is valid
aapt dump badging "your-file.apk"
```

---

## Performance Testing

### Load Test Endpoints

```powershell
# Simple health check load test (100 requests)
$result = @()
1..100 | ForEach-Object {
    $start = Get-Date
    $response = curl.exe http://localhost:8000/health 2>&1
    $elapsed = ((Get-Date) - $start).TotalMilliseconds
    $result += $elapsed
    Write-Progress -Activity "Load Testing" -Status "Request $_" -PercentComplete ($_ / 100 * 100)
}

# Calculate stats
$result | Measure-Object -Average -Minimum -Maximum | Format-Table -AutoSize

# Expected: ~50-200ms per request locally
```

### Database Performance

```powershell
# Check query performance
docker exec failsafe-postgres psql -U failsafe -d failsafe -c "SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname NOT LIKE 'pg_%'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;"
```

---

## Cleanup

### Stop Services

```powershell
# Stop containers (keep data)
docker-compose stop

# Stop everything
./scripts/docker-helper.ps1 -Interactive
# Select option 4: Stop containers
```

### Clean Up

```powershell
# Remove stopped containers
docker container prune

# Remove unused images
docker image prune

# Full reset (removes data)
docker-compose down -v

# Rebuild from scratch
docker-compose build --no-cache
docker-compose up
```

---

## Environment Variables for Testing

Set these before starting containers:

```powershell
# Android Executor URL
$env:ANDROID_EXECUTOR_URL = "http://host.docker.internal:9090"

# APK Upload Directory
$env:APK_UPLOAD_DIR = "uploads/apks"

# Database
$env:DB_HOST = "failsafe-postgres"
$env:DB_PORT = "5432"
$env:DB_USER = "failsafe"
$env:DB_PASSWORD = "failsafe"
$env:DB_NAME = "failsafe"

# Start services
docker-compose up
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Backend Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_USER: failsafe
          POSTGRES_PASSWORD: failsafe
          POSTGRES_DB: failsafe
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run tests
        run: go test ./internal/handlers ./internal/orchestrator ./internal/storage -v
      
      - name: Build
        run: go build -v ./cmd/controller
```

---

## Documentation References

- [ANDROID_EXECUTOR_ARCHITECTURE.md](ANDROID_EXECUTOR_ARCHITECTURE.md)
- [ANDROID_EXECUTOR_QUICKSTART.md](ANDROID_EXECUTOR_QUICKSTART.md)
- [ANDROID_EXECUTOR_TEST_PLAN.md](ANDROID_EXECUTOR_TEST_PLAN.md)
- [API_DOCS.md](../API_DOCS.md)
- [INTEGRATION_TESTING_STATUS.md](../INTEGRATION_TESTING_STATUS.md)
