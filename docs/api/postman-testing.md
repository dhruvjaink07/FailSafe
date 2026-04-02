# Postman Testing Guide

## Launch Commands

Run backend API:

```powershell
go run cmd/controller/main.go
```

Run docs server:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\serve-docs.ps1 -Port 8090
```

## Environment Variables

Create a Postman environment with:

- `baseUrl` = `http://localhost:8000`
- `apkId`
- `experimentId`

## Separate Postman Collections

Import these collection files for strict separation:

- Docker/backend collection: `docs/postman/failsafe-docker.collection.json`
- Android collection: `docs/postman/failsafe-android.collection.json`

## Platform Endpoints

1. Health: `GET {{baseUrl}}/health`
2. Backend start: `POST {{baseUrl}}/experiments/backend/start`
3. Backend status: `GET {{baseUrl}}/experiments/backend/status?id={{experimentId}}`
4. Backend metrics: `GET {{baseUrl}}/experiments/backend/metrics?id={{experimentId}}`
5. Backend stop (optional): `POST {{baseUrl}}/experiments/backend/stop?id={{experimentId}}`

Each platform now has a dedicated route family, with no generic lifecycle endpoint.

## Section A: Backend (Docker) Collection

Use this mode for service/container fault testing.

Required shape:

- `targets`: Docker service/container names (for example `svc-b`)
- `target_type`: `docker`
- `observation_type`: `http`
- `duration_seconds` (or `duration`) > 0

Legacy key mapping (if using older collection body):

- `targetContainers` -> `targets`
- `containerEndpointMap` -> `targetEndpointMap`

### Docker Start Request Body Example

```json
{
  "faultType": "network_delay",
  "targets": ["svc-c"],
  "targetType": "docker",
  "observationType": "http",
  "observedEndpoints": ["http://svc-a", "http://svc-b", "http://svc-c"],
  "duration": 60,
  "adaptive": true,
  "stepIntensity": 20,
  "maxIntensity": 100,
  "dependencyGraph": {
    "http://svc-a": ["http://svc-b"],
    "http://svc-b": ["http://svc-c"],
    "http://svc-c": []
  },
  "targetEndpointMap": {
    "svc-a": ["http://svc-a"],
    "svc-b": ["http://svc-b"],
    "svc-c": ["http://svc-c"]
  }
}
```

### Docker Fault Templates

#### Docker A: network delay + packet loss

```json
{
  "fault_type": "network_delay",
  "targets": ["svc-b"],
  "target_type": "docker",
  "observation_type": "http",
  "observed_endpoints": ["http://svc-a", "http://svc-b", "http://svc-c"],
  "duration_seconds": 60,
  "scenarios": [
    { "type": "network_delay", "at": 5, "duration_seconds": 20 },
    { "type": "packet_loss", "at": 30, "duration_seconds": 15 }
  ],
  "expected": { "running": true }
}
```

#### Docker B: kill + cpu + memory

```json
{
  "fault_type": "kill",
  "targets": ["svc-c"],
  "target_type": "docker",
  "observation_type": "http",
  "observed_endpoints": ["http://svc-a", "http://svc-b", "http://svc-c"],
  "duration_seconds": 75,
  "scenarios": [
    { "type": "kill", "at": 8, "duration_seconds": 1 },
    { "type": "cpu_stress", "at": 20, "duration_seconds": 25 },
    { "type": "memory_stress", "at": 50, "duration_seconds": 15 }
  ],
  "expected": { "running": true }
}
```

## Section B: Android Collection

Use this mode for APK/app resilience testing.

Required shape:

- `targets`: app package target (for example `com.example.code`)
- `target_type`: `android`
- `observation_type`: `android`
- `apk`: upload id from `POST /upload/apk`
- `duration_seconds` (or `duration`) > 0

Expected and scenario keys should remain snake_case:

- `expected`: `app_state`, `running`, `not_crash`, `not_anr`, `should_recover`
- scenario step: `type`, `at`, `duration_seconds`, `intensity`, `trigger`

### Upload APK First (Android Collection)

```http
POST {{baseUrl}}/upload/apk
```

Body: form-data, key `file` (or `apk`).

Postman test script:

```javascript
const data = pm.response.json();
pm.environment.set("apkId", data.apk || data.id);
```

### Android Start Request Body Example

```json
{
  "fault_type": "kill_app",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 70,
  "apk": "{{apkId}}",
  "android_run": {
    "avd_name": "Pixel_8a",
    "headless": true,
    "reset_app_state": true
  },
  "scenarios": [
    { "type": "network_disable", "at": 10, "duration_seconds": 6 },
    { "type": "network_enable", "at": 18, "duration_seconds": 1 },
    { "type": "kill_app", "at": 30, "duration_seconds": 1 },
    { "type": "foreground_app", "at": 40, "duration_seconds": 2 }
  ],
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

Postman test script:

```javascript
const data = pm.response.json();
pm.environment.set("experimentId", data.id);
```

### Android Fault Templates

#### Android A: kill and relaunch

```json
{
  "fault_type": "kill_app",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 70,
  "apk": "{{apkId}}",
  "android_run": { "avd_name": "Pixel_8a", "headless": true },
  "scenarios": [
    { "type": "kill_app", "at": 20, "duration_seconds": 1 },
    { "type": "foreground_app", "at": 30, "duration_seconds": 2 }
  ],
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

#### Android B: network degrade and recover

```json
{
  "fault_type": "network_disable",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 85,
  "apk": "{{apkId}}",
  "android_run": { "avd_name": "Pixel_8a", "headless": true },
  "scenarios": [
    { "type": "network_disable", "at": 8, "duration_seconds": 7 },
    { "type": "network_enable", "at": 18, "duration_seconds": 1 },
    { "type": "network_flaky", "at": 28, "duration_seconds": 10 },
    { "type": "network_latency", "at": 44, "duration_seconds": 8 }
  ],
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

#### Android C: permission and lifecycle stress

```json
{
  "fault_type": "revoke_location",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 80,
  "apk": "{{apkId}}",
  "android_run": { "avd_name": "Pixel_8a", "headless": true },
  "scenarios": [
    { "type": "revoke_location", "at": 10, "duration_seconds": 2 },
    { "type": "background_app", "at": 22, "duration_seconds": 4 },
    { "type": "foreground_app", "at": 32, "duration_seconds": 2 }
  ],
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

## Polling And Results

Android live status (Android mode):

```http
GET {{baseUrl}}/experiments/android/status?id={{experimentId}}
```

Final metrics (both modes):

```http
GET {{baseUrl}}/experiments/android/metrics?id={{experimentId}}
```

Segregated metrics endpoints:

Backend/docker only:

```http
GET {{baseUrl}}/experiments/backend/metrics?id={{experimentId}}
```

Android only:

```http
GET {{baseUrl}}/experiments/android/metrics?id={{experimentId}}
```

## Fault Options Quick List

Backend/docker: `network_delay`, `packet_loss`, `cpu_stress`, `memory_stress`, `kill`.

Android: `kill_app`, `kill_repeated`, `network_disable`, `network_enable`, `network_flaky`, `network_latency`, `network_packet_loss`, `revoke_camera`, `revoke_storage`, `revoke_location`, `background_app`, `foreground_app`, `clear_data`.

## SQL Validation After Postman Run

Run in postgres:

```sql
SELECT experiment_id, updated_at
FROM android_experiment_report
ORDER BY updated_at DESC
LIMIT 5;
```

```sql
SELECT experiment_id, summary_result, failure_type, recovered, auto_recovered, recovery_time_ms
FROM android_experiment_summary
ORDER BY updated_at DESC
LIMIT 5;
```

```sql
SELECT experiment_id, count(*) AS raw_samples
FROM metrics_raw
GROUP BY experiment_id
ORDER BY max(timestamp) DESC
LIMIT 5;
```
