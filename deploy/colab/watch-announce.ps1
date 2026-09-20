# watch-announce.ps1 - arranca el watcher ntfy en una ventana propia
# (desacoplado: sobrevive al cierre de la terminal que lo lanza)
$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)   # brain-master/
$script = Join-Path $PSScriptRoot "watch-announce.py"

$py = (Get-Command py -ErrorAction SilentlyContinue)
$pyCmd = if ($py) { "py -3" } else { "python" }

Start-Process -WindowStyle Minimized cmd -ArgumentList "/k", "$pyCmd `"$script`""
Write-Host "watcher ON (ventana propia) - topic: bm-brain-master-tunnels-v1"
Write-Host "Ctrl+C en esa ventana para detenerlo"
