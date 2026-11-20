$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'assert-test-storage.ps1')

# Exercise success and low-space outcomes without allocating files or a cluster.
Assert-EcoSnapTestStorage -Paths @($env:TEMP, $env:TEMP) -MinimumFreeBytes 1
$blocked = $false
try {
    Assert-EcoSnapTestStorage -Paths @($env:TEMP) -MinimumFreeBytes ([long]::MaxValue)
}
catch {
    if ($_.Exception.Message -notlike 'ECOSNAP_TEST_DISK_LOW:*') { throw }
    $blocked = $true
}
if (-not $blocked) { throw 'Guard seharusnya menolak ruang di bawah batas.' }
Write-Output 'PASS: jalur ruang cukup dan ruang tidak cukup; tidak membuat cluster/container.'
