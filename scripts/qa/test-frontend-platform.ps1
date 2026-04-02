param(
    [string]$ControllerBaseUrl = "http://localhost:8000",
    [string]$FrontendBaseUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"

function Invoke-CurlJson {
    param(
        [string]$Method,
        [string]$Url,
        [string]$Body = ""
    )

    if ($Body -ne "") {
        $raw = curl.exe -sS -X $Method $Url -H "Content-Type: application/json" -d $Body
    } else {
        $raw = curl.exe -sS -X $Method $Url
    }

    if ($LASTEXITCODE -ne 0) {
        throw "curl failed for $Method $Url"
    }

    return $raw
}

$startBody = @"
{
  "fault_type": "latency",
  "target_type": "frontend",
  "duration_seconds": 20,
  "frontend_run": {
    "base_url": "$FrontendBaseUrl",
    "metrics_endpoint": "$ControllerBaseUrl/frontend/metrics",
    "target_urls": ["/api/users", "/api/orders"]
  }
}
"@

Write-Host "[frontend] start experiment"
$startRaw = Invoke-CurlJson -Method "POST" -Url "$ControllerBaseUrl/experiments/frontend/start" -Body $startBody
$start = $startRaw | ConvertFrom-Json
if (-not $start.id) {
    throw "frontend start response missing id: $startRaw"
}

$id = $start.id
Write-Host "[frontend] experiment id: $id"

Write-Host "[frontend] push synthetic collector batch"
$batchBody = @"
{
  "metrics": [
    {
      "experiment_id": "$id",
      "phase": "baseline",
      "page": "/",
      "metrics": {
        "lcp": 1200,
        "cls": 0.04,
        "inp": 90,
        "long_tasks": 1,
        "errors": 0,
        "unhandled_rejections": 0
      },
      "api_calls": [
        { "url": "/api/users/1", "duration": 120, "status": 200 }
      ],
      "timestamp": 1712000000000
    }
  ]
}
"@

$ingestRaw = Invoke-CurlJson -Method "POST" -Url "$ControllerBaseUrl/frontend/metrics" -Body $batchBody
if ($ingestRaw -notmatch "metrics received") {
    throw "frontend ingest unexpected response: $ingestRaw"
}

Write-Host "[frontend] status"
$statusRaw = Invoke-CurlJson -Method "GET" -Url "$ControllerBaseUrl/experiments/frontend/status?id=$id"
$status = $statusRaw | ConvertFrom-Json
if (-not $status.experiment -and -not $status.id) {
    throw "frontend status missing payload: $statusRaw"
}

Write-Host "[frontend] metrics"
$metricsRaw = Invoke-CurlJson -Method "GET" -Url "$ControllerBaseUrl/experiments/frontend/metrics?id=$id"
$metrics = $metricsRaw | ConvertFrom-Json
if (-not $metrics.frontend) {
    throw "frontend metrics missing frontend samples: $metricsRaw"
}

Write-Host "[frontend] stop"
$stopRaw = Invoke-CurlJson -Method "POST" -Url "$ControllerBaseUrl/experiments/frontend/stop?id=$id"
if ($stopRaw -notmatch "experiment stopped") {
    throw "frontend stop unexpected response: $stopRaw"
}

Write-Host "[frontend] PASS"
