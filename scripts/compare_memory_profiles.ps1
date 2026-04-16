param(
    [string]$Address = "",
    [int]$RequestCount = 2000,
    [string]$OutputDir = "artifacts/memory-profiles",
    [int]$StartupTimeoutSeconds = 30,
    [int]$InterRequestDelayMs = 0,
    [switch]$SkipTop
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Get-FreeLoopbackAddress {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    try {
        $listener.Start()
        $port = $listener.LocalEndpoint.Port
        return "127.0.0.1:$port"
    }
    finally {
        $listener.Stop()
    }
}

function Build-ProfilerBinary {
    param([Parameter(Mandatory = $true)][string]$OutputDir)

    $exePath = Join-Path $OutputDir "memory_profiler.exe"
    & go build -o $exePath ./cmd/memory_profiler
    if ($LASTEXITCODE -ne 0) {
        throw "go build failed for cmd/memory_profiler"
    }
    return $exePath
}

function Wait-ProfilerReady {
    param(
        [Parameter(Mandatory = $true)][string]$Address,
        [Parameter(Mandatory = $true)][int]$TimeoutSeconds,
        [Parameter(Mandatory = $true)]$Process,
        [Parameter(Mandatory = $true)][string]$Mode,
        [Parameter(Mandatory = $true)][string]$OutputDir
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $url = "http://$Address/debug/pprof/"

    while ((Get-Date) -lt $deadline) {
        if ($Process.HasExited) {
            $stderrPath = Join-Path $OutputDir ("{0}-stderr.log" -f $Mode)
            $stdoutPath = Join-Path $OutputDir ("{0}-stdout.log" -f $Mode)

            $stderrTail = ""
            $stdoutTail = ""

            if (Test-Path $stderrPath) {
                $stderrTail = (Get-Content -Path $stderrPath -Tail 40 -ErrorAction SilentlyContinue) -join "`n"
            }
            if (Test-Path $stdoutPath) {
                $stdoutTail = (Get-Content -Path $stdoutPath -Tail 40 -ErrorAction SilentlyContinue) -join "`n"
            }

            throw "Profiler process for mode '$Mode' exited before becoming ready. stdout:`n$stdoutTail`n`nstderr:`n$stderrTail"
        }

        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec 2
            if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 300) {
                return
            }
        }
        catch {
            # keep polling until timeout
        }
        Start-Sleep -Milliseconds 250
    }

    throw "Profiler endpoint was not ready within $TimeoutSeconds seconds: $url"
}

function Invoke-Load {
    param(
        [Parameter(Mandatory = $true)][string]$Address,
        [Parameter(Mandatory = $true)][int]$RequestCount,
        [Parameter(Mandatory = $true)][int]$InterRequestDelayMs
    )

    $url = "http://$Address/"
    for ($i = 0; $i -lt $RequestCount; $i++) {
        Invoke-WebRequest -UseBasicParsing -Uri $url -TimeoutSec 10 | Out-Null
        if ($InterRequestDelayMs -gt 0) {
            Start-Sleep -Milliseconds $InterRequestDelayMs
        }
    }
}

function Save-Profiles {
    param(
        [Parameter(Mandatory = $true)][string]$Address,
        [Parameter(Mandatory = $true)][string]$Mode,
        [Parameter(Mandatory = $true)][string]$OutputDir,
        [Parameter(Mandatory = $true)][switch]$SkipTop
    )

    $modeDir = Join-Path $OutputDir $Mode
    New-Item -ItemType Directory -Force -Path $modeDir | Out-Null

    Invoke-WebRequest -UseBasicParsing -Uri "http://$Address/debug/pprof/heap?gc=1" -OutFile (Join-Path $modeDir "heap.pb.gz")
    Invoke-WebRequest -UseBasicParsing -Uri "http://$Address/debug/pprof/allocs?gc=1" -OutFile (Join-Path $modeDir "allocs.pb.gz")
    Invoke-WebRequest -UseBasicParsing -Uri "http://$Address/debug/pprof/goroutine?debug=2" -OutFile (Join-Path $modeDir "goroutine.txt")

    if (-not $SkipTop) {
        & go tool pprof -top "http://$Address/debug/pprof/heap?gc=1" | Out-File -Encoding utf8 (Join-Path $modeDir "heap-top.txt")
        & go tool pprof -top "http://$Address/debug/pprof/allocs?gc=1" | Out-File -Encoding utf8 (Join-Path $modeDir "allocs-top.txt")
    }
}

function Start-Profiler {
    param(
        [Parameter(Mandatory = $true)][string]$ExecutablePath,
        [Parameter(Mandatory = $true)][string]$Mode,
        [Parameter(Mandatory = $true)][string]$Address,
        [Parameter(Mandatory = $true)][string]$OutputDir
    )

    $stdout = Join-Path $OutputDir ("{0}-stdout.log" -f $Mode)
    $stderr = Join-Path $OutputDir ("{0}-stderr.log" -f $Mode)

    $priorMode = $env:MEMORY_PROFILER_MODE
    $priorAddr = $env:MEMORY_PROFILER_ADDR

    try {
        $env:MEMORY_PROFILER_MODE = $Mode
        $env:MEMORY_PROFILER_ADDR = $Address

        return Start-Process -FilePath $ExecutablePath -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    }
    finally {
        if ($null -eq $priorMode) { Remove-Item Env:MEMORY_PROFILER_MODE -ErrorAction SilentlyContinue } else { $env:MEMORY_PROFILER_MODE = $priorMode }
        if ($null -eq $priorAddr) { Remove-Item Env:MEMORY_PROFILER_ADDR -ErrorAction SilentlyContinue } else { $env:MEMORY_PROFILER_ADDR = $priorAddr }
    }
}

function Stop-Profiler {
    param([Parameter(Mandatory = $true)]$Process)

    if ($null -eq $Process) {
        return
    }

    if (-not $Process.HasExited) {
        try {
            Stop-Process -Id $Process.Id -Force
        }
        catch {
            # ignore cleanup failure
        }
    }
}

function Get-GoroutineCountFromProfileFile {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path $Path)) {
        return $null
    }

    $lines = Get-Content -Path $Path -ErrorAction SilentlyContinue
    if ($null -eq $lines -or $lines.Count -eq 0) {
        return $null
    }

    foreach ($line in $lines) {
        if ($line -match "goroutine profile:\s*total\s+(\d+)") {
            return [int]$Matches[1]
        }
    }

    $goroutineHeaders = ($lines | Select-String -Pattern '^goroutine\s+\d+\s+\[').Count
    if ($goroutineHeaders -gt 0) {
        return [int]$goroutineHeaders
    }

    if ($lines[0] -match "goroutine profile:\s*total\s+(\d+)") {
        return [int]$Matches[1]
    }

    return $null
}

function Get-FileSizeBytes {
    param([Parameter(Mandatory = $true)][string]$Path)

    if (-not (Test-Path $Path)) {
        return $null
    }

    return (Get-Item -Path $Path).Length
}

function Get-TopPreviewLines {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [int]$Count = 10
    )

    if (-not (Test-Path $Path)) {
        return @()
    }

    $lines = Get-Content -Path $Path
    $cleaned = $lines | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    return @($cleaned | Select-Object -First $Count)
}

function Write-ComparisonSummary {
    param(
        [Parameter(Mandatory = $true)][string]$OutputDir,
        [Parameter(Mandatory = $true)][string]$Address,
        [Parameter(Mandatory = $true)][int]$RequestCount,
        [Parameter(Mandatory = $true)][switch]$SkipTop
    )

    $steadyDir = Join-Path $OutputDir "steady"
    $leakDir = Join-Path $OutputDir "leak"

    $steadyGoroutines = Get-GoroutineCountFromProfileFile -Path (Join-Path $steadyDir "goroutine.txt")
    $leakGoroutines = Get-GoroutineCountFromProfileFile -Path (Join-Path $leakDir "goroutine.txt")

    $steadyHeapBytes = Get-FileSizeBytes -Path (Join-Path $steadyDir "heap.pb.gz")
    $leakHeapBytes = Get-FileSizeBytes -Path (Join-Path $leakDir "heap.pb.gz")

    $steadyAllocsBytes = Get-FileSizeBytes -Path (Join-Path $steadyDir "allocs.pb.gz")
    $leakAllocsBytes = Get-FileSizeBytes -Path (Join-Path $leakDir "allocs.pb.gz")

    $summaryPath = Join-Path $OutputDir "comparison-summary.txt"
    $lines = [System.Collections.Generic.List[string]]::new()

    $lines.Add("timestampUtc=$([DateTime]::UtcNow.ToString("o"))")
    $lines.Add("address=$Address")
    $lines.Add("requestCount=$RequestCount")
    $lines.Add("")

    $lines.Add("goroutines.steady=$steadyGoroutines")
    $lines.Add("goroutines.leak=$leakGoroutines")
    if ($null -ne $steadyGoroutines -and $null -ne $leakGoroutines) {
        $lines.Add("goroutines.delta=$($leakGoroutines - $steadyGoroutines)")
    }
    else {
        $lines.Add("goroutines.delta=unknown")
    }

    $lines.Add("")
    $lines.Add("heapBytes.steady=$steadyHeapBytes")
    $lines.Add("heapBytes.leak=$leakHeapBytes")
    if ($null -ne $steadyHeapBytes -and $steadyHeapBytes -gt 0 -and $null -ne $leakHeapBytes) {
        $heapRatio = [math]::Round(($leakHeapBytes / [double]$steadyHeapBytes), 3)
        $lines.Add("heapBytes.ratioLeakOverSteady=$heapRatio")
    }
    else {
        $lines.Add("heapBytes.ratioLeakOverSteady=unknown")
    }

    $lines.Add("")
    $lines.Add("allocsBytes.steady=$steadyAllocsBytes")
    $lines.Add("allocsBytes.leak=$leakAllocsBytes")
    if ($null -ne $steadyAllocsBytes -and $steadyAllocsBytes -gt 0 -and $null -ne $leakAllocsBytes) {
        $allocsRatio = [math]::Round(($leakAllocsBytes / [double]$steadyAllocsBytes), 3)
        $lines.Add("allocsBytes.ratioLeakOverSteady=$allocsRatio")
    }
    else {
        $lines.Add("allocsBytes.ratioLeakOverSteady=unknown")
    }

    if (-not $SkipTop) {
        $lines.Add("")
        $lines.Add("steady.heapTop.preview:")
        foreach ($line in (Get-TopPreviewLines -Path (Join-Path $steadyDir "heap-top.txt"))) {
            $lines.Add("  $line")
        }

        $lines.Add("")
        $lines.Add("leak.heapTop.preview:")
        foreach ($line in (Get-TopPreviewLines -Path (Join-Path $leakDir "heap-top.txt"))) {
            $lines.Add("  $line")
        }

        $lines.Add("")
        $lines.Add("steady.allocsTop.preview:")
        foreach ($line in (Get-TopPreviewLines -Path (Join-Path $steadyDir "allocs-top.txt"))) {
            $lines.Add("  $line")
        }

        $lines.Add("")
        $lines.Add("leak.allocsTop.preview:")
        foreach ($line in (Get-TopPreviewLines -Path (Join-Path $leakDir "allocs-top.txt"))) {
            $lines.Add("  $line")
        }
    }

    $lines | Out-File -Encoding utf8 $summaryPath
}

$resolvedOutputDir = Resolve-Path -LiteralPath "." | ForEach-Object { Join-Path $_.Path $OutputDir }
New-Item -ItemType Directory -Force -Path $resolvedOutputDir | Out-Null

if ([string]::IsNullOrWhiteSpace($Address)) {
    $Address = Get-FreeLoopbackAddress
}

$profilerExePath = Build-ProfilerBinary -OutputDir $resolvedOutputDir

$metadataPath = Join-Path $resolvedOutputDir "run-metadata.txt"
@(
    "address=$Address",
    "requestCount=$RequestCount",
    "startupTimeoutSeconds=$StartupTimeoutSeconds",
    "interRequestDelayMs=$InterRequestDelayMs",
    "timestampUtc=$([DateTime]::UtcNow.ToString("o"))"
) | Out-File -Encoding utf8 $metadataPath

foreach ($mode in @("steady", "leak")) {
    Write-Host "=== Running mode: $mode ==="
    $proc = $null
    try {
        $proc = Start-Profiler -ExecutablePath $profilerExePath -Mode $mode -Address $Address -OutputDir $resolvedOutputDir
        Wait-ProfilerReady -Address $Address -TimeoutSeconds $StartupTimeoutSeconds -Process $proc -Mode $mode -OutputDir $resolvedOutputDir
        Invoke-Load -Address $Address -RequestCount $RequestCount -InterRequestDelayMs $InterRequestDelayMs
        Save-Profiles -Address $Address -Mode $mode -OutputDir $resolvedOutputDir -SkipTop:$SkipTop
    }
    finally {
        Stop-Profiler -Process $proc
    }
}

Write-ComparisonSummary -OutputDir $resolvedOutputDir -Address $Address -RequestCount $RequestCount -SkipTop:$SkipTop

Write-Host "Done. Artifacts written to: $resolvedOutputDir"
