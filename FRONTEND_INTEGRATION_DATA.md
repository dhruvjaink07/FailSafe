# FailSafe Frontend Integration Data

This document contains all the necessary information to build a compatible frontend with the FailSafe backend.

---

## 1. BACKEND BASE URL & ENVIRONMENT

```
Base URL: http://localhost:8000
Content Type: application/json (for all endpoints unless specified)
```

---

## 2. CORE DATA STRUCTURES

### Experiment Object

```json
{
  "id": "string (UUID)",
  "api_key_id": "string",
  "observed_endpoints": ["string"],
  "targets": ["string"],
  "target_type": "docker | android | frontend",
  "observation_type": "http | android",
  "fault_type": "string",
  "duration_seconds": "integer",
  "scenario": [
    {
      "type": "string",
      "at": "integer (seconds)",
      "duration_seconds": "integer (optional)",
      "intensity": "integer (optional)",
      "trigger": {
        "type": "string (optional)",
        "pattern": "string (optional)",
        "timeout_seconds": "integer (optional)"
      }
    }
  ],
  "expected": {
    "app_state": "string (optional)",
    "running": "boolean (optional)",
    "not_crash": "boolean",
    "not_anr": "boolean",
    "should_recover": "boolean (optional)"
  },
  "apk_path": "string (optional)",
  "package": "string (optional)",
  "activity": "string (optional)",
  "frontend_run": {
    "base_url": "string",
    "metrics_endpoint": "string",
    "target_urls": ["string"]
  },
  "state": "running | completed | failed",
  "phase": "baseline | injecting | recovering | completed",
  "fault_started_at": "ISO 8601 timestamp",
  "intensity": "integer",
  "adaptive": "boolean",
  "max_intensity": "integer",
  "step_intensity": "integer",
  "current_intensity": "integer",
  "max_stable_intensity": "integer",
  "breaking_intensity": "integer",
  "intensity_history": ["integer"],
  "timeline_history": {},
  "baseline_metrics": {
    "avg_latency": "float",
    "p95": "integer",
    "error_rate": "float"
  },
  "dependency_graph": {
    "endpoint": ["endpoint"]
  },
  "target_endpoint_map": {
    "service": ["endpoint"]
  },
  "graph_metadata": {
    "total_nodes": "integer",
    "max_depth": "integer"
  },
  "created_at": "ISO 8601 timestamp",
  "updated_at": "ISO 8601 timestamp"
}
```

### Frontend Metrics Object

```json
{
  "experiment_id": "string",
  "phase": "baseline | injecting | recovery",
  "page": "string (URL path)",
  "metrics": {
    "lcp": "float (Largest Contentful Paint in ms)",
    "cls": "float (Cumulative Layout Shift score)",
    "inp": "float (Interaction to Next Paint in ms)",
    "long_tasks": "integer (number of long tasks)",
    "errors": "integer (JS errors count)",
    "unhandled_rejections": "integer (unhandled promise rejections)"
  },
  "api_calls": [
    {
      "url": "string",
      "duration": "float (ms)",
      "status": "integer (HTTP status)"
    }
  ],
  "timestamp": "integer (milliseconds since epoch)"
}
```

### Frontend Metrics Batch (ingest format)

```json
{
  "metrics": [
    { /* FrontendMetrics object */ }
  ]
}
```

### API Key Object

```json
{
  "id": "string (UUID)",
  "key": "string (secret)",
  "role": "viewer | engineer | admin",
  "environment": "dev | prod",
  "created_at": "ISO 8601 timestamp"
}
```

---

## 3. PUBLIC ENDPOINTS (No API Key Required)

### 3.1 Health Check
```
GET /health

Response: "OK" (plain text)
Used to verify backend is running
```

### 3.2 Get Scenario Presets
```
GET /scenarios/presets

Response:
{
  "presets": [
    {
      "name": "string",
      "description": "string",
      "fault_types": ["string"],
      "trigger_types": ["string"],
      "scenario": [ /* ScheduledFault array */ ]
    }
  ]
}
Used to fetch available fault types and scenario templates
```

### 3.3 Upload APK
```
POST /upload/apk
Content-Type: multipart/form-data

Form fields: file OR apk (binary APK file)

Response:
{
  "id": "string (UUID)",
  "apk": "string (same as id)",
  "path": "string (server storage path)",
  "package": "string (app package name)",
  "activity": "string (main activity)"
}
Used for Android testing - extract APK metadata
```

### 3.4 Ingest Frontend Metrics
```
POST /frontend/metrics
Content-Type: application/json

Request: FrontendMetricsBatch (see data structures above)

Response: 
{
  "status": "accepted | error",
  "message": "string"
}
Browser collector posts metrics batches here
```

### 3.5 Create API Key
```
POST /internal/api-keys/create
Content-Type: application/json

Request:
{
  "role": "viewer | engineer | admin",
  "environment": "dev | prod"
}

Response:
{
  "id": "string",
  "key": "string (store securely!)",
  "role": "string",
  "environment": "string",
  "created_at": "ISO 8601 timestamp"
}
Used to provision API keys for authenticated requests
```

### 3.6 Get Experiment Status
```
GET /experiments/{platform}/status?id={experiment_id}

Supported platforms: backend | android | frontend

Response: Experiment object (see data structures)
Poll this endpoint ~every 2 seconds to track experiment state
```

### 3.7 Get Frontend Fault Command
```
GET /experiments/frontend/fault-command?id={experiment_id}

Response:
{
  "experiment_id": "string",
  "fault_type": "string",
  "intensity": "integer",
  "active": "boolean"
}
Returns current active fault command for browser injection
```

---

## 4. PROTECTED ENDPOINTS (Requires x-api-key header)

All protected endpoints require header:
```
x-api-key: <api-key-value>
```

### 4.1 Start Experiment (Backend - Docker)
```
POST /experiments/backend/start
Content-Type: application/json
x-api-key: <engineer|admin key>

Minimum required fields in request body:
{
  "fault_type": "string",
  "target_type": "docker",
  "targets": ["string (service names)"],
  "observation_type": "http",
  "duration_seconds": "integer (> 0)",
  "api_key_id": "string (optional)"
}

Full supported fields:
{
  "fault_type": "string",
  "targets": ["string"],
  "target_type": "docker",
  "observation_type": "http",
  "observed_endpoints": ["string (URLs)"],
  "duration_seconds": "integer",
  "adaptive": "boolean",
  "step_intensity": "integer",
  "max_intensity": "integer",
  "dependency_graph": {
    "endpoint": ["dependent_endpoints"]
  },
  "target_endpoint_map": {
    "service": ["endpoints"]
  },
  "scenario": [ /* ScheduledFault array */ ],
  "expected": { /* ExpectedState object */ }
}

Response: Experiment object

Valid fault types (backend):
- kill
- network_delay
- packet_loss
- cpu_stress
- memory_stress
```

### 4.2 Start Experiment (Android)
```
POST /experiments/android/start
Content-Type: application/json
x-api-key: <engineer|admin key>

Minimum required fields:
{
  "fault_type": "string",
  "target_type": "android",
  "targets": ["string (app package names)"],
  "observation_type": "android",
  "duration_seconds": "integer",
  "apk": "string (UUID from /upload/apk)",
  "android_run": {
    "avd_name": "string",
    "headless": "boolean",
    "reset_app_state": "boolean"
  }
}

Full supported fields:
{
  "fault_type": "string",
  "targets": ["string"],
  "target_type": "android",
  "observation_type": "android",
  "observed_endpoints": ["string"],
  "duration_seconds": "integer",
  "apk": "string",
  "android_run": {
    "avd_name": "string",
    "headless": "boolean",
    "reset_app_state": "boolean"
  },
  "adaptive": "boolean",
  "step_intensity": "integer",
  "max_intensity": "integer",
  "scenario": [ /* ScheduledFault array */ ],
  "expected": { /* ExpectedState object */ }
}

Response: Experiment object

Valid fault types (android):
- kill_app
- network_disable
- network_latency
- revoke_camera
- revoke_location
- kill_process
- battery_drain
```

### 4.3 Start Experiment (Frontend)
```
POST /experiments/frontend/start
Content-Type: application/json
x-api-key: <engineer|admin key>

Minimum required fields:
{
  "fault_type": "string",
  "target_type": "frontend",
  "duration_seconds": "integer",
  "frontend_run": {
    "base_url": "string (target URL)",
    "metrics_endpoint": "string (metrics ingest URL)",
    "target_urls": ["string"]
  }
}

Full supported fields:
{
  "fault_type": "string",
  "target_type": "frontend",
  "targets": ["string (optional)"],
  "duration_seconds": "integer",
  "frontend_run": {
    "base_url": "string",
    "metrics_endpoint": "string",
    "target_urls": ["string"]
  },
  "adaptive": "boolean",
  "step_intensity": "integer",
  "max_intensity": "integer",
  "scenario": [ /* ScheduledFault array */ ]
}

Response: Experiment object

Valid fault types (frontend):
- latency
- packet_loss
- bandwidth_limit
- cpu_throttle
- memory_constraint
```

### 4.4 Stop Experiment
```
POST /experiments/{platform}/stop?id={experiment_id}
x-api-key: <engineer|admin key>

Supported platforms: backend | android | frontend

Response: 
{
  "id": "string",
  "state": "completed",
  "message": "Experiment stopped"
}
```

### 4.5 Get Experiment Metrics
```
GET /experiments/{platform}/metrics?id={experiment_id}
x-api-key: <viewer|engineer|admin key>

Supported platforms: backend | android | frontend

Response (Frontend specific):
{
  "experiment_id": "string",
  "state": "string",
  "phase": "string",
  "total_metrics": "integer",
  "phases": {
    "baseline": { /* aggregated metrics */ },
    "injecting": { /* aggregated metrics */ },
    "recovery": { /* aggregated metrics */ }
  },
  "vitals": {
    "lcp": { "baseline": float, "injecting": float, "recovery": float },
    "cls": { "baseline": float, "injecting": float, "recovery": float },
    "inp": { "baseline": float, "injecting": float, "recovery": float }
  },
  "stability": {
    "long_tasks": { "baseline": int, "injecting": int, "recovery": int },
    "errors": { "baseline": int, "injecting": int, "recovery": int },
    "unhandled_rejections": { "baseline": int, "injecting": int, "recovery": int }
  },
  "api_quality": {
    "success_rate": float,
    "avg_latency": float,
    "error_count": int
  }
}
```

### 4.6 List Environment Containers
```
GET /environment/containers
x-api-key: <viewer|engineer|admin key>

Response:
{
  "timestamp": "ISO 8601",
  "count": "integer",
  "containers": [
    {
      "id": "string",
      "name": "string",
      "image": "string",
      "state": "string",
      "status": "string",
      "ports": "string",
      "running": "boolean"
    }
  ]
}
```

### 4.7 Start Container
```
POST /environment/containers/start
x-api-key: <engineer|admin key>

Request (JSON or query param):
{
  "name": "string (container name)"
}

Response:
{
  "name": "string",
  "action": "started | already_running",
  "timestamp": "ISO 8601"
}
Or as query: POST /environment/containers/start?name=container-name
```

### 4.8 Start Docker Engine
```
POST /environment/docker/engine/start
x-api-key: <engineer|admin key>

Response (success):
{
  "os": "windows | macOS | linux",
  "already_running": "boolean",
  "desktop_started": "boolean",
  "engine_ready": "boolean",
  "message": "string"
}

Response (failure - 503):
{
  "error": "string",
  "result": { /* same structure as success */ }
}
```

---

## 5. EXPERIMENT WORKFLOW

### Frontend Testing Workflow (Step-by-step)

1. **Start Experiment**
   ```
   POST /experiments/frontend/start
   Body: { fault_type, target_type: "frontend", duration_seconds, frontend_run: { base_url, metrics_endpoint, target_urls } }
   Store: experiment_id from response
   ```

2. **Browser Collector Posts Metrics** (happens in background)
   ```
   POST /frontend/metrics
   Body: FrontendMetricsBatch
   (Browser automation sends this automatically)
   ```

3. **Poll Status** (every 2-3 seconds)
   ```
   GET /experiments/frontend/status?id={experiment_id}
   Track: state and phase changes
   Stop when: state = "completed" or "failed"
   ```

4. **Fetch Final Metrics**
   ```
   GET /experiments/frontend/metrics?id={experiment_id}
   Parse: vitals, stability, api_quality by phase
   ```

5. **Render Results**
   Display: LCP, CLS, INP deltas between phases
   Show: Error counts, long tasks, recovery patterns

---

## 6. API KEY ROLES & PERMISSIONS

| Role | Permissions |
|------|------------|
| `viewer` | Read metrics only |
| `engineer` | Start/stop experiments, read metrics |
| `admin` | All permissions + provision new API keys |

---

## 7. ERROR RESPONSES

All errors follow this format:

```json
{
  "error": "string (error message)",
  "code": "string (error code)",
  "status": "integer (HTTP status)",
  "details": "string (optional extra info)"
}
```

Common HTTP Status Codes:
- `200`: Success
- `201`: Resource created
- `202`: Accepted (async)
- `400`: Bad request (invalid payload)
- `401`: Unauthorized (missing API key)
- `403`: Forbidden (insufficient permissions)
- `404`: Not found
- `409`: Conflict
- `500`: Server error
- `503`: Service unavailable

---

## 8. POLLING INTERVALS & TIMEOUTS

| Operation | Interval | Timeout |
|-----------|----------|---------|
| Experiment status | 2-3 seconds | 10 minutes |
| Health check | 5-10 seconds | 30 seconds |
| Metrics fetch | Once (after completion) | - |

---

## 9. FIELD ALIASES (camelCase support)

The backend accepts both snake_case and camelCase:

| snake_case | camelCase |
|-----------|-----------|
| `fault_type` | `faultType` |
| `target_type` | `targetType` |
| `observation_type` | `observationType` |
| `observed_endpoints` | `observedEndpoints` |
| `duration_seconds` | `duration` |
| `android_run` | `androidRun` |
| `frontend_run` | `frontendRun` |
| `step_intensity` | `stepIntensity` |
| `max_intensity` | `maxIntensity` |
| `target_endpoint_map` | `targetEndpointMap` |
| `apk_id`, `uploaded_apk_id` | `uploadedApkId` |

---

## 10. FRONTEND UI REQUIREMENTS

### Screens to Build

1. **Dashboard / Home**
   - Show active experiments
   - Quick start buttons
   - Links to experiment history

2. **Experiment Creator**
   - Select platform (Docker/Android/Frontend)
   - Choose fault type (from presets endpoint)
   - Configure duration, targets, intensity
   - Set expected outcomes

3. **Experiment Runner**
   - Show current state/phase
   - Live phase indicator (Baseline → Injecting → Recovery → Completed)
   - Intensity slider (if adaptive)
   - Cancel/Stop button

4. **Results Dashboard**
   - Vitals (LCP, CLS, INP) in table/chart
   - Stability metrics (errors, long tasks)
   - API performance (success rate, latency)
   - Phase comparison charts
   - Export/download results

5. **Environment Manager** (optional)
   - List containers
   - Start/stop containers
   - Start Docker engine
   - Health status

---

## 11. SAMPLE PAYLOAD REFERENCE

### Backend (Docker) Experiment
```json
{
  "faultType": "kill",
  "targets": ["user-service"],
  "targetType": "docker",
  "observationType": "http",
  "observedEndpoints": [
    "http://api-gateway:8080/api/users/1",
    "http://api-gateway:8080/api/orders/10"
  ],
  "duration": 60,
  "adaptive": true,
  "stepIntensity": 20,
  "maxIntensity": 100,
  "dependencyGraph": {
    "http://api-gateway:8080/api/users/1": ["http://user-service:8081/users/1"],
    "http://api-gateway:8080/api/orders/10": ["http://order-service:8082/orders/10"]
  },
  "targetEndpointMap": {
    "api-gateway": ["http://api-gateway:8080/api/users/1"],
    "user-service": ["http://user-service:8081/users/1"]
  }
}
```

### Frontend Experiment
```json
{
  "faultType": "latency",
  "targets": ["target-site"],
  "targetType": "frontend",
  "duration": 30,
  "frontendRun": {
    "baseUrl": "https://example.com",
    "metricsEndpoint": "http://localhost:8000/frontend/metrics",
    "targetUrls": ["/api/", "/"]
  }
}
```

---

## 12. INTEGRATION CHECKLIST

- [ ] API base URL configuration
- [ ] API key management (create, store, use in headers)
- [ ] Health check on app load
- [ ] Experiment creation form with validation
- [ ] Experiment start endpoint integration
- [ ] Status polling loop
- [ ] Metrics fetching
- [ ] Results rendering
- [ ] Error handling & display
- [ ] Loading states
- [ ] Mobile responsiveness
- [ ] Dark/Light mode (optional)

---

## 13. POSTMAN COLLECTION LOCATIONS

For reference and testing:
- Frontend collection: `docs/postman/failsafe-frontend.collection.json`
- Backend collection: `docs/postman/failsafe-docker.collection.json`
- Android collection: `docs/postman/failsafe-android.collection.json`

Import these into Postman for quick API testing.

---

## QUESTIONS FOR CHATGPT

When configuring with ChatGPT, ask:

1. How to implement a polling mechanism for experiment status?
2. How to structure Redux/Zustand state for experiment data?
3. How to display real-time metrics charts (LCP, CLS, INP)?
4. How to implement phase transitions UI?
5. How to handle long-running API calls with timeouts?
6. How to structure API service layer for backend communication?
7. Best practices for storing API keys securely in browser/frontend?
8. How to implement experiment history persistence?
9. How to create responsive forms for complex payloads?
10. How to implement dark mode with metrics visualization?
