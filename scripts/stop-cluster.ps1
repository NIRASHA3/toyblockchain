param(
    [switch]$Clean
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = (Resolve-Path (Join-Path $ScriptDir "..")).Path
$ClusterDir = Join-Path $RootDir ".cluster"
$PidFile = Join-Path $ClusterDir "pids.json"

if (-not (Test-Path $PidFile)) {
    Write-Host "No cluster PID file found at: $PidFile"
    Write-Host "Nothing to stop."

    if ($Clean -and (Test-Path $ClusterDir)) {
        Write-Host "Clean enabled: removing .cluster directory"
        Remove-Item $ClusterDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    exit 0
}

$Processes = Get-Content $PidFile -Raw | ConvertFrom-Json

if ($null -eq $Processes) {
    Write-Host "PID file was empty. Removing it."
    Remove-Item $PidFile -Force -ErrorAction SilentlyContinue

    if ($Clean -and (Test-Path $ClusterDir)) {
        Write-Host "Clean enabled: removing .cluster directory"
        Remove-Item $ClusterDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    exit 0
}

Write-Host "Stopping Toy Blockchain cluster..."

foreach ($ProcessInfo in @($Processes)) {
    $PidValue = [int]$ProcessInfo.pid

    try {
        $Process = Get-Process -Id $PidValue -ErrorAction Stop
        Stop-Process -Id $PidValue -Force
        Write-Host "Stopped node $($ProcessInfo.node), PID $PidValue"
    }
    catch {
        Write-Host "Node $($ProcessInfo.node), PID $PidValue was not running"
    }
}

Remove-Item $PidFile -Force -ErrorAction SilentlyContinue

if ($Clean -and (Test-Path $ClusterDir)) {
    Write-Host "Clean enabled: removing .cluster directory"
    Remove-Item $ClusterDir -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "Cluster stopped."
