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

## 3) Start Experiment

### Request

```http
POST /experiment/start
```

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

## 5) Get Metrics

### Request

```http
GET /experiment/metrics?id={experiment_id}
```

### Response

For Android runs, response includes:

- `health`
- `recovery`
- `stability`
- `state_transitions`
- `timeline`
- `replay_hints`
- `validation`
- `summary`
- `scenario`

## 6) Android Live Status

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

## 7) Stop Experiment

### Request

```http
POST /experiment/stop?id={experiment_id}
```

### Response

```text
experiment stopped
```

## 8) Scenario Presets

### Request

```http
GET /scenarios/presets
```

### Response

Preset names, fault types, and trigger types.

## Error Responses

- `400 Bad Request`: invalid payload, missing id, invalid APK id, preset load failure.
- `405 Method Not Allowed`: wrong method for endpoint.
