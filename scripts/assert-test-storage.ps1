function Assert-EcoSnapTestStorage {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory = $true)][string[]]$Paths,
        [ValidateRange(1, [long]::MaxValue)][long]$MinimumFreeBytes = 4GB
    )
    $roots = @($Paths | ForEach-Object {
        [IO.Path]::GetPathRoot([IO.Path]::GetFullPath($_))
    } | Sort-Object -Unique)
    foreach ($root in $roots) {
        $drive = [IO.DriveInfo]::new($root)
        if (-not $drive.IsReady) { throw "Drive tes tidak tersedia: $root" }
        $available = $drive.AvailableFreeSpace
        if ($available -lt $MinimumFreeBytes) {
            $freeGiB = [math]::Round($available / 1GB, 2)
            $requiredGiB = [math]::Round($MinimumFreeBytes / 1GB, 2)
            throw "ECOSNAP_TEST_DISK_LOW: $root tersedia $freeGiB GiB; batas awal tes $requiredGiB GiB. Tes dibatalkan sebelum membuat database/container."
        }
    }
}

function Assert-EcoSnapRunnerStorage {
    param([Parameter(Mandatory = $true)][string]$RepoRoot)
    $cache = & go env GOCACHE
    if ($LASTEXITCODE -ne 0 -or -not $cache) { throw 'Tidak dapat membaca GOCACHE; periksa instalasi Go.' }
    Assert-EcoSnapTestStorage -Paths @($RepoRoot, $env:TEMP, ($cache -join '').Trim())
}
