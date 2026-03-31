# Backend API Contract

## Authentication

Current API is unauthenticated for local/dev usage. Add auth middleware before public exposure.

## Content Type

- JSON endpoints: `Content-Type: application/json`
- APK upload endpoint: `multipart/form-data`

## 1) Health

### Request

```http
GET /health
```

### Response

```text
OK
```

## 2) Upload APK

### Request

```http
POST /upload/apk
Content-Type: multipart/form-data
```

Form-data fields:

- `file` (preferred) or `apk`: binary APK file

### Response

```json
{
  "id": "384f624c-c218-43de-b013-65331eb6edaf",
  "apk": "384f624c-c218-43de-b013-65331eb6edaf",
  "path": "D:\\FailSafe\\uploads\\apks\\384f624c-c218-43de-b013-65331eb6edaf.apk",
  "package": "com.example.code",
  "activity": "com.example.code.MainActivity"
}
```

## 3) Start Experiment (Shared Endpoint)

### Request

```http
POST /experiment/start
```

This is one endpoint with two request modes:

- Docker mode: backend service/container resilience tests.
- Android mode: APK/app resilience tests.

### Payload

Top-level request accepts both snake_case and camelCase aliases for most fields.
The `expected` and `scenario` object fields are strict and should be sent in snake_case as shown below.

```json
{
  "fault_type": "kill_app",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 70,
  "apk": "384f624c-c218-43de-b013-65331eb6edaf",
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

### Key Fields

- `fault_type`: primary fault for non-scenario mode.
- `targets`: containers (Docker) or package targets (Android).
- `target_type`: `docker` or `android`.
- `observation_type`: `http` or `android`.
- `apk`: uploaded APK id from `/upload/apk`.
- `android_run.headless`: emulator background mode.
- `android_run.reset_app_state`: clear app data before run.
- `scenarios`: timed fault sequence.
- `expected`: expectation-based validation contract.

### Required Minimum For Valid Start

- `fault_type`: required.
- `targets`: required and non-empty.
- `duration_seconds` (or `duration`): required and must be greater than 0.

### Mode-Specific Required Fields

Docker mode:

- `target_type`: `docker`
- `observation_type`: `http`
- `targets`: Docker service/container names

Android mode:

- `target_type`: `android`
- `observation_type`: `android`
- `targets`: app package target(s)
- `apk` (or alias): uploaded APK id

### Accepted Top-Level Alias Keys

- `fault_type` or `faultType`
- `target_type` or `targetType`
- `observation_type` or `observationType`
- `observed_endpoints` or `observedEndpoints`
- `duration_seconds` or `duration`
- `scenario` or `scenarios`
- `scenario_preset` or `scenarioPreset`
- `android_run` or `androidRun`
- `apk`, `apk_id`, `uploadedApkId`, or `uploaded_apk_id`

### Expected Object (Strict Fields)

Use these exact keys inside `expected`:

- `app_state`
- `running`
- `not_crash`
- `not_anr`
- `should_recover`

Example:

```json
"expected": {
  "app_state": "foreground",
  "running": true,
  "not_crash": true,
  "not_anr": true,
  "should_recover": true
}
```

### Scenario Step Object (Strict Fields)

Each step in `scenario` or `scenarios` should use:

- `type`
- `at`
- `duration_seconds` (optional)
- `intensity` (optional)
- `trigger` (optional object)

Optional trigger object fields:

- `type`
- `pattern`
- `timeout_seconds`

### Common Payload Mistakes

- Using `targetContainers` instead of `targets`.
- Using `expected.notCrash` or `expected.notAnr` instead of `expected.not_crash` and `expected.not_anr`.
- Using `scenarios[].duration` instead of `scenarios[].duration_seconds`.
- Sending `duration_seconds: 0` which fails with `invalid duration`.

### Response

Returns experiment object with id, state, phase, and metadata.

### Backend Testing Request Bodies


Use these payloads directly for integration tests against `POST /experiment/start`.

#### Example: Backend (Docker) Experiment Request

```json
{
  "faultType": "kill",
  "targets": ["user-service"],
  "targetType": "docker",
  "observationType": "http",
  "observedEndpoints": [
    "http://api-gateway:8080/api/users/1",
    "http://api-gateway:8080/api/orders/10",
    "http://user-service:8081/users/1",
    "http://order-service:8082/orders/10",
    "http://payment-service:8083/payments/10",
    "http://inventory-service:8084/inventory/A1",
    "http://shipping-service:8085/shipping/10",
    "http://notification-service:8086/notifications/10",
    "http://recommendation-service:8087/recommendations/1"
  ],
  "duration": 60,
  "adaptive": true,
  "stepIntensity": 20,
  "maxIntensity": 100,
  "dependencyGraph": {
    "http://api-gateway:8080/api/users/1": ["http://user-service:8081/users/1"],
    "http://api-gateway:8080/api/orders/10": ["http://order-service:8082/orders/10"],
    "http://user-service:8081/users/1": ["http://recommendation-service:8087/recommendations/1"],
    "http://order-service:8082/orders/10": [
      "http://payment-service:8083/payments/10",
      "http://inventory-service:8084/inventory/A1",
      "http://shipping-service:8085/shipping/10"
    ],
    "http://payment-service:8083/payments/10": [],
    "http://inventory-service:8084/inventory/A1": [],
    "http://shipping-service:8085/shipping/10": ["http://notification-service:8086/notifications/10"],
    "http://notification-service:8086/notifications/10": [],
    "http://recommendation-service:8087/recommendations/1": ["http://inventory-service:8084/inventory/A1"]
  },
  "targetEndpointMap": {
    "api-gateway": [
      "http://api-gateway:8080/api/users/1",
      "http://api-gateway:8080/api/orders/10"
    ],
    "user-service": ["http://user-service:8081/users/1"],
    "order-service": ["http://order-service:8082/orders/10"],
    "payment-service": ["http://payment-service:8083/payments/10"],
    "inventory-service": ["http://inventory-service:8084/inventory/A1"],
    "shipping-service": ["http://shipping-service:8085/shipping/10"],
    "notification-service": ["http://notification-service:8086/notifications/10"],
    "recommendation-service": ["http://recommendation-service:8087/recommendations/1"]
  }
}
```

> **Note:**
> - Use this shape for backend (docker) experiments. For Android, see the Android example below.
> - The available fault types depend on the experiment mode:
>   - **Backend (Docker):** kill, network_delay, packet_loss, cpu_stress, memory_stress, etc.
>   - **Android:** kill_app, network_disable, network_latency, revoke_camera, etc.
> - The UI and API will only show valid fault types for the selected mode. Toggle the experiment type to see the relevant fault options.

### Embedded From Existing Collection (Docker)

The body below is the legacy payload format you shared:

```json
{
  "faultType": "network_delay",
  "targetContainers": ["svc-c"],
  "observedEndpoints": [
    "http://svc-a",
    "http://svc-b",
    "http://svc-c"
  ],
  "duration": 60,
  "adaptive": true,
  "stepIntensity": 20,
  "maxIntensity": 100,
  "dependencyGraph": {
    "http://svc-a": ["http://svc-b"],
    "http://svc-b": ["http://svc-c"],
    "http://svc-c": []
  },
  "containerEndpointMap": {
    "svc-a": ["http://svc-a"],
    "svc-b": ["http://svc-b"],
    "svc-c": ["http://svc-c"]
  }
}
```

Why this can fail:

- `targetContainers` is not a recognized key.
- `containerEndpointMap` is not a recognized key.

Corrected equivalent payload:

```json
{
  "faultType": "network_delay",
  "targets": ["svc-c"],
  "targetType": "docker",
  "observationType": "http",
  "observedEndpoints": [
    "http://svc-a",
    "http://svc-b",
    "http://svc-c"
  ],
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

### Embedded From Existing Collection (Android)

The Android request body you shared is valid with current API contract:

```json
{
  "fault_type": "kill_app",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 70,
  "apk": "3f99e4f9-1e61-442e-a587-257ed0549261",
  "android_run": {
    "avd_name": "Pixel_8a",
    "headless": true
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

### Fault Options You Can Test

Docker/backend fault types:

- `network_delay`
- `packet_loss`
- `cpu_stress`
- `memory_stress`
- `kill`

Android fault types:

- `kill_app`
- `kill_repeated`
- `network_disable`
- `network_enable`
- `network_flaky`
- `network_latency`
- `network_packet_loss`
- `revoke_camera`
- `revoke_storage`
- `revoke_location`
- `background_app`
- `foreground_app`
- `clear_data`

#### A) Android app kill and recovery validation

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
    { "type": "kill_app", "at": 25, "duration_seconds": 1 },
    { "type": "foreground_app", "at": 33, "duration_seconds": 2 }
  ],
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

#### B) Android network chaos sequence

```json
{
  "fault_type": "network_disable",
  "targets": ["com.example.code"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 90,
  "apk": "{{apkId}}",
  "android_run": {
    "avd_name": "Pixel_8a",
    "headless": true,
    "reset_app_state": false
  },
  "scenarios": [
    { "type": "network_disable", "at": 10, "duration_seconds": 8 },
    { "type": "network_enable", "at": 20, "duration_seconds": 1 },
    { "type": "network_flaky", "at": 32, "duration_seconds": 12 },
    { "type": "network_latency", "at": 50, "duration_seconds": 10 },
    { "type": "network_packet_loss", "at": 64, "duration_seconds": 10 }
  ],
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

#### C) Android permission disruption and foreground recovery

```json
{
  "fault_type": "revoke_camera",
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
    { "type": "revoke_camera", "at": 12, "duration_seconds": 2 },
    { "type": "background_app", "at": 24, "duration_seconds": 4 },
    { "type": "foreground_app", "at": 35, "duration_seconds": 2 }
  ],
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
```

#### D) Docker network delay cascade test

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

#### E) Docker resource stress test

```json
{
  "fault_type": "cpu_stress",
  "targets": ["svc-c"],
  "target_type": "docker",
  "observation_type": "http",
  "observed_endpoints": ["http://svc-a", "http://svc-b", "http://svc-c"],
  "duration_seconds": 75,
  "scenarios": [
    { "type": "cpu_stress", "at": 10, "duration_seconds": 25 },
    { "type": "memory_stress", "at": 45, "duration_seconds": 20 }
  ],
  "expected": {
    "running": true
  }
}
```

## 4) Get Experiment

### Request

```http
GET /experiment/get?id={experiment_id}
```

### Response

Experiment metadata object (`state`, `phase`, timestamps, baseline fields, etc.).

## 5) Get Metrics (Compatibility)

### Request

```http
GET /experiment/metrics?id={experiment_id}
```

### Response

Compatibility endpoint. It routes by experiment type internally.

For Android experiments, response includes:

- `health`
- `recovery`
- `stability`
- `state_transitions`
- `timeline`
- `replay_hints`
- `validation`
- `summary`
- `scenario`

For backend/docker experiments, response includes:

- `endpoints`
- `blast_radius_percent`
- `cascade_depth`
- `system_severity`
- `resilience_threshold`
- `timeline`

## 6) Get Backend Metrics (Segregated)

### Request

```http
GET /experiment/metrics/backend?id={experiment_id}
```

### Behavior

- Returns only backend/docker metrics shape.
- Returns `400` if experiment id belongs to Android run.

## 7) Get Android Metrics (Segregated)

### Request

```http
GET /experiment/metrics/android?id={experiment_id}
```

### Behavior

- Returns only Android metrics shape.
- Returns `400` if experiment id belongs to backend/docker run.

## 8) Android Live Status

### Request

```http
GET /experiment/android/status?id={experiment_id}
```

### Response

Lightweight polling payload containing:

- `phase`
- `state`
- `health`
- `progress`
- `timeline`
- recent transitions and fault events

## 9) Stop Experiment

### Request

```http
POST /experiment/stop?id={experiment_id}
```

### Response

```text
experiment stopped
```

## 10) Scenario Presets

### Request

```http
GET /scenarios/presets
```

### Response

Preset names, fault types, and trigger types.

## Error Responses

- `400 Bad Request`: invalid payload, missing id, invalid APK id, preset load failure.
- `405 Method Not Allowed`: wrong method for endpoint.
