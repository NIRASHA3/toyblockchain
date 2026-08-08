param(
    [int]$NodeCount = 3,
    [int]$BasePort = 8081,
    [int]$Difficulty = 1,
    [int]$RetargetInterval = 0,
    [switch]$Reset,
    [switch]$NoBuild
)

$ErrorActionPreference = "Stop"

if ($NodeCount -lt 3) {
    throw "NodeCount must be at least 3 for the assessment cluster."
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RootDir = (Resolve-Path (Join-Path $ScriptDir "..")).Path
$ClusterDir = Join-Path $RootDir ".cluster"
$ExePath = Join-Path $RootDir "toychain.exe"
$PidFile = Join-Path $ClusterDir "pids.json"

Write-Host "Toy Blockchain local cluster launcher"
Write-Host "Root: $RootDir"
Write-Host "Nodes: $NodeCount"
Write-Host "Base port: $BasePort"
Write-Host "Difficulty: $Difficulty"
Write-Host "Retarget interval: $RetargetInterval"
Write-Host ""

if ($Reset -and (Test-Path $ClusterDir)) {
    Write-Host "Reset enabled: removing existing .cluster directory"
    Remove-Item $ClusterDir -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $ClusterDir | Out-Null

if (-not $NoBuild) {
    Write-Host "Building toychain.exe..."
    Push-Location $RootDir
    try {
        go build -o toychain.exe ./cmd/toychain
    }
    finally {
        Pop-Location
    }
}

if (-not (Test-Path $ExePath)) {
    throw "toychain.exe was not found. Run without -NoBuild first, or build manually using: go build -o toychain.exe ./cmd/toychain"
}

Write-Host "Preparing node state files..."

for ($i = 1; $i -le $NodeCount; $i++) {
    $StatePath = Join-Path $ClusterDir "node$i.json"

    if ($Reset -or -not (Test-Path $StatePath)) {
        Write-Host "Initialising node $i state: $StatePath"
        & $ExePath -data $StatePath -difficulty $Difficulty -retarget-interval $RetargetInterval init -force
    }
    else {
        Write-Host "Keeping existing node $i state: $StatePath"
    }
}

Write-Host ""
Write-Host "Starting nodes..."

$Processes = @()

for ($i = 1; $i -le $NodeCount; $i++) {
    $Port = $BasePort + $i - 1
    $StatePath = Join-Path $ClusterDir "node$i.json"

    $Peers = @()
    for ($j = 1; $j -le $NodeCount; $j++) {
        if ($j -eq $i) {
            continue
        }

        $PeerPort = $BasePort + $j - 1
        $Peers += "http://127.0.0.1:$PeerPort"
    }

    $PeerList = $Peers -join ","
    $NodeURL = "http://127.0.0.1:$Port"

    $Command = @"
Set-Location -LiteralPath '$RootDir'
& '$ExePath' -data '$StatePath' -difficulty $Difficulty -retarget-interval $RetargetInterval node -addr 127.0.0.1:$Port -peers '$PeerList'
"@

    $Process = Start-Process `
        -FilePath "powershell.exe" `
        -ArgumentList @("-NoExit", "-ExecutionPolicy", "Bypass", "-Command", $Command) `
        -WorkingDirectory $RootDir `
        -PassThru

    $Processes += [PSCustomObject]@{
        node  = $i
        pid   = $Process.Id
        port  = $Port
        url   = $NodeURL
        state = $StatePath
        peers = $PeerList
    }

    Write-Host "Node $i started"
    Write-Host "  PID:   $($Process.Id)"
    Write-Host "  URL:   $NodeURL"
    Write-Host "  Peers: $PeerList"
}

$Processes | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 $PidFile

Write-Host ""
Write-Host "Cluster PID file written to: $PidFile"
Write-Host ""

Start-Sleep -Seconds 2

Write-Host "Quick health check:"

foreach ($ProcessInfo in $Processes) {
    try {
        $Health = Invoke-RestMethod "$($ProcessInfo.url)/health" -TimeoutSec 3
        $Status = Invoke-RestMethod "$($ProcessInfo.url)/status" -TimeoutSec 3

        Write-Host "Node $($ProcessInfo.node): health=$($Health.status), height=$($Status.height), pending=$($Status.pending_count), peers=$($Status.peer_count)"
    }
    catch {
        Write-Host "Node $($ProcessInfo.node): health check failed: $($_.Exception.Message)"
    }
}

Write-Host ""
Write-Host "Cluster started."
Write-Host ""
Write-Host "Useful commands:"
Write-Host "  Invoke-RestMethod http://127.0.0.1:$BasePort/status"
Write-Host "  Invoke-RestMethod http://127.0.0.1:$BasePort/peers"
Write-Host "  Invoke-RestMethod http://127.0.0.1:$BasePort/chain"
Write-Host "  powershell -ExecutionPolicy Bypass -File .\scripts\stop-cluster.ps1"
