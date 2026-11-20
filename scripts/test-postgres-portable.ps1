param([Parameter(Mandatory = $true)][string]$PostgresBin)
$ErrorActionPreference = 'Stop'
$bin = (Resolve-Path -LiteralPath $PostgresBin).Path
foreach ($name in @('initdb.exe','pg_ctl.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $bin $name))) { throw "Binary tidak tersedia: $name" }
}
$testRoot = [IO.Path]::GetFullPath((Join-Path $env:TEMP 'EcoSnapPostgresTests'))
$cluster = [IO.Path]::GetFullPath((Join-Path $testRoot ([guid]::NewGuid().ToString('N'))))
if (-not $cluster.StartsWith($testRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Cluster harus berada di direktori tes.' }
$previousUrl = $env:ECOSNAP_TEST_ADMIN_URL
$previousMaxProcs = $env:GOMAXPROCS
$previousMemLimit = $env:GOMEMLIMIT
$started = $false
$canDelete = $true
$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
$listener.Start()
$testPort = $listener.LocalEndpoint.Port
$listener.Stop()
New-Item -ItemType Directory -Path $cluster -Force | Out-Null
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
    # Trust is limited to this short-lived, loopback-only, synthetic test cluster.
    & (Join-Path $bin 'initdb.exe') -D (Join-Path $cluster 'data') -U postgres --auth=trust --encoding=UTF8 --locale=C | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'initdb gagal.' }
    & (Join-Path $bin 'pg_ctl.exe') -D (Join-Path $cluster 'data') -l (Join-Path $cluster 'postgres.log') -o "-h 127.0.0.1 -p $testPort -c shared_buffers=32MB -c max_connections=30 -c timezone=UTC" -w start
    if ($LASTEXITCODE -ne 0) { $canDelete = $false; throw "Server tes gagal start; log: $cluster" }
    $started = $true
    $env:ECOSNAP_TEST_ADMIN_URL = "postgresql://postgres@127.0.0.1:$testPort/postgres?sslmode=disable"
    $env:GOMAXPROCS = '2'
    $env:GOMEMLIMIT = '128MiB'
    & go test -p 1 -mod=readonly -count=1 -v ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tes gagal; lihat output Go.' }
}
finally {
    $env:ECOSNAP_TEST_ADMIN_URL = $previousUrl
    $env:GOMAXPROCS = $previousMaxProcs
    $env:GOMEMLIMIT = $previousMemLimit
    if ($started) {
        & (Join-Path $bin 'pg_ctl.exe') -D (Join-Path $cluster 'data') -m fast -w stop
        if ($LASTEXITCODE -ne 0) { $canDelete = $false; Write-Warning "Server belum berhenti; cluster dipertahankan: $cluster" }
    }
    if ($canDelete -and $cluster.StartsWith($testRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $cluster -Recurse -Force
    }
    Pop-Location
}
