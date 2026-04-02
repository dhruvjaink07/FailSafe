# FailSafe API Reference

## Base URL

```text
http://localhost:8000
```

## Audience

- Backend engineers integrating resilience orchestration.
- Frontend engineers building test dashboards and run controls.
- QA and SRE teams running Postman-based resilience tests.

## Endpoint Map

| Area | Method | Path | Purpose |
| --- | --- | --- | --- |
| Health | GET | /health | Liveness check |
| APK upload | POST | /upload/apk | Upload APK and extract package/activity |
| Backend | POST | /experiments/backend/start | Start backend resilience run |
| Backend | GET | /experiments/backend/status?id=... | Backend lifecycle status payload |
| Backend | POST | /experiments/backend/stop?id=... | Stop backend run |
| Backend | GET | /experiments/backend/metrics?id=... | Backend metrics payload |
| Android | POST | /experiments/android/start | Start Android resilience run |
| Android | GET | /experiments/android/status?id=... | Android runtime status payload |
| Android | POST | /experiments/android/stop?id=... | Stop Android run |
| Android | GET | /experiments/android/metrics?id=... | Android metrics payload |
| Frontend | POST | /experiments/frontend/start | Start frontend resilience run |
| Frontend | GET | /experiments/frontend/status?id=... | Frontend lifecycle status payload |
| Frontend | POST | /experiments/frontend/stop?id=... | Stop frontend run |
| Frontend | GET | /experiments/frontend/metrics?id=... | Frontend metrics payload |
| Frontend collector | POST | /frontend/metrics | Ingest browser-collected frontend metrics batches |
| Presets | GET | /scenarios/presets | List available Android scenario presets and fault types |

## Data Persistence

### Shared tables

- `experiments`: lifecycle and run metadata.
- `metrics_raw`: time-series samples.
- `metrics_aggregated`: aggregated endpoint metrics.

### Docker report table

- `experiment_summary`: Docker/system summary fields.

### Android-specific report tables

- `android_experiment_summary`: typed Android summary fields.
- `android_experiment_report`: full Android report JSON (`report` JSONB).

## See Also

- Platform validation workflow: `docs/testing/README.md`
- Backend contract details: `docs/api/backend-api.md`
- Frontend workflow details: `docs/api/frontend-testing.md`
- Postman examples: `docs/api/postman-testing.md`
