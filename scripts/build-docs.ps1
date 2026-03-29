param()

$ErrorActionPreference = "Stop"

Write-Host "Building API docs site from docs/api markdown..."
go run ./cmd/docs build
if ($LASTEXITCODE -ne 0) {
  throw "docs build failed"
}

$required = @(
  "docs/site/index.html",
  "docs/site/backend-api.html",
  "docs/site/frontend-testing.html",
  "docs/site/postman-testing.html"
)

foreach ($file in $required) {
  if (-not (Test-Path $file)) {
    throw "missing generated file: $file"
  }
}

$apiSignals = @(
  "/experiment/start",
  "/experiment/metrics",
  "/upload/apk"
)

foreach ($signal in $apiSignals) {
  $found = Select-String -Path "docs/site/*.html" -Pattern ([regex]::Escape($signal))
  if (-not $found) {
    throw "API docs verification failed: missing signal $signal"
  }
}

$disallowed = @(
  "Why this structure",
  "internal/",
  "architecture"
)

foreach ($term in $disallowed) {
  $bad = Select-String -Path "docs/site/*.html" -Pattern ([regex]::Escape($term))
  if ($bad) {
    throw "API-only verification failed: disallowed content found -> $term"
  }
}

Write-Host "Generated and verified docs/site/*.html"
