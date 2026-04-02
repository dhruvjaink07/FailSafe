# QA Curl Automation Scripts

These scripts run platform lifecycle checks using `curl.exe` and verify key outputs.

## Prerequisites

- Controller running on `http://localhost:8000`
- Mock microservices running (`docker-compose.test.yml`) for backend and frontend checks
- Android prerequisites running for Android checks

## Run Commands

Backend:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\qa\test-backend-platform.ps1
```

Android (with APK path):

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\qa\test-android-platform.ps1 -APKPath "uploads/apks/app.apk"
```

Frontend:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\qa\test-frontend-platform.ps1
```