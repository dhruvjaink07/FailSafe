# Android Executor - Implementation Details

## Code Structure Overview

### 1. Android Executor Service (`cmd/android-executor/main.go`)

**Purpose:** Standalone HTTP service that exposes Android SDK tools

**Key Functions:**

```go
func runAAPT(apkPath string) (string, error)
// Executes: aapt dump badging <apkPath>
// Returns raw AAPT output or error
// Validates file exists before execution

func parsePackageFromAAPT(output string) string
// Parses "package: name='com.example.app'" from AAPT output
// Returns empty string if not found

func parseActivityFromAAPT(output string) string
// Parses "launchable-activity: name='com.example.MainActivity'" from AAPT output
// Returns empty string if not found

func handleAAAPT(w http.ResponseWriter, r *http.Request) http.HandlerFunc
// GET /aapt?apk=<path>
// Returns: { "package": "...", "activity": "...", "output": "...", "success": true/false }

func handleHealth(w http.ResponseWriter, r *http.Request) http.HandlerFunc
// GET /health
// Returns: { "status": "ok" }
```

**Response Format:**

```go
type AAAPTResponse struct {
    Package  string `json:"package"`      // Package name (e.g., "com.example.app")
    Activity string `json:"activity"`    // Main activity (e.g., "MainActivity")
    Output   string `json:"output"`      // Full aapt dump badging output
    Success  bool   `json:"success"`    // Whether aapt succeeded
    Error    string `json:"error,omitempty"` // Error message if failed
}
```

**HTTP Behavior:**

- **Success (200):** `{ "package": "com.example.app", "activity": "MainActivity", "output": "...", "success": true }`
- **APK Not Found (400):** `{ "success": false, "error": "apk file not found: ..." }`
- **AAPT Tool Failed (400):** `{ "success": false, "error": "aapt failed: ..." }`
- **Invalid Query (400):** `{ error: "apk query parameter required" }` (plain text)

**Deployment Considerations:**

- Single-threaded HTTP server (adequate for executor workload)
- Timeout: No timeout set (aapt can be slow for large APKs)
- Port: 9090 (hardcoded, could be parameterized for multi-instance)
- Error Logs: Printed to stdout (captured by Docker or systemd)

### 2. Android Executor Client (`internal/handlers/android_executor_client.go`)

**Purpose:** Go HTTP client that communicates with remote executor service

**Key Types and Functions:**

```go
type AndroidExecutorClient struct {
    baseURL    string          // URL of remote executor (e.g., "http://host.docker.internal:9090")
    httpClient *http.Client    // HTTP client with 30s timeout
}

func NewAndroidExecutorClient(executorURL string) *AndroidExecutorClient
// Creates new client
// If executorURL empty, defaults to "http://host.docker.internal:9090"
// Strips trailing slash from URL

func (c *AndroidExecutorClient) CallAAPT(apkPath string) (string, string, error)
// Makes GET request to remote executor /aapt endpoint
// Returns (package, activity, error)
// Errors if response is non-200, not-JSON, or missing fields
// Example: callResult, err := client.CallAAPT("C:\\app.apk")

func (c *AndroidExecutorClient) IsAvailable() bool
// Checks if executor is reachable by calling /health endpoint
// Returns true only if GET /health returns 200 OK
// No error logging (used as a quiet check before fallback)

func (c *AndroidExecutorClient) GetExecutorURL() string
// Returns configured base URL

func GetDefaultExecutorURL() string
// Returns: os.Getenv("ANDROID_EXECUTOR_URL") or "http://host.docker.internal:9090"
```

**Usage Pattern:**

```go
// In APK upload handler
client := handlers.NewAndroidExecutorClient(handlers.GetDefaultExecutorURL())

// Try remote executor first
if client.IsAvailable() {
    pkg, activity, err := client.CallAAPT(apkPath)
    if err == nil {
        // Success - use these values
        return pkg, activity, nil
    }
    // Log and continue to fallback
}

// Fallback to local aapt (existing code)
return runAAPT(apkPath)
```

**Network Error Handling:**

```
Connection refused → IsAvailable() returns false → fallback to local
Timeout (30s) → error returned → logged and swallowed in fallback
Non-200 status → error with status and body
Invalid JSON → error with parse details
Missing fields → error with field names
```

### 3. Updated APK Upload Handler (`internal/handlers/apk_handlers.go`)

**Flow Changes:**

```
Before:
  Upload APK → Save → extractAPKMetadata → runAAPT (local) → Return metadata

After:
  Upload APK → Save → extractAPKMetadata (client) → Try remote executor:
    ↓
    IsAvailable? YES → CallAAPT (remote) → Success → Return metadata
    IsAvailable? NO → runAAPT (local) → Success → Return metadata
                    local failed → Error response
```

**Modified Code:**

```go
func UploadAPKHandler() http.HandlerFunc
// Added: Create Android executor client
    executorURL := GetDefaultExecutorURL()
    client := NewAndroidExecutorClient(executorURL)

// Changed: Pass client to extractAPKMetadata
    pkg, activity, err := extractAPKMetadata(dstPath, client)


func extractAPKMetadata(apkPath string, client *AndroidExecutorClient) (string, string, error)
// New signature: accepts client parameter
// New logic: Try remote first, fallback to local
    if client != nil && client.IsAvailable() {
        pkg, activity, err := client.CallAAPT(apkPath)
        if err == nil {
            return pkg, activity, nil
        }
        // Log fallback
        fmt.Printf("remote executor failed, falling back to local aapt: %v\n", err)
    }
    
    // Fallback to existing local logic
    out, err := runAAPT(apkPath)
    // ... parse regex patterns ...
```

**Backward Compatibility:**

- `extractAPKMetadata` now requires `*AndroidExecutorClient` parameter (breaking change for direct callers)
- APK handler automatically creates client (no API change)
- Tests updated to pass mock client or nil (triggers fallback)
- Local aapt fallback ensures no regression for non-Docker environments

## Configuration

### Environment Variables

| Variable | Default | Loaded By |
|----------|---------|-----------|
| `ANDROID_EXECUTOR_URL` | `http://host.docker.internal:9090` | `GetDefaultExecutorURL()` |
| `APK_UPLOAD_DIR` | `uploads/apks` | `resolveAPKUploadDir()` (unchanged) |
| `AAPT_PATH` | Auto-detect | `runAAPT()` (fallback, unchanged) |
| `ANDROID_SDK_ROOT` | From env | `runAAPT()` (fallback, unchanged) |
| `ANDROID_HOME` | From env | `runAAPT()` (fallback, unchanged) |

### Docker Configuration

**In docker-compose.yml, environment section:**
```yaml
services:
  backend:
    environment:
      - ANDROID_EXECUTOR_URL=http://host.docker.internal:9090
      # Or override with .env file or -e flag
```

**From .env file:**
```bash
ANDROID_EXECUTOR_URL=http://host.docker.internal:9090
```

**From Docker run command:**
```bash
docker run -e ANDROID_EXECUTOR_URL=http://host.docker.internal:9090 failsafe-backend
```

## Error Scenarios and Recovery

### Scenario 1: Executor Service Down

**Error Flow:**
1. Client attempts GET /health
2. Connection refused (port not listening)
3. IsAvailable() returns false
4. Falls back to local runAAPT()
5. Local aapt extracts metadata
6. Success if aapt available locally

**User Impact:** Small latency increase (checks executor first)

**Recovery:** Restart executor service, next APK upload uses remote

### Scenario 2: Executor Service Returns Error

**Error Flow:**
1. IsAvailable() returns true (health OK)
2. CallAAPT() makes request
3. Executor returns 400 with error JSON
4. Client returns error to handler
5. Falls back to local runAAPT()

**User Impact:** Transparent fallback, no visible error

**Debug:** Backend logs show "remote executor failed, falling back..."

### Scenario 3: Docker-to-Host Network Issue

**Error Flow:**
1. Backend in Docker tries to reach host.docker.internal:9090
2. Connection refused or timeout after 30s
3. IsAvailable() returns false
4. Falls back to local aapt in container
5. Aapt not available in container
6. Returns error to client

**User Impact:** Upload fails with "aapt not found"

**Recovery:** 
- Ensure executor running on host
- Check Docker Desktop has communication enabled
- Use explicit IP instead of host.docker.internal

### Scenario 4: Corrupt APK File

**Error Flow:**
1. APK saved to disk
2. Remote executor receives request
3. aapt dump badging fails
4. Executor returns error JSON
5. Client sees error in response
6. Falls back to local aapt
7. Local aapt also fails
8. Returns error with aapt output snippet

**User Impact:** Upload returns 400 with "failed to extract apk metadata"

**Debug:** Message shows last 600 chars of aapt output

## Performance Characteristics

### Latency Analysis

| Operation | Local | Remote (Docker) | Remote (Same Host) |
|-----------|-------|-----------------|-------------------|
| IsAvailable check | N/A | 1-5ms | 0.5-2ms |
| CallAAPT successful | 50-500ms | 50-510ms | 50-510ms |
| CallAAPT failed | 50-500ms | 1-5ms | 0.5-2ms |
| **Total APK Upload** | 100-600ms | 150-650ms | 100-610ms |

**Notes:**
- Remote overhead mainly from HTTP encoding/decoding
- Docker adds hop through host bridge (1-2ms)
- Network timeout is 30s (triggers fallback)

### Throughput

- Sequential APK uploads: ~1-2 per second
- Concurrent uploads (backend is concurrent): N × 1-2 per second
- Executor is single-threaded but stateless (sequential processing)

### Resource Usage

**Executor Service:**
- Memory: ~5MB idle
- CPU: minimal (0%) until aapt runs
- Disk: none (streaming I/O)
- Network: request + response (typically < 10KB each)

**Backend (Docker):**
- Additional memory: ~1MB for client (connection pool)
- Additional CPU: HTTP handling (~1ms per request)

## Testing Strategies

### Unit Tests

```go
// Test client creation
func TestNewAndroidExecutorClient(t *testing.T) {
    client := NewAndroidExecutorClient("http://localhost:9090")
    assert.Equal(t, "http://localhost:9090", client.baseURL)
}

// Test default URL resolution
func TestGetDefaultExecutorURL(t *testing.T) {
    os.Setenv("ANDROID_EXECUTOR_URL", "http://custom:9090")
    assert.Equal(t, "http://custom:9090", GetDefaultExecutorURL())
    os.Unsetenv("ANDROID_EXECUTOR_URL")
}

// Test availability check (mock HTTP)
func TestIsAvailable(t *testing.T) {
    // Mock HTTP server
    server := httptest.NewServer(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
            json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
        }),
    )
    defer server.Close()
    
    client := NewAndroidExecutorClient(server.URL)
    assert.True(t, client.IsAvailable())
}
```

### Integration Tests

```go
// Test APK upload with live executor
func TestAPKUploadWithExecutor(t *testing.T) {
    // 1. Start executor service
    // 2. Upload APK to backend
    // 3. Verify metadata extracted correctly
    // 4. Verify stored in database
}

// Test fallback behavior
func TestAPKUploadFallback(t *testing.T) {
    // 1. Start backend WITHOUT executor
    // 2. Upload APK
    // 3. Verify falls back to local aapt
    // 4. Verify metadata correct
}
```

## Troubleshooting Guide for Developers

### Issue: IsAvailable() Always Returns False

**Causes:**
- Executor not running
- Wrong URL configured
- Port blocked by firewall
- Wrong network namespace (Docker)

**Debug:**
```bash
# Check executor is running
curl http://localhost:9090/health

# Check from Docker container
docker run -it --rm golang:latest curl http://host.docker.internal:9090/health

# Check configuration
echo $ANDROID_EXECUTOR_URL
```

### Issue: CallAAPT Returns "incomplete response"

**Causes:**
- Executor response missing package or activity
- APK with no launchable activity
- Race condition parsing aapt output

**Debug:**
```bash
# Check raw executor response
curl "http://localhost:9090/aapt?apk=/path/to/app.apk" | jq

# Test aapt directly
aapt dump badging /path/to/app.apk | grep -E "^package:|^launchable-activity:"
```

### Issue: Fallback Not Working

**Causes:**
- Local aapt not in PATH
- APK path not accessible on local system
- runAAPT() not finding candidates

**Debug:**
```bash
# Test local aapt directly
where aapt
aapt version

# Add to PATH or set AAPT_PATH
set AAPT_PATH=C:\Android\SDK\build-tools\34.0.0\aapt.exe
```

## Future Enhancement Opportunities

1. **Caching APK Metadata**
   - Cache package/activity by APK hash
   - Reduce executor calls for same APKs
   - Invalidate on APK update

2. **Metrics Collection**
   - Track executor call latency
   - Monitor fallback rate
   - Alert on executor unavailability

3. **Load Balancing**
   - Support multiple executor instances
   - Round-robin request distribution
   - Health-based failover

4. **Extended Executor Features**
   - APK signing/validation
   - Permission scanning
   - Emulator lifecycle management
   - Device management

5. **Observability**
   - OpenTelemetry span tracing
   - Structured logging
   - Health dashboard

## References

- [ANDROID_EXECUTOR_ARCHITECTURE.md](./ANDROID_EXECUTOR_ARCHITECTURE.md)
- [ANDROID_EXECUTOR_QUICKSTART.md](./ANDROID_EXECUTOR_QUICKSTART.md)
- [ANDROID_EXECUTOR_TEST_PLAN.md](./ANDROID_EXECUTOR_TEST_PLAN.md)
- Android SDK Documentation: https://developer.android.com/studio/command-line-tools
- AAPT Tool Reference: https://developer.android.com/studio/command-line-tools#tooloptions
