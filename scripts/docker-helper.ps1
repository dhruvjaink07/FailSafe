#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Docker utility script for FailSafe backend testing
    
.DESCRIPTION
    Lists running/stopped containers and ensures Docker is running on Windows
    
.PARAMETER Start
    Starts Docker Desktop if not running
    
.PARAMETER Status
    Shows Docker and container status only
    
.PARAMETER Interactive
    Interactive mode to select and manage containers
    
.EXAMPLE
    ./docker-helper.ps1 -Start
    ./docker-helper.ps1 -Status
    ./docker-helper.ps1 -Interactive
#>

param(
    [switch]$Start,
    [switch]$Status,
    [switch]$Interactive
)

# Prefer compose in deployments/docker if present
$composeCandidates = @(
    "deployments/docker/docker-compose.yml",
    "docker-compose.yml"
)
$composeFile = $composeCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1

function Ensure-Docker-Running {
    <#
    .SYNOPSIS
    Ensures Docker is running on Windows
    #>
    Write-Host "Checking Docker status..." -ForegroundColor Cyan
    
    try {
        $dockerStatus = docker ps 2>&1
        Write-Host "✓ Docker is running" -ForegroundColor Green
        return $true
    }
    catch {
        Write-Host "✗ Docker is not accessible" -ForegroundColor Red
    }
    
    Write-Host "Attempting to start Docker Desktop..." -ForegroundColor Yellow
    
    $dockerDesktop = "C:\Program Files\Docker\Docker\Docker Desktop.exe"
    if (Test-Path $dockerDesktop) {
        & $dockerDesktop
        Write-Host "Docker Desktop started. Waiting 10 seconds..." -ForegroundColor Yellow
        Start-Sleep -Seconds 10
        
        try {
            $null = docker ps
            Write-Host "✓ Docker is now running" -ForegroundColor Green
            return $true
        }
        catch {
            Write-Host "✗ Docker still not responding" -ForegroundColor Red
            return $false
        }
    }
    else {
        Write-Host "✗ Docker Desktop not found at: $dockerDesktop" -ForegroundColor Red
        Write-Host "  Please install Docker Desktop from: https://www.docker.com/products/docker-desktop" -ForegroundColor Yellow
        return $false
    }
}

function List-Containers {
    <#
    .SYNOPSIS
    Lists all Docker containers with status
    #>
    Write-Host "`n=== FailSafe Docker Containers ===" -ForegroundColor Cyan
    
    # Running containers
    Write-Host "`nRunning Containers:" -ForegroundColor Green
    $running = docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"
    if ($running) {
        Write-Host $running
    } else {
        Write-Host "  (none)" -ForegroundColor Gray
    }
    
    # Stopped containers
    Write-Host "`nStopped Containers:" -ForegroundColor Yellow
    $stopped = docker ps -a --filter "status=exited" --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
    if ($stopped) {
        Write-Host $stopped
    } else {
        Write-Host "  (none)" -ForegroundColor Gray
    }
}

function Show-Status {
    <#
    .SYNOPSIS
    Shows Docker status and container health
    #>
    Write-Host "`n=== Docker Status ===" -ForegroundColor Cyan
    
    try {
        $version = docker version --format "Client: {{.Client.Version}}, Server: {{.Server.Version}}"
        Write-Host "✓ $version" -ForegroundColor Green
    }
    catch {
        Write-Host "✗ Docker not accessible" -ForegroundColor Red
        return
    }
    
    # Container count
    $runningCount = docker ps -q | Measure-Object | Select-Object -ExpandProperty Count
    $totalCount = docker ps -a -q | Measure-Object | Select-Object -ExpandProperty Count
    
    Write-Host "  Running: $runningCount / Total: $totalCount" -ForegroundColor Cyan
    
    # FailSafe specific containers
    Write-Host "`n✓ FailSafe services:" -ForegroundColor Cyan
    
    $backend = docker ps --filter "name=failsafe-backend" --format "{{.Names}} ({{.Status}})"
    if ($backend) {
        Write-Host "  Backend: $backend" -ForegroundColor Green
    } else {
        Write-Host "  Backend: not running" -ForegroundColor Yellow
    }
    
    $postgres = docker ps --filter "name=failsafe-postgres" --format "{{.Names}} ({{.Status}})"
    if ($postgres) {
        Write-Host "  PostgreSQL: $postgres" -ForegroundColor Green
    } else {
        Write-Host "  PostgreSQL: not running" -ForegroundColor Yellow
    }
}

function Show-Interactive-Menu {
    <#
    .SYNOPSIS
    Interactive menu for container management
    #>
    while ($true) {
        Write-Host "`n=== FailSafe Docker Management ===" -ForegroundColor Cyan
        List-Containers
        
        Write-Host "`nOptions:" -ForegroundColor Cyan
        Write-Host "  1. Show logs (backend)" -ForegroundColor Gray
        Write-Host "  2. Show logs (postgres)" -ForegroundColor Gray
                Write-Host "  3. Restart containers (docker compose -f <compose-file> up)" -ForegroundColor Gray
                Write-Host "  4. Stop containers (docker compose -f <compose-file> down)" -ForegroundColor Gray
        Write-Host "  5. Refresh status" -ForegroundColor Gray
        Write-Host "  6. Exit" -ForegroundColor Gray
        
        $choice = Read-Host "`nSelect option (1-6)"
        
        switch ($choice) {
            "1" {
                Write-Host "`n=== Backend Logs ===" -ForegroundColor Cyan
                docker logs -f failsafe-backend 2>$null
            }
            "2" {
                Write-Host "`n=== PostgreSQL Logs ===" -ForegroundColor Cyan
                docker logs -f failsafe-postgres 2>$null
            }
            "3" {
                Write-Host "Restarting containers..." -ForegroundColor Yellow
                if ($composeFile) {
                    docker compose -f $composeFile up -d
                    Write-Host "✓ Containers restarted using $composeFile" -ForegroundColor Green
                    Start-Sleep -Seconds 3
                } else {
                    Write-Host "✗ docker-compose file not found (looked for deployments/docker/docker-compose.yml and docker-compose.yml)" -ForegroundColor Red
                }
            }
            "4" {
                Write-Host "Stopping containers..." -ForegroundColor Yellow
                if ($composeFile) {
                    docker compose -f $composeFile down
                    Write-Host "✓ Containers stopped using $composeFile" -ForegroundColor Green
                    Start-Sleep -Seconds 2
                } else {
                    Write-Host "✗ docker-compose file not found (looked for deployments/docker/docker-compose.yml and docker-compose.yml)" -ForegroundColor Red
                }
            }
            "5" {
                Show-Status
            }
            "6" {
                Write-Host "`nExiting..." -ForegroundColor Yellow
                exit 0
            }
            default {
                Write-Host "Invalid option. Please select 1-6." -ForegroundColor Red
            }
        }
    }
}

# Main execution
if ($Start) {
    Ensure-Docker-Running | Out-Null
    List-Containers
    Show-Status
}
elseif ($Status) {
    Show-Status
}
elseif ($Interactive) {
    Ensure-Docker-Running | Out-Null
    Show-Interactive-Menu
}
else {
    # Default: show status and list
    Ensure-Docker-Running | Out-Null
    List-Containers
    Show-Status
}
