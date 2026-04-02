# Frontend Web Domain Promotion Plan (2026-04-02)

## Scope

This plan is intentionally scoped to web target execution for testing target web pages.

Out of scope:

- Android flows
- APK upload/emulator concerns
- FailSafe dashboard UI testing

## Strategic Direction

FailSafe should treat web as a first-class execution domain, at parity with docker and android.

Target model:

- orchestrator -> target(docker|android|web) -> monitor -> unified metrics

Web is not dashboard UI testing and not Android-adjacent logic. It is browser-runtime resilience testing under a single experiment lifecycle.

## Current State (Observed)

### 1) Usable web runtime exists (manual path)

- `playwright/runner.js` launches Chromium, opens target pages, and runs baseline/injecting/recovery phases.
- `playwright/collector.js` collects browser-side metrics (LCP, CLS, INP, long tasks, errors, API calls).
- `playwright/sender.js` batches and posts metric payloads.
- Runner can resolve runtime config from `/experiments/frontend/status` and submit to `/frontend/metrics`.

Result: manual live-site web testing is operational.

### 2) Web HTTP contract is tested

- `internal/handlers/platform_endpoints_test.go` includes:
  - validation tests for frontend routes
  - `TestFrontendLifecycleConnectivity` end-to-end flow:
    - start frontend experiment
    - ingest collector batch to `/frontend/metrics`
    - fetch `/experiments/frontend/metrics`
    - stop experiment
- Verified passing in this audit run.

### 3) QA helper script exists

- `scripts/qa/test-frontend-platform.ps1` exercises the same route family with synthetic collector data.

### 4) Core execution abstractions are still stubs

- `internal/execution/target.go` is currently package-only.
- `internal/execution/docker_target.go` is currently package-only.
- `internal/execution/android_target.go` is currently package-only.
- `internal/execution/web_target.go` is currently package-only.

Impact: target abstraction exists structurally but not behaviorally; web cannot yet be first-class in execution-layer code.

### 5) Web monitor and web injector are stubs

- `internal/monitoring/web_monitor.go` is currently package-only.
- `internal/fault/web_injector.go` is currently package-only.

Impact: web push-ingestion and web fault control are not represented as concrete domain components yet.

### 6) Web domain entrypoint is missing

- `cmd/web-test/` exists but is empty.

Impact: there is no dedicated executable entrypoint for web target runs.

## Gaps To Reach First-Class Domain

### 1) No Playwright automated test suite yet

- There are no Playwright spec files (`*.spec.*` / `*.test.*`) in the repo.
- `package.json` still has placeholder test script:
  - `"test": "echo \"Error: no test specified\" && exit 1"`

Impact: web-page testing currently depends on manual runner execution.

### 2) Scenario configs are present but empty

- `configs/scenarios/web/offline.json`
- `configs/scenarios/web/latency.json`
- `configs/scenarios/web/cpu_throttle.json`

Impact: no reusable, versioned scenario library for page tests.

### 3) Internal frontend module tree is mostly scaffold

`internal/frontend/` has 13 JS files:

- non-empty: 3
- empty: 10

Non-empty:

- `internal/frontend/automation/playwright/runner.js`
- `internal/frontend/runtime/collector.js`
- `internal/frontend/transport/sender.js`

Empty placeholders include chaos/runtime/lighthouse modules.

Impact: architecture suggests richer web-testing capabilities than currently implemented.

### 4) Duplicate implementation paths without clear single source

- Top-level `playwright/*` files are implemented.
- Internal `internal/frontend/*` equivalents are also partially implemented but differ in content.

Impact: maintenance risk and unclear canonical runtime path.

### 5) Web execution-domain contracts are not codified

- No implemented target interface and concrete target types for docker/android/web.
- No implemented web monitor semantics for passive metric ingestion.
- No implemented web injector semantics for Playwright/CDP control delegation.

Impact: architecture intent is present, but code-level domain contracts are not finalized.

## Completion Snapshot (First-Class Web Domain)

- Manual runnable flow: yes.
- Frontend route contract and ingestion: yes.
- Target abstraction implementation parity (docker/android/web): no.
- Web monitor and injector implementation: no.
- Automated JS test coverage (unit/e2e): no.
- Scenario catalog for repeatable web chaos runs: no.
- CI-ready web test command set: no.
- Dedicated web domain executable: no.

## Implementation Plan (Promotion Phases)

## Phase 1: Codify execution-layer contracts

- Implement `execution.Target` interface in `internal/execution/target.go`.
- Implement minimal concrete targets:
  - `internal/execution/docker_target.go`
  - `internal/execution/android_target.go`
  - `internal/execution/web_target.go`
- Ensure orchestrator uses target abstraction uniformly when creating runs.

Exit criteria:

- web target type is not special-cased through no-op monitor/injector path.
- docker/android/web all satisfy one lifecycle contract.

## Phase 2: Implement web monitor + injector domain behavior

- Implement `internal/monitoring/web_monitor.go` as passive ingestion adapter.
- Implement `internal/fault/web_injector.go` as control plane for Playwright run behavior (throttle/offline/abort/cpu/JS chaos selection).
- Keep browser-runtime logic in JS runtime and automation layers, not deep Go embedding.

Exit criteria:

- web monitor lifecycle is explicit in code.
- web injector can request scenario-level behavior changes without direct browser internals in Go.

## Phase 3: Establish dedicated web entrypoint

- Add `cmd/web-test/main.go` to run web target experiments using first-class execution APIs.

Exit criteria:

- web target can be launched through a dedicated executable path.

## Phase 4: Establish automated web-page tests

- Add Playwright test specs under `playwright/tests/`:
  - smoke navigation + collector injection
  - chaos phase transition assertions
  - payload shape assertion before POST
Add npm scripts:

- `test:frontend:e2e`
- `test:frontend:smoke`

## Phase 5: Add JS unit tests for collector and sender

- Validate sender batching, retry, and flush semantics.
- Validate collector metric mapping and API call capture.
- Suggested stack: Node test runner or Jest/Vitest with minimal setup.

## Phase 6: Make scenarios reusable

- Populate web scenario JSON files with real defaults:
  - offline
  - latency
  - cpu throttle
- Update runner to load selected scenario by name/path.

## Phase 7: Resolve duplicate runtime paths

- Pick one canonical implementation path:
  - either keep `playwright/*` as source of truth
  - or migrate to `internal/frontend/*`
- Remove or clearly mark stale duplicates.

## Suggested Definition Of Done

Web is first-class and production-ready when all are true:

- A concrete target abstraction is implemented and used for docker/android/web.
- `web_target.go`, `web_monitor.go`, and `web_injector.go` are implemented (not stubs).
- `cmd/web-test/main.go` exists and can execute a web run lifecycle.
- Playwright smoke test runs in CI.
- At least one chaos resilience scenario test runs in CI.
- Collector and sender unit tests pass.
- Scenario JSON files are non-empty and used by runner.
- One canonical runtime implementation path is documented.
