# Postman Testing Guide

## Collection Structure (Recommended)

1. Health Check
2. Upload APK
3. Start Experiment
4. Poll Android Status
5. Get Final Metrics
6. Stop Experiment (optional)

## Environment Variables

Create a Postman environment with:

- `baseUrl` = `http://localhost:8000`
- `apkId`
- `experimentId`

## 1) Health Check

```http
GET {{baseUrl}}/health
```

## 2) Upload APK

```http
POST {{baseUrl}}/upload/apk
```

Body: form-data

- key: `file` (type file)
- value: local apk file

Test script:

```javascript
const data = pm.response.json();
pm.environment.set("apkId", data.apk || data.id);
```

## 3) Start Experiment

```http
POST {{baseUrl}}/experiment/start
Content-Type: application/json
```

Contract notes for this request:

- Top-level keys can be snake_case or camelCase aliases.
- `expected` keys should be snake_case: `app_state`, `running`, `not_crash`, `not_anr`, `should_recover`.
- Scenario step keys should use `duration_seconds` (not `duration`).

Body example:

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

Test script:

```javascript
const data = pm.response.json();
pm.environment.set("experimentId", data.id);
```

### Additional Start Body Templates

Use these in the same request to run different fault models quickly.

#### Template A: Android kill and relaunch

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

#### Template B: Android network degrade and recover

```json
{
  "fault_type": "network_disable",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 85,
  "apk": "{{apkId}}",
  "android_run": {
    "avd_name": "Pixel_8a",
    "headless": true,
    "reset_app_state": false
  },
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

#### Template C: Android permission and lifecycle stress

```json
{
  "fault_type": "revoke_location",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 80,
  "apk": "{{apkId}}",
  "android_run": {
    "avd_name": "Pixel_8a",
    "headless": true,
    "reset_app_state": false
  },
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

#### Template D: Docker latency and packet loss

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
  "expected": {
    "running": true
  }
}
```

## 4) Poll Android Status

```http
GET {{baseUrl}}/experiment/android/status?id={{experimentId}}
```

Use Collection Runner with delay (for example 2000 ms) until terminal state.

## 5) Get Final Metrics

```http
GET {{baseUrl}}/experiment/metrics?id={{experimentId}}
```

Expected Android fields:

- `health`
- `recovery`
- `timeline`
- `state_transitions`
- `summary`
- `validation`

## 6) Optional Stop

```http
POST {{baseUrl}}/experiment/stop?id={{experimentId}}
```

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
