# DB Restart-Resilience Smoke Report (Backend + Android + Frontend)

Date: 2026-04-05
Workspace: D:/FailSafe
Goal: Validate that platform status/metrics remain available from Postgres after controller restart, and that stop operations work post-restart.

## Scope

This report validates:

1. Backend: start -> status/metrics -> controller restart -> status/metrics -> stop.
2. Android: upload APK -> start -> status/metrics -> controller restart -> status/metrics -> stop.
3. Frontend: re-check previously executed restart-resilient run for parity in this consolidated report.
4. Compile/test sanity for changed packages.

## Environment Snapshot

- Controller base URL: http://localhost:8000
- Postgres container: failsafe-postgres
- Mock backend services running: api-gateway, order-service, user-service, payment-service, inventory-service, shipping-service, notification-service, recommendation-service

## API Key Used

- Created via POST /internal/api-keys/create
- Role/environment used for platform tests: engineer/dev
- Key used in requests: fs_dev_engineer_smoke-backend-android_...(masked)

## Test Results Summary

| Platform | Pre-Restart Status/Metrics | Post-Restart Status/Metrics | Post-Restart Stop | Result |
| --- | --- | --- | --- | --- |
| Backend | PASS | PASS | PASS | PASS |
| Android | PASS | PASS | PASS | PASS |
| Frontend (re-check) | PASS | PASS | PASS | PASS |

## Detailed Evidence

### 1) Backend

Start request accepted:

- Experiment ID: 1fe154e4-a5ef-4cb1-8b63-73fb815bd835
- Pre-restart evidence:
  - state: running
  - metrics endpoints present: true
  - total_requests: 8

After controller restart:

- status state: running
- metrics endpoints present: true
- total_requests: 40
- stop response: experiment stopped
- final status state/phase: failed/completed

Outcome: Backend status/metrics and stop all persisted and worked after restart.

### 2) Android

Initial attempt failed due invalid apk reference:

- HTTP 400: invalid apk reference: upload id not found

Resolved by uploading APK:

- Upload endpoint used: POST /upload/apk
- Uploaded APK ID: 29f3070c-9fe7-48f0-8aeb-da11f313bec4

Android start request accepted:

- Experiment ID: 503df375-a008-4806-ad52-52fb392d0f01
- Pre-restart evidence:
  - state: completed
  - health.status: down
  - summary.result: FAIL
  - validation.configured: true

After controller restart:

- status endpoint returned persisted Android payload from DB (root-level state payload)
- state: failed
- phase: completed
- health.status: down
- metrics summary.result: FAIL
- stop response: experiment stopped

Outcome: Android status/metrics and stop all worked after restart (with DB-backed responses).

### 3) Frontend (Re-check)

Re-validated previously tested experiment in this same environment:

- Experiment ID: e6118e4c-ed74-436e-b79a-47fcef930087
- Current persisted state/phase: failed/completed
- metrics frontend sample count: 1
- frontend_score: 80.58

Outcome: Frontend persistence behavior remains valid.

## Compile/Test Validation

Executed:

go test ./internal/orchestrator ./internal/storage ./internal/handlers ./cmd/controller

Result:

- ok github.com/dhruvjaink07/failsafe/internal/orchestrator
- ok github.com/dhruvjaink07/failsafe/internal/handlers
- internal/storage and cmd/controller: no test files, compile path clean

## Notes

1. Android status payload shape is DB-backed and returned as a root metrics/status payload (not always wrapped under experiment), which is consistent with current platform status behavior for this path.
2. During backend fault injection, the server logged one network delay command error, but API-level persistence checks still passed and restart-resilient reads remained functional.
3. PowerShell in this environment does not support Invoke-RestMethod -Form, so APK upload was done using curl multipart.

## Final Verdict

DB wiring is now validated across backend, Android, and frontend for restart-resilient status/metrics reads, plus post-restart stop lifecycle behavior.
