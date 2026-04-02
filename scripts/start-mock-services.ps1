param(
    [string]$PythonExe = "python",
    [switch]$Stop
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$stateDir = Join-Path $PSScriptRoot "experiment_results"
$pidFile = Join-Path $stateDir "mock-services.pids.json"

if (-not (Test-Path $stateDir)) {
    New-Item -ItemType Directory -Path $stateDir | Out-Null
}

function Get-ServiceSpecs {
    return @(
        @{
            Name = "notification-service"
            Dir = "test/mock-services/notification-service"
            Env = @{}
        },
        @{
            Name = "inventory-service"
            Dir = "test/mock-services/inventory-service"
            Env = @{}
        },
        @{
            Name = "payment-service"
            Dir = "test/mock-services/payment-service"
            Env = @{}
        },
        @{
            Name = "recommendation-service"
            Dir = "test/mock-services/recommendation-service"
            Env = @{
                INVENTORY_SERVICE_URL = "http://localhost:8084"
            }
        },
        @{
            Name = "shipping-service"
            Dir = "test/mock-services/shipping-service"
            Env = @{
                NOTIFICATION_SERVICE_URL = "http://localhost:8086"
            }
        },
        @{
            Name = "user-service"
            Dir = "test/mock-services/user-service"
            Env = @{
                RECOMMENDATION_SERVICE_URL = "http://localhost:8087"
            }
        },
        @{
            Name = "order-service"
            Dir = "test/mock-services/order-service"
            Env = @{
                PAYMENT_SERVICE_URL = "http://localhost:8083"
                INVENTORY_SERVICE_URL = "http://localhost:8084"
                SHIPPING_SERVICE_URL = "http://localhost:8085"
            }
        },
        @{
            Name = "api-gateway"
            Dir = "test/mock-services/api-gateway"
            Env = @{
                USER_SERVICE_URL = "http://localhost:8081"
                ORDER_SERVICE_URL = "http://localhost:8082"
            }
        }
    )
}

function Stop-MockServices {
    if (-not (Test-Path $pidFile)) {
        Write-Host "No PID file found. Nothing to stop."
        return
    }

    $records = Get-Content -Raw -Path $pidFile | ConvertFrom-Json
    foreach ($record in $records) {
        try {
            $proc = Get-Process -Id $record.pid -ErrorAction Stop
            Stop-Process -Id $proc.Id -Force
            Write-Host ("Stopped {0} (PID {1})" -f $record.name, $record.pid)
        } catch {
            Write-Host ("Process already stopped or missing for {0} (PID {1})" -f $record.name, $record.pid)
        }
    }

    Remove-Item -Path $pidFile -Force -ErrorAction SilentlyContinue
}

if ($Stop) {
    Stop-MockServices
    return
}

$specs = Get-ServiceSpecs
$started = @()

foreach ($spec in $specs) {
    $servicePath = Join-Path $repoRoot $spec.Dir
    if (-not (Test-Path $servicePath)) {
        throw "Service directory not found: $servicePath"
    }

    $envCommands = @()
    foreach ($kv in $spec.Env.GetEnumerator()) {
        $envCommands += ('$env:{0} = "{1}"' -f $kv.Key, $kv.Value)
    }

    $cmdParts = @()
    $cmdParts += "Set-Location '$servicePath'"
    if ($envCommands.Count -gt 0) {
        $cmdParts += ($envCommands -join "; ")
    }
    $cmdParts += "& '$PythonExe' app.py"
    $cmd = $cmdParts -join "; "

    $proc = Start-Process -FilePath "powershell" -ArgumentList @("-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $cmd) -WindowStyle Hidden -PassThru

    $started += [PSCustomObject]@{
        name = $spec.Name
        pid  = $proc.Id
        dir  = $spec.Dir
    }

    Write-Host ("Started {0} (PID {1})" -f $spec.Name, $proc.Id)
}

$started | ConvertTo-Json | Set-Content -Path $pidFile

Write-Host ""
Write-Host "Mock services launched."
Write-Host "Health checks:"
Write-Host "  http://localhost:8080/health"
Write-Host "  http://localhost:8081/health"
Write-Host "  http://localhost:8082/health"
Write-Host "  http://localhost:8083/health"
Write-Host "  http://localhost:8084/health"
Write-Host "  http://localhost:8085/health"
Write-Host "  http://localhost:8086/health"
Write-Host "  http://localhost:8087/health"
Write-Host ""
Write-Host "To stop all services:"
Write-Host "  powershell -ExecutionPolicy Bypass -File .\scripts\start-mock-services.ps1 -Stop"