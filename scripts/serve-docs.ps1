param(
  [int]$Port = 8090
)

$ErrorActionPreference = "Stop"

Write-Host "Building docs before serve..."
& "$PSScriptRoot/build-docs.ps1"

Write-Host "Serving docs on http://localhost:$Port"
go run ./cmd/docs serve -port $Port
