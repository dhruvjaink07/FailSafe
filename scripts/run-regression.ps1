Param()

$ErrorActionPreference = 'Stop'

# Regression runner: runs an experiment matrix against the backend controller

$base = 'http://localhost:8000'
$apiKey = 'fs_dev_engineer_ci-key_6099171d2c319b9951ff102386b073bc74896c26a037b1b213caafea7f54a4e7'
$hdr = @{ 'x-api-key' = $apiKey }

$resultsDir = Join-Path -Path (Get-Location) -ChildPath 'experiments\results'
New-Item -ItemType Directory -Force -Path $resultsDir | Out-Null

Write-Output "Saving results to: $resultsDir"

# Safe mode: avoid destructive faults (kill) unless explicitly allowed
$allowDestructive = $false

# Define matrix: fault -> durations/intensities
$matrix = @()

# Observed endpoints (mock services)
$observed = @(
    'http://localhost:8080/api/users/1',
    'http://localhost:8080/api/orders/10',
    'http://localhost:8081/users/1',
    'http://localhost:8082/orders/10',
    'http://localhost:8083/payments/10',
    'http://localhost:8084/inventory/A1',
    'http://localhost:8085/shipping/10',
    'http://localhost:8086/notifications/10',
    'http://localhost:8087/recommendations/1'
)

# Kill (simple stop/start) - only include if allowed
if ($allowDestructive) {
    $matrix += @{ faultType='kill'; duration=20 }
    $matrix += @{ faultType='kill'; duration=40 }
}

# CPU stress
$matrix += @{ faultType='cpu_stress'; duration=30; maxIntensity=40 }
$matrix += @{ faultType='cpu_stress'; duration=45; maxIntensity=80 }

# Memory stress
$matrix += @{ faultType='memory_stress'; duration=25; maxIntensity=40 }
$matrix += @{ faultType='memory_stress'; duration=45; maxIntensity=70 }

# Network faults
$matrix += @{ faultType='network_delay'; duration=30; maxIntensity=40 }
$matrix += @{ faultType='packet_loss'; duration=30; maxIntensity=40 }

function Start-Experiment($payload) {
    $body = $payload | ConvertTo-Json -Depth 8
    try {
        $resp = Invoke-RestMethod -Uri "$base/experiments/backend/start" -Method Post -Body $body -Headers $hdr -ContentType 'application/json'
        return $resp
    } catch {
        Write-Error "Start failed: $_"
        return $null
    }
}

function Poll-Status($id, $timeoutSeconds = 180) {
    $end = (Get-Date).AddSeconds($timeoutSeconds)
    while ((Get-Date) -lt $end) {
        try {
            $s = Invoke-RestMethod -Uri ("$base/experiments/backend/status?id={0}" -f $id) -Method Get
            $state = $s.experiment.state
            $phase = $s.experiment.phase
            Write-Output "$id state=$state phase=$phase"
            if ($state -eq 'completed' -or $state -eq 'failed') { return $s }
        } catch {
            Write-Output ("status poll error for {0}: {1}" -f $id, $_)
        }
        Start-Sleep -Seconds 2
    }
    throw "timeout waiting for experiment $id"
}

function Fetch-And-Save($id, $outPrefix) {
    # metrics
    try {
        $m = Invoke-RestMethod -Uri ("$base/experiments/backend/metrics?id={0}" -f $id) -Method Get -Headers $hdr
        $m | ConvertTo-Json -Depth 10 | Out-File -FilePath ($outPrefix + '-metrics.json') -Encoding UTF8
    } catch {
        Write-Output ("No metrics for {0}: {1}" -f $id, $_)
    }

    # logs
    try {
        $l = Invoke-RestMethod -Uri ("$base/experiments/backend/logs?id={0}" -f $id) -Method Get -Headers $hdr
        if ($l -is [string]) { $l | Out-File -FilePath ($outPrefix + '-logs.txt') -Encoding UTF8 }
        else { ($l | ConvertTo-Json -Depth 6) | Out-File -FilePath ($outPrefix + '-logs.json') -Encoding UTF8 }
    } catch {
        Write-Output ("No logs for {0}: {1}" -f $id, $_)
    }
}

$summary = @()

foreach ($entry in $matrix) {
    $payload = @{}
    $payload.faultType = $entry.faultType
    $payload.targets = @('user-service')
    $payload.targetType = 'docker'
    $payload.observationType = 'http'
    $payload.observedEndpoints = $observed
    $payload.duration = $entry.duration
    if ($entry.ContainsKey('maxIntensity')) { $payload.maxIntensity = $entry.maxIntensity }

    Write-Output "Starting experiment: $($payload | ConvertTo-Json -Compress)"
    $resp = Start-Experiment $payload
    if ($null -eq $resp) { $summary += @{ payload = $payload; error = 'start_failed' }; continue }

    $id = $resp.id
    $outPrefix = Join-Path $resultsDir $id
    $resp | ConvertTo-Json -Depth 6 | Out-File -FilePath ($outPrefix + '-start.json') -Encoding UTF8

    try {
        $status = Poll-Status $id 240
        $status | ConvertTo-Json -Depth 10 | Out-File -FilePath ($outPrefix + '-status.json') -Encoding UTF8
        Fetch-And-Save $id $outPrefix
        $summary += @{ id = $id; payload = $payload; state = $status.experiment.state; phase = $status.experiment.phase }
    } catch {
        Write-Output "Experiment $id failed or timed out: $_"
        $summary += @{ id = $id; payload = $payload; error = 'timeout_or_failed'; details = $_.ToString() }
    }

    Start-Sleep -Seconds 2
}

# summary export
$summary | ConvertTo-Json -Depth 10 | Out-File -FilePath (Join-Path $resultsDir 'summary.json') -Encoding UTF8

Write-Output "Regression run complete. Summary written to $(Join-Path $resultsDir 'summary.json')"
