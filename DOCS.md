# FailSafe Backend API

---

## Setup

### Prerequisites

* Docker installed

---

### Run the system

```bash
docker-compose up --build
```

Backend will be available at:

```text
http://localhost:8080
```

---

## Architecture (for context)

```text
backend → controls docker → injects faults into svc-a/b/c  
postgres → stores experiments + metrics  
svc-a/b/c → test services (nginx)
```

All services are started automatically via docker-compose.

---

## API Endpoints

---

### 1) Start Experiment

```text
POST /experiment/start
```

#### Request

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

#### Response

```json
{
  "id": "experiment-id",
  "state": "running"
}
```

---

### 2) Get Experiment

```text
GET /experiment/get?id=<id>
```

---

### 3) Get Metrics

```text
GET /experiment/metrics?id=<id>
```

#### Sample Response

```json
{
  "experiment_state": "completed",
  "system_severity": "systemic",
  "blast_radius_percent": 100,
  "cascade_depth": 3,
  "endpoints": {
    "http://svc-a": {
      "latency": {
        "p95_ms": 2000,
        "avg_ms": 600
      },
      "errors": {
        "rate_percent": 20
      },
      "degraded": true
    }
  }
}
```

---

### 4) Stop Experiment

```text
POST /experiment/stop?id=<id>
```

---

## Important Notes

```text
- Always send Content-Type: application/json
- Do NOT use localhost inside requests
- Use service names: svc-a, svc-b, svc-c
- Experiment ID is required for all GET/STOP endpoints
```

---

## UI Requirements

Frontend should implement:

```text
- Experiment creation form
- Real-time experiment status
- Metrics dashboard (latency, errors, degradation)
- Dependency graph visualization
```

---

## One Command Workflow

```bash
docker-compose up --build
```

This starts:

```text
- backend
- postgres
- svc-a
- svc-b
- svc-c
```

No manual setup required.

---

## Troubleshooting

```text
If experiment fails:
- ensure docker is running
- ensure all containers are up (docker ps)
- ensure correct API payload
```

---

## Summary

```text
Single command → full system  
Frontend consumes API → builds UI  
Backend handles orchestration + metrics
```
