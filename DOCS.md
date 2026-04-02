# FailSafe API Documentation

This file is the entrypoint for API docs.

## HTML Docs Site

- Site overview: `docs/site/index.html`
- Backend API contract: `docs/site/backend-api.html`
- Frontend integration guide: `docs/site/frontend-testing.html`
- Postman testing guide: `docs/site/postman-testing.html`

## Documentation Pages

- Consolidated API reference: `API_DOCS.md`
- API overview and endpoint map: `docs/api/README.md`
- Backend API contract: `docs/api/backend-api.md`
- Frontend integration and polling flow: `docs/api/frontend-testing.md`
- Postman testing guide and request examples: `docs/api/postman-testing.md`

## Quick Start

1. Start infra and backend:

```bash
docker compose up -d postgres
go run cmd/controller/main.go
```

2. Open API docs:

```text
docs/site/index.html
```

3. Markdown source docs are kept under:

```text
docs/api/
```

## Non-Docker Manual Setup (Backend + Android)

Use this path if you do not want to run Docker for local backend and Android API testing.

### 1) Install prerequisites

- Go 1.22+
- PostgreSQL 15+
- Python 3.10+
- Android SDK (platform-tools, emulator, build-tools with `aapt`)

### 2) Configure environment variables

Create a local shell profile or `.env` with these required controller vars:

```powershell
$env:DB_URL = "postgres://failsafe:failsafe@localhost:5432/failsafe?sslmode=disable"
$env:CONFIG_PARAM_1 = "${env:LOCALAPPDATA}\Android\Sdk\platform-tools\adb.exe"
$env:CONFIG_PARAM_2 = "${env:LOCALAPPDATA}\Android\Sdk\emulator\emulator.exe"
$env:CONFIG_PARAM_3 = "local"
$env:CONFIG_PARAM_4 = "local"
$env:CONFIG_PARAM_5 = "local"

# Optional but recommended for APK metadata extraction:
$env:ANDROID_SDK_ROOT = "${env:LOCALAPPDATA}\Android\Sdk"
```

Notes:

- `CONFIG_PARAM_1` must point to `adb`.
- `CONFIG_PARAM_2` must point to the Android emulator binary.
- `CONFIG_PARAM_3..5` are required to be non-empty by startup validation; keep safe placeholders if unused.

### 3) Start PostgreSQL locally and initialize schema

Create database and user (example):

```sql
CREATE USER failsafe WITH PASSWORD 'failsafe';
CREATE DATABASE failsafe OWNER failsafe;
\c failsafe
\i internal/storage/schema.sql
```

### 4) Start mock backend services manually (no Docker)

Each service is a Flask app under `test/mock-services/*`.

Quick option (recommended):

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-mock-services.ps1
```

Stop all services started by the helper:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-mock-services.ps1 -Stop
```

Install deps once per service folder:

```powershell
python -m pip install -r test/mock-services/api-gateway/requirements.txt
python -m pip install -r test/mock-services/user-service/requirements.txt
python -m pip install -r test/mock-services/order-service/requirements.txt
python -m pip install -r test/mock-services/payment-service/requirements.txt
python -m pip install -r test/mock-services/inventory-service/requirements.txt
python -m pip install -r test/mock-services/shipping-service/requirements.txt
python -m pip install -r test/mock-services/notification-service/requirements.txt
python -m pip install -r test/mock-services/recommendation-service/requirements.txt
```

Run services in separate terminals with localhost dependency URLs:

```powershell
# notification-service (8086)
cd test/mock-services/notification-service; python app.py

# inventory-service (8084)
cd test/mock-services/inventory-service; python app.py

# payment-service (8083)
cd test/mock-services/payment-service; python app.py

# recommendation-service (8087)
cd test/mock-services/recommendation-service
$env:INVENTORY_SERVICE_URL = "http://localhost:8084"
python app.py

# shipping-service (8085)
cd test/mock-services/shipping-service
$env:NOTIFICATION_SERVICE_URL = "http://localhost:8086"
python app.py

# user-service (8081)
cd test/mock-services/user-service
$env:RECOMMENDATION_SERVICE_URL = "http://localhost:8087"
python app.py

# order-service (8082)
cd test/mock-services/order-service
$env:PAYMENT_SERVICE_URL = "http://localhost:8083"
$env:INVENTORY_SERVICE_URL = "http://localhost:8084"
$env:SHIPPING_SERVICE_URL = "http://localhost:8085"
python app.py

# api-gateway (8080)
cd test/mock-services/api-gateway
$env:USER_SERVICE_URL = "http://localhost:8081"
$env:ORDER_SERVICE_URL = "http://localhost:8082"
python app.py
```

### 5) Start controller

```powershell
go run cmd/controller/main.go
```

### 6) Android prerequisites check

```powershell
# verify adb
& $env:CONFIG_PARAM_1 devices

# verify emulator binary exists
Test-Path $env:CONFIG_PARAM_2

# optional: explicit aapt path when detection fails
$env:AAPT_PATH = "${env:LOCALAPPDATA}\Android\Sdk\build-tools\<version>\aapt.exe"
```

If `adb` is not available on a tester machine:

- Android tooling is validated at server startup. If unavailable, Android experiment start fails fast with `400` and `android preflight failed`.
- Backend and frontend flows are unaffected and can still be validated.
- Use platform-specific endpoints only for available targets:
	- backend: `/experiments/backend/*`
	- frontend: `/experiments/frontend/*`

### 7) Smoke checks

```powershell
curl.exe -sS http://localhost:8000/health
curl.exe -sS http://localhost:8080/health
```

## Build And Serve Pipeline

Quick launch commands:

```powershell
go run cmd/controller/main.go
powershell -ExecutionPolicy Bypass -File .\scripts\serve-docs.ps1 -Port 8090
```

Generate HTML pages from API markdown sources:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build-docs.ps1
```

Serve locally with hosted-style relative link behavior:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\serve-docs.ps1 -Port 8090
```

The build script verifies API-only output and fails if architecture-oriented content appears in generated site pages.