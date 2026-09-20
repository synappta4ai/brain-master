# =============================================================================
# connect-gateway.ps1 — apunta el gateway local al worker de Colab (T4)
#
# Uso (después de ejecutar la celda 4 del notebook):
#   .\deploy\colab\connect-gateway.ps1 -WorkerHost "bore.pub:50051" `
#                                       -ArtifactBase "https://xxx.trycloudflare.com"
#
# Qué hace:
#   1. Reinicia el gateway con PYTHON_WORKER_HOST y BM_WORKER_ARTIFACT_BASE
#   2. Verifica /api/v1/health (worker_connected: true) y /api/v1/models
#   3. Crea un job de prueba y espera el artifact_url descargable
# =============================================================================
param(
  [Parameter(Mandatory = $true)][string]$WorkerHost,
  [Parameter(Mandatory = $true)][string]$ArtifactBase
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)   # brain-master/
$backend = Join-Path $repo "backend"

Write-Host "» Deteniendo gateway previo..." -ForegroundColor Cyan
$conn = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
if ($conn) { Stop-Process -Id $conn[0].OwningProcess -Force; Start-Sleep 2 }

Write-Host "» Recompilando gateway (evita el binario viejo sin features)…" -ForegroundColor Cyan
Push-Location $backend
go build -o gateway.exe .
if ($LASTEXITCODE -ne 0) { Pop-Location; throw "go build falló" }
Pop-Location

Write-Host "» Levantando gateway → worker $WorkerHost (artefactos: $ArtifactBase)" -ForegroundColor Cyan
$env:PYTHON_WORKER_HOST = $WorkerHost
$env:BM_WORKER_ARTIFACT_BASE = $ArtifactBase
Start-Process -WindowStyle Hidden -WorkingDirectory $backend -FilePath "$backend\gateway.exe" `
  -RedirectStandardOutput "$backend\gateway.log" -RedirectStandardError "$backend\gateway-err.log"

Start-Sleep 4
$health = Invoke-RestMethod "http://127.0.0.1:8080/api/v1/health" -TimeoutSec 10
Write-Host "  health: $($health.status) | worker_connected: $($health.worker_connected)" -ForegroundColor $(if ($health.worker_connected) { "Green" } else { "Red" })
if (-not $health.worker_connected) { throw "el gateway no pudo conectar al worker en $WorkerHost" }

$models = Invoke-RestMethod "http://127.0.0.1:8080/api/v1/models" -TimeoutSec 15
Write-Host "  catálogo: $($models.models.Count) modelos | device: $($models.device_name)" -ForegroundColor Green

Write-Host "» Job de prueba a través del túnel..." -ForegroundColor Cyan
$job = Invoke-RestMethod -Method Post "http://127.0.0.1:8080/api/v1/jobs/create" `
  -ContentType "application/json" -Body (@{
    mode = "image"; model = "SD-Tiny-Test"
    prompt = "first colab T4 generation"; steps = 4; width = 512; height = 512
  } | ConvertTo-Json)
Write-Host "  job: $($job.id)"

$deadline = (Get-Date).AddMinutes(4)
while ((Get-Date) -lt $deadline) {
  Start-Sleep 6
  $j = Invoke-RestMethod "http://127.0.0.1:8080/api/v1/jobs/detail?id=$($job.id)" -TimeoutSec 10
  if ($j.status -in @("COMPLETED", "FAILED", "CANCELLED")) { break }
}

Write-Host "  estado final: $($j.status) | progreso: $($j.progress)%" -ForegroundColor $(if ($j.status -eq "COMPLETED") { "Green" } else { "Red" })
if ($j.artifact_url) {
  try {
    $head = Invoke-WebRequest $j.artifact_url -Method Head -TimeoutSec 20
    Write-Host "  artifact_url OK ($($head.Headers['Content-Type'])) → $($j.artifact_url)" -ForegroundColor Green
  } catch { Write-Host "  artifact_url NO descargable: $_" -ForegroundColor Yellow }
} else {
  Write-Host "  sin artifact_url (¿BM_WORKER_ARTIFACT_BASE correcto?)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "✓ Front en http://localhost:4200 — los nuevos jobs usan la T4 de Colab" -ForegroundColor Cyan
