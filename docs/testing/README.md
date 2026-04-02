# Platform Validation Workflow

This guide covers the new platform-separated handler tests and the recommended Postman execution order.

## What It Covers

- Backend lifecycle routes under `/experiments/backend/*`
- Android lifecycle routes under `/experiments/android/*`
- Frontend lifecycle routes under `/experiments/frontend/*`
- Frontend metric ingestion under `/frontend/metrics`

## Go Test Harness

Run the handler and orchestration checks with:

```powershell
go test ./internal/handlers ./cmd/controller ./internal/orchestrator
```

What the tests verify:

- Old `/experiment/*` routes are no longer registered.
- Backend and Android routes work through fake-backed handlers without Docker or emulator dependencies.
- Frontend lifecycle still works end to end with real HTTP requests.
- Frontend collector ingestion and metrics aggregation remain connected.

Why the fake-backed tests still matter:

- They validate handler wiring, payload validation, and route separation without waiting for Docker, adb, or an emulator.
- They make failures deterministic, so QA can tell whether the problem is the HTTP contract or the external runtime.
- They complement, rather than replace, the real scenarios. Use the real backend and Android runs for integration coverage and the fake-backed tests for fast contract checks.

## Postman Execution Order

Use the collections in this order for manual QA:

1. Backend collection: `docs/postman/failsafe-docker.collection.json`
2. Android collection: `docs/postman/failsafe-android.collection.json`
3. Frontend runtime flow: browser collector + `/frontend/metrics`

Recommended environment variables:

- `baseUrl` = `http://localhost:8000`
- `experimentId` = populated from the collection start response
- `apkId` = populated from the APK upload step for Android runs

## Contract Comparison Checklist

Compare these fields when you validate each platform:

- Start response includes `id`, `state`, `phase`, and platform fields.
- Status response reflects the correct experiment type.
- Metrics response is platform-specific and does not include the old shared compatibility shape.
- Stop response returns `experiment stopped`.

Backend-specific checks:

- `/experiments/backend/metrics` returns backend endpoint summaries.
- `/experiments/backend/status` is the lifecycle source of truth.

Android-specific checks:

- `/experiments/android/status` is the live polling endpoint.
- `/experiments/android/metrics` returns Android summary payloads.

Frontend-specific checks:

- `/experiments/frontend/status` returns the lifecycle state.
- `/experiments/frontend/metrics` returns frontend metrics plus score data.
- `/frontend/metrics` accepts browser-collected batches.

## Notes

- The legacy `/experiment/*` routes are intentionally removed.
- The frontend runner now resolves config from `/experiments/frontend/status`.
- If you regenerate docs, run `scripts/build-docs.ps1` after updating markdown.
