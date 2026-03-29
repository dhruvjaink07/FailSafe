# FailSafe API Reference

## Base URL

```text
http://localhost:8080
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
| Experiment | POST | /experiment/start | Start Docker or Android resilience run |
| Experiment | GET | /experiment/get?id=... | Fetch experiment object and lifecycle state |
| Experiment | GET | /experiment/metrics?id=... | Fetch final/full metrics report |
| Android runtime | GET | /experiment/android/status?id=... | Lightweight polling payload during active run |
| Experiment | POST | /experiment/stop?id=... | Stop an active run |
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

- Backend contract details: `docs/api/backend-api.md`
- Frontend workflow details: `docs/api/frontend-testing.md`
- Postman examples: `docs/api/postman-testing.md`
