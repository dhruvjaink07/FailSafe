# FailSafe Welcomes you

## Why this structure?

> cmd/ → entry point binaries

> internal/ → private app logic

> models/ → structs

> orchestrator/ → experiment lifecycle logic

> monitoring/ → metrics collectors

> handlers/ → HTTP handlers

### Command to install Postgres
```
docker run -d --name failsafe-postgres -e POSTGRES_USER=failsafe -e POSTGRES_PASSWORD=failsafe -e POSTGRES_DB=failsafe -p 5432:5432 postgres:15
```
### Command to initiate local postgres
```
docker exec -it failsafe-postgres psql -U failsafe -d failsafe
```

### Docker Setup and Management

Use the Docker helper script to manage containers:

```powershell
# Check Docker status and list containers
./scripts/docker-helper.ps1

# Ensure Docker is running (auto-starts if needed)
./scripts/docker-helper.ps1 -Start

# Interactive container management (logs, restart, stop)
./scripts/docker-helper.ps1 -Interactive
```

### to build the docker image
```
docker-compose up --build
```

### Testing & Documentation

For comprehensive backend testing workflows, see:
- [docs/BACKEND_TESTING_GUIDE.md](docs/BACKEND_TESTING_GUIDE.md) - Complete testing guide
- [docs/ANDROID_EXECUTOR_QUICKSTART.md](docs/ANDROID_EXECUTOR_QUICKSTART.md) - Quick start guide
- [docs/ANDROID_EXECUTOR_ARCHITECTURE.md](docs/ANDROID_EXECUTOR_ARCHITECTURE.md) - Architecture details

### Android 
Required commands (must exist)
App control
- ForceStop(pkg string)
- Launch(pkg string, activity string)
- ClearData(pkg string)
System control
- DisableWifi()
- EnableWifi()
- DisableData()
- EnableData()
Device state
- Rotate(mode int)
- SetBattery(level int)
Reboot()
Info extraction
- GetTopActivity()
- GetMemory(pkg string)
- GetCPU(pkg string)