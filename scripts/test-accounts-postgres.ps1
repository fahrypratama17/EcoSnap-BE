param([string]$Image = 'postgres:17-alpine')

$ErrorActionPreference = 'Stop'
$containerId = $null
$previousTestUrl = $env:ECOSNAP_TEST_ADMIN_URL
$previousMaxProcs = $env:GOMAXPROCS
$previousMemLimit = $env:GOMEMLIMIT
$repoRoot = Split-Path -Parent $PSScriptRoot
$containerName = 'ecosnap-account-tests-' + [guid]::NewGuid().ToString('N')

Push-Location $repoRoot
try {
    & docker info --format '{{.ServerVersion}}' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Docker Engine belum siap. Aktifkan Docker Desktop lalu ulangi runner ini.' }

    # No volume and no application credentials: this container is disposable.
    $runOutput = & docker run --detach --rm --name $containerName --env POSTGRES_PASSWORD=ecosnap-test-only --publish 127.0.0.1::5432 $Image
    if ($LASTEXITCODE -ne 0) { throw 'Gagal membuat container PostgreSQL tes.' }
    $containerId = ($runOutput | Select-Object -Last 1).Trim()
    if ($containerId -notmatch '^[a-f0-9]{64}$') { throw 'Container ID tidak valid; periksa Docker secara manual.' }

    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        & docker exec $containerId pg_isready -U postgres -d postgres 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) { $ready = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw 'PostgreSQL tes belum siap setelah 60 detik.' }
    $portOutput = & docker port $containerId 5432/tcp
    if ($LASTEXITCODE -ne 0) { throw 'Tidak dapat membaca port PostgreSQL tes.' }
    $portMatch = [regex]::Match(($portOutput -join ''), '^127\.0\.0\.1:(\d+)$')
    if (-not $portMatch.Success) { throw 'Port tes harus terikat ke loopback.' }
    $testPort = $portMatch.Groups[1].Value
    $env:ECOSNAP_TEST_ADMIN_URL = "postgresql://postgres:ecosnap-test-only@127.0.0.1:$testPort/postgres?sslmode=disable"
    $env:GOMAXPROCS = '2'
    $env:GOMEMLIMIT = '128MiB'
    & go test -p 1 -mod=readonly -count=1 ./... -v
    if ($LASTEXITCODE -ne 0) { throw 'Integration test akun gagal; lihat output Go di atas.' }
}
finally {
    $env:ECOSNAP_TEST_ADMIN_URL = $previousTestUrl
    $env:GOMAXPROCS = $previousMaxProcs
    $env:GOMEMLIMIT = $previousMemLimit
    if ($containerId -match '^[a-f0-9]{64}$') {
        & docker stop --time 5 $containerId | Out-Null
        if ($LASTEXITCODE -ne 0) { Write-Warning "Cleanup container tes gagal: $containerName" }
    }
    Pop-Location
}
