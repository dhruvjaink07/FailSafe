# Frontend Testing Integration Guide

## Goal

Provide a predictable UI flow for uploading an APK, running a resilience scenario, polling status, and rendering final results.

## Recommended UI Flow

1. Upload APK.
2. Build experiment payload.
3. Start experiment.
4. Poll Android status endpoint every 2 seconds.
5. On terminal state, fetch full metrics.
6. Render summary, timeline, and validation reasons.

## Step 1: Upload APK

Endpoint:

```http
POST /upload/apk
```

Store returned fields for run:

- `apk` (id)
- `package`
- `activity`

## Step 2: Start Experiment

Endpoint:

```http
POST /experiment/start
```

Frontend payload should include:

- scenario steps
- expected validation contract
- `apk` upload id
- emulator mode (`android_run.headless`)

## Step 3: Poll Live Status

Endpoint:

```http
GET /experiment/android/status?id={experiment_id}
```

Poll interval:

- 2 seconds (recommended)

Stop polling when:

- `state` is `completed` or `failed`

## Step 4: Fetch Final Report

Endpoint:

```http
GET /experiment/metrics?id={experiment_id}
```

Use this for full charts and post-run report pages.

## Suggested UI Sections

### Run Header

- experiment id
- phase/state badges
- scenario label

### Health and Recovery

- failure type
- severity
- status
- recovered / auto_recovered / stable_recovered
- recovery time

### Timeline

- fault start
- first impact
- recovery
- replay hints list

### Validation

- configured or not
- passed state
- reasons list

### Samples and Aggregates

- metrics sample count
- uptime percent
- crash rate percent
- warning signals

## Frontend Contract Notes

- If `validation.passed` is `null`, treat as observational run (no strict expectation configured).
- `recovered=true` can still be `auto_recovered=false` when recovery needed external trigger.
- `stable_recovered=false` means the app came back but has not met stability window criteria.
