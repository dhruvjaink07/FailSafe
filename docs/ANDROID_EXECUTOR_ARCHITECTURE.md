# Android Executor Architecture

## Overview

The FailSafe application uses a **host-based Android executor** architecture to overcome Docker limitations with the Android SDK and emulator. This document describes the architecture, setup, and integration.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                   Docker Container                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         FailSafe Backend Controller                  │   │
│  │  - HTTP API endpoints                                │   │
│  │  - Experiment orchestration                          │   │
│  │  - Metric collection & storage                       │   │
│  │  - Database integration (PostgreSQL)                 │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │                                  │
│                           │ HTTP/REST                        │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Android Executor Client (in handler)                │   │
│  │  - Calls remote executor via host.docker.internal    │   │
│  │  - Fallback to local aapt if available               │   │
│  └──────────────────────────────────────────────────────┘   │
└────────────────────────────┬───────────────────────────────┘
                             │ host.docker.internal:9090
                             ▼
┌─────────────────────────────────────────────────────────────┐
│             Host Machine (Windows/Linux/Mac)                │
│  ┌──────────────────────────────────────────────────────┐   │
│  │    Android Executor Service (:9090)                  │   │
│  │  - AAPT tool wrapper                                 │   │
│  │  - APK metadata extraction                           │   │
│  │  - POST http://localhost:9090/aapt?apk=...           │   │
│  │  - GET http://localhost:9090/health                  │   │
│  └──────────────────────────────────────────────────────┘   │
│                           │                                  │
│                           │ exec("aapt dump badging ...")   │
│                           ▼                                  │
│  ┌──────────────────────────────────────────────────────┐   │
│  │    Android SDK (aapt, emulator, adb)                 │   │
│  │    - AAPT: Extract APK package/activity              │   │
│  │    - Emulator: Run instrumented Android              │   │
│  │    - ADB: Device/emulator communication              │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Components

### 1. Android Executor Service (`cmd/android-executor/main.go`)

A lightweight HTTP service that runs on the host machine and provides access to Android SDK tools.

**Endpoints:**
- `GET /health` - Service health check
- `GET /aapt?apk=<path>` - Extract APK metadata using aapt

**Example Request:**
```bash
GET http://localhost:9090/aapt?apk=/path/to/app.apk
```

**Example Response:**
```json
{
  "package": "com.example.app",
  "activity": "com.example.MainActivity",
  "output": "... full aapt output ...",
  "success": true
}
```

### 2. Android Executor Client (`internal/handlers/android_executor_client.go`)

A Go HTTP client that wraps calls to the remote executor service.

**Key Functions:**
- `NewAndroidExecutorClient(executorURL)` - Create a new client
- `CallAAPT(apkPath)` - Extract APK metadata
- `IsAvailable()` - Check if executor is reachable
- `GetDefaultExecutorURL()` - Get the executor URL from environment

**Environment Variables:**
- `ANDROID_EXECUTOR_URL` - Override default executor URL
- Default: `http://host.docker.internal:9090`

### 3. Updated APK Upload Handler (`internal/handlers/apk_handlers.go`)

The APK upload endpoint now uses the remote executor for metadata extraction.

**Flow:**
1. Receive APK file upload
2. Save to disk in `uploads/apks` directory
3. Create Android executor client
4. Call remote executor to extract package/activity
5. Fallback to local aapt if remote unavailable
6. Store metadata and return to caller

## Setup & Configuration

### Prerequisites

**On Host Machine:**
- Android SDK installed with build-tools
- `aapt` tool accessible (usually in `$ANDROID_SDK_ROOT/build-tools/`)
- Port 9090 available for executor service

**In Docker (Backend):**
- No Android SDK required
- Access to `host.docker.internal` (Docker Desktop feature)

### Installation

#### 1. Start Android Executor Service (Host)

```bash
# Build the executor service
go build -o android-executor.exe ./cmd/android-executor

# Run it (will listen on :9090)
./android-executor.exe
```

Or if you have the executable pre-built:
```bash
cmd\android-executor\android-executor.exe
```

#### 2. Configure Backend

**Option A: Use default URL**
The backend defaults to `http://host.docker.internal:9090`

**Option B: Override with environment variable**
```bash
export ANDROID_EXECUTOR_URL=http://host.docker.internal:9090
# or on Windows
set ANDROID_EXECUTOR_URL=http://host.docker.internal:9090
```

**Option C: For non-Docker development**
```bash
# If running backend locally (not in Docker)
export ANDROID_EXECUTOR_URL=http://localhost:9090
```

#### 3. Start Backend Controller

```bash
# With Docker
docker-compose up

# Or locally for testing
go run ./cmd/controller/main.go
```

## Testing

### Unit Tests

All existing tests pass with the new architecture:

```bash
go test ./internal/handlers -v
```

Tests include:
- APK upload with metadata extraction
- API key authentication
- Experiment creation and lifecycle
- Platform-specific operations

### Integration Testing

#### Manual APK Upload Test

1. Start executor service:
```bash
cmd\android-executor\android-executor.exe
```

2. Start backend (local for this test):
```bash
go run ./cmd/controller/main.go
```

3. Upload an APK:
```bash
curl -X POST -F "file=@app.apk" http://localhost:8080/upload/apk
```

Expected response:
```json
{
  "id": "uuid-string",
  "apk": "uuid-string",
  "package": "com.example.app",
  "activity": "com.example.MainActivity",
  "path": "uploads/apks/uuid-string.apk"
}
```

#### Testing with Docker

1. Start executor on host:
```bash
cmd\android-executor\android-executor.exe
```

2. Start backend in Docker:
```bash
docker-compose up
```

3. Upload APK to backend in container:
```bash
curl -X POST -F "file=@app.apk" http://localhost:8080/upload/apk
```

The backend will call the executor via `host.docker.internal:9090`

### Debugging

#### Check Executor Health

```bash
curl http://localhost:9090/health
# Response: {"status": "ok"}
```

#### Test AAPT Directly

```bash
curl "http://localhost:9090/aapt?apk=/path/to/app.apk"
```

#### Check Connectivity (from Docker)

If APK upload fails, verify Docker can reach the executor:

```bash
# From inside container
docker run -it golang:latest curl http://host.docker.internal:9090/health
```

## Environment Variables Reference

| Variable | Default | Purpose |
|----------|---------|---------|
| `ANDROID_EXECUTOR_URL` | `http://host.docker.internal:9090` | Remote executor service URL |
| `APK_UPLOAD_DIR` | `uploads/apks` | Directory for uploaded APK files |
| `AAPT_PATH` | Auto-detect | Explicit path to aapt tool (local fallback) |
| `ANDROID_SDK_ROOT` | From env | Android SDK root directory (local fallback) |
| `ANDROID_HOME` | From env | Alternative Android home path (local fallback) |

## Fallback Strategy

The APK upload handler uses a **two-tier fallback strategy**:

1. **Primary: Remote Executor**
   - Try to connect to remote executor via `ANDROID_EXECUTOR_URL`
   - If executor is unavailable, fall through to local

2. **Fallback: Local AAPT**
   - Try local `aapt` command
   - Search Android SDK paths if configured
   - Support Windows and Unix variants

This ensures:
- ✅ Works with Docker (via remote executor)
- ✅ Works with local development (fallback to local aapt)
- ✅ Graceful degradation if executor is down
- ✅ No hard requirement on Android SDK in Docker

## Why This Architecture?

### Problem
- Docker containers don't support Android emulator virtualization
- Android SDK tools (aapt, adb) need access to host system resources
- Bundling Android SDK in a Docker image is large and complex

### Solution
- Keep the **control plane** (backend API, orchestration) in Docker
- Move the **execution plane** (Android SDK tools) to the host
- Use HTTP for clean communication between planes
- Use `host.docker.internal` for Docker-to-host connectivity

### Benefits
- 🎯 Clean separation of concerns
- 📦 Smaller Docker images (no Android SDK)
- 🔄 Easier to update Android SDK independently
- 🛠️ Can run executor on different machine if needed
- ℹ️ Frontend/backend/Android all work independently

## Troubleshooting

### APK Upload Returns "Connection Refused"

**Cause:** Android executor service not running

**Fix:**
```bash
# Start the service
cmd\android-executor\android-executor.exe

# OR use explicit URL if running elsewhere
export ANDROID_EXECUTOR_URL=http://your-host:9090
```

### APK Upload Returns "AAPT Failed"

**Cause:** AAPT tool not found on host

**Fix:**
```bash
# Ensure Android SDK is installed and in PATH
set ANDROID_SDK_ROOT=C:\Android\SDK
# or
set ANDROID_HOME=/home/user/Android/Sdk

# Verify aapt is available
aapt version

# Run executor with explicit aapt path
set AAPT_PATH=C:\Android\SDK\build-tools\34.0.0\aapt.exe
cmd\android-executor\android-executor.exe
```

### Docker Backend Can't Reach Executor

**Cause:** `host.docker.internal` not available (not Docker Desktop)

**Fix:** 
- If using Docker on Linux, use the host machine's IP address instead:
  ```bash
  export ANDROID_EXECUTOR_URL=http://192.168.1.100:9090
  ```

### APK Metadata Shows Empty

**Cause:** APK file is corrupted or path is wrong

**Fix:**
- Verify APK file is valid: `aapt dump badging app.apk`
- Check file permissions
- Ensure path is accessible to executor service

## Future Enhancements

- [ ] Health check with executor availability status
- [ ] Metrics for executor calls (latency, failures)
- [ ] Load balancing for multiple executor instances
- [ ] Emulator lifecycle management on executor
- [ ] Device pool management via executor
- [ ] Cached APK metadata to reduce executor calls
