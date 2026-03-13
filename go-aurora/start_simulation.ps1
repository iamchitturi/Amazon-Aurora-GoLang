

# Check if Go is in PATH, if not try default installation path
$GO = "go"
try {
    & $GO version | Out-Null
} catch {
    $GO = "C:\Program Files\Go\bin\go.exe"
    if (-not (Test-Path $GO)) {
        Write-Host "CRITICAL: Go is not installed or not found at $GO" -ForegroundColor Red
        exit
    }
}

Write-Host "--- Diagnostics ---" -ForegroundColor Gray
Write-Host "Using Go: $((& $GO version))" -ForegroundColor Gray
Write-Host "-------------------" -ForegroundColor Gray

Get-ChildItem -Filter "storage_node_*.json" | Remove-Item -Force -ErrorAction SilentlyContinue


Write-Host "Cleaning up any processes holding ports 5001-5004..." -ForegroundColor Gray
for ($p = 5001; $p -le 5004; $p++) {
    $item = Get-NetTCPConnection -LocalPort $p -ErrorAction SilentlyContinue
    if ($item) {
        Stop-Process -Id $item.OwningProcess -Force -ErrorAction SilentlyContinue
    }
}
Start-Sleep -Seconds 1

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "   Amazon Aurora Simulation Starter (PowerShell)" -ForegroundColor Cyan
Write-Host "===============================================" -ForegroundColor Cyan

Write-Host "Starting 4 Aurora Nodes in background..." -ForegroundColor Yellow
$jobs = @()
for ($i = 1; $i -le 4; $i++) {
    Write-Host "Starting Node $i..."
    $jobs += Start-Process -FilePath $GO -ArgumentList "run node.go $i" -NoNewWindow -PassThru
}

Write-Host "Waiting 5 seconds for leader election..." -ForegroundColor Yellow
Start-Sleep -Seconds 5

Write-Host "Starting Client Simulator..." -ForegroundColor Green
& $GO run client.go

Write-Host "Cleaning up node processes..." -ForegroundColor Red
foreach ($job in $jobs) {
    Stop-Process -Id $job.Id -Force -ErrorAction SilentlyContinue
}
