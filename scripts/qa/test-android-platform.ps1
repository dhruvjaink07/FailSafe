param(
    [string]$ControllerBaseUrl = "http://localhost:8000",
    [string]$APKPath = "uploads/apks/app.apk",
    [string]$AVDName = "Pixel_8a"
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

if (-not (Test-Path $APKPath)) {
    throw "APK not found at path: $APKPath"
}

Write-Host "[android] upload apk"
$uploadRaw = curl.exe -sS -X POST "$ControllerBaseUrl/upload/apk" -F "file=@$APKPath"
if ($LASTEXITCODE -ne 0) {
    throw "curl failed for APK upload"
}
$apk = $uploadRaw | ConvertFrom-Json
if (-not $apk.apk -or -not $apk.package) {
    throw "APK upload response missing apk/package: $uploadRaw"
}

$startBody = @"
{
  "fault_type": "kill_app",
  "targets": ["$($apk.package)"],
  "target_type": "android",
  "observation_type": "android",
  "duration_seconds": 30,
  "apk": "$($apk.apk)",
  "android_run": {
    "avd_name": "$AVDName",
    "headless": true,
    "reset_app_state": true
  },
  "expected": {
    "running": true,
    "not_crash": true,
    "not_anr": true,
    "should_recover": true
  }
}
"@

Write-Host "[android] start experiment"
$startRaw = Invoke-CurlJson -Method "POST" -Url "$ControllerBaseUrl/experiments/android/start" -Body $startBody
$start = $startRaw | ConvertFrom-Json
if (-not $start.id) {
    throw "android start response missing id: $startRaw"
}

$id = $start.id
Write-Host "[android] experiment id: $id"

Write-Host "[android] status"
$statusRaw = Invoke-CurlJson -Method "GET" -Url "$ControllerBaseUrl/experiments/android/status?id=$id"
$status = $statusRaw | ConvertFrom-Json
if (-not $status.state) {
    throw "android status missing state: $statusRaw"
}

Write-Host "[android] metrics"
$metricsRaw = Invoke-CurlJson -Method "GET" -Url "$ControllerBaseUrl/experiments/android/metrics?id=$id"
$metrics = $metricsRaw | ConvertFrom-Json
if (-not ($metrics.health -or $metrics.recovery -or $metrics.stability)) {
    throw "android metrics missing expected keys: $metricsRaw"
}

Write-Host "[android] stop"
$stopRaw = Invoke-CurlJson -Method "POST" -Url "$ControllerBaseUrl/experiments/android/stop?id=$id"
if ($stopRaw -notmatch "experiment stopped") {
    throw "android stop unexpected response: $stopRaw"
}

Write-Host "[android] PASS"
