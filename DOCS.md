Two things: **handoff doc** and **.gitignore**.

---

# 1 Frontend Handoff Document

Give this as a README or PDF. No explanations beyond this.

---

## Project: FailSafe Backend API

---

## Setup (MANDATORY)

### Step 1 — Install Docker

---

### Step 2 — Run system

```bash
docker-compose up
```

Backend runs on:

```text
http://localhost:8080
```

---

## Test Services (for simulation)

If not already running:

```bash
docker run -d --name svc-a nginx
docker run -d --name svc-b nginx
docker run -d --name svc-c nginx
```

Connect them:

```bash
docker network connect failsafe_default svc-a
docker network connect failsafe_default svc-b
docker network connect failsafe_default svc-c
```

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

---

### 4) Stop Experiment

```text
POST /experiment/stop?id=<id>
```

---

## UI Expectations

Frontend should build:

```text
- Experiment form
- Live status view
- Metrics dashboard
- Dependency graph visualization
```

---

## Notes

```text
- Always send JSON
- Use service names (svc-a, svc-b, svc-c), not localhost
- ID is required for all GET endpoints
```

