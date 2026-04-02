param(
    [string]$ControllerBaseUrl = "http://localhost:8000"
)

$ErrorActionPreference = "Stop"

function Invoke-CurlJson {
    param(
        [string]$Method,
        [string]$Url,
        [string]$Body = ""
    )

    if ($Body -ne "") {
        $tempPath = [System.IO.Path]::GetTempFileName() + ".json"
        Set-Content -Path $tempPath -Value $Body -NoNewline
        try {
            $raw = curl.exe -sS -X $Method $Url -H "Content-Type: application/json" --data-binary "@$tempPath"
        } finally {
            Remove-Item -Path $tempPath -Force -ErrorAction SilentlyContinue
        }
    } else {
        $raw = curl.exe -sS -X $Method $Url
    }

    if ($LASTEXITCODE -ne 0) {
        throw "curl failed for $Method $Url"
    }

    return $raw
}

function Parse-JsonOrThrow {
    param(
        [string]$Raw,
        [string]$Label
    )

    try {
        return $Raw | ConvertFrom-Json
    } catch {
        throw "$Label response was not valid JSON: $Raw"
    }
}

function Invoke-BackendTraffic {
    $urls = @(
        "http://localhost:8080/api/users/1",
        "http://localhost:8080/api/orders/10",
        "http://localhost:8082/orders/10",
        "http://localhost:8083/payments/10",
        "http://localhost:8084/inventory/A1",
        "http://localhost:8085/shipping/10",
        "http://localhost:8086/notifications/10",
        "http://localhost:8087/recommendations/1"
    )

    foreach ($url in $urls) {
        $null = curl.exe -sS $url
        if ($LASTEXITCODE -ne 0) {
            throw "backend traffic request failed: $url"
        }
    }
}

$startBody = @'
{
  "fault_type": "network_delay",
  "targets": ["order-service"],
  "target_type": "docker",
  "observation_type": "http",
  "observed_endpoints": [
    "http://api-gateway:8080/api/users/1",
    "http://api-gateway:8080/api/orders/10",
    "http://user-service:8081/users/1",
    "http://order-service:8082/orders/10",
    "http://payment-service:8083/payments/10",
    "http://inventory-service:8084/inventory/A1",
    "http://shipping-service:8085/shipping/10",
    "http://notification-service:8086/notifications/10",
    "http://recommendation-service:8087/recommendations/1"
  ],
  "duration_seconds": 30,
  "scenarios": [
    { "type": "network_delay", "at": 5, "duration_seconds": 10 },
    { "type": "packet_loss", "at": 18, "duration_seconds": 8 }
  ]
}
'@

Write-Host "[backend] start experiment"
$startRaw = Invoke-CurlJson -Method "POST" -Url "$ControllerBaseUrl/experiments/backend/start" -Body $startBody
$start = Parse-JsonOrThrow -Raw $startRaw -Label "backend start"
if (-not $start.id) {
    throw "backend start response missing id: $startRaw"
}

$id = $start.id
Write-Host "[backend] experiment id: $id"

Write-Host "[backend] generate traffic"
Invoke-BackendTraffic
Start-Sleep -Seconds 2

Write-Host "[backend] status"
$statusRaw = Invoke-CurlJson -Method "GET" -Url "$ControllerBaseUrl/experiments/backend/status?id=$id"
$status = Parse-JsonOrThrow -Raw $statusRaw -Label "backend status"
if (-not $status.experiment) {
    throw "backend status missing experiment payload: $statusRaw"
}

if ($status.experiment.id -ne $id) {
    throw "backend status id mismatch: expected $id but got $($status.experiment.id)"
}

if ($status.experiment.state -ne "running") {
    throw "backend status expected running state, got $($status.experiment.state)"
}

if (-not $status.aggregate_stats) {
    throw "backend status missing aggregate_stats: $statusRaw"
}

Write-Host "[backend] metrics"
$metricsRaw = Invoke-CurlJson -Method "GET" -Url "$ControllerBaseUrl/experiments/backend/metrics?id=$id"
$metrics = Parse-JsonOrThrow -Raw $metricsRaw -Label "backend metrics"
if (-not $metrics.endpoints) {
    throw "backend metrics missing endpoints: $metricsRaw"
}

if ($metrics.total_requests -lt 1) {
    Write-Warning "backend metrics returned no collected samples yet; structural fields are present, but the run did not record traffic"
}

if (-not $metrics.resilience_threshold) {
    throw "backend metrics missing resilience_threshold: $metricsRaw"
}

if (-not $metrics.timeline) {
    throw "backend metrics missing timeline: $metricsRaw"
}

Write-Host "[backend] stop"
$stopRaw = Invoke-CurlJson -Method "POST" -Url "$ControllerBaseUrl/experiments/backend/stop?id=$id"
if ($stopRaw -notmatch "experiment stopped") {
    throw "backend stop unexpected response: $stopRaw"
}

Write-Host "[backend] PASS"
