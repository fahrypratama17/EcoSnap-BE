<#
.SYNOPSIS
  End-to-end smoke test against a RUNNING EcoSnap server with real Firebase
  tokens: estimate -> deposit -> QR -> claim -> wallet -> reward -> voucher.

.DESCRIPTION
  Prerequisites: server running (go run ./cmd/app), migrations 00001-00009
  applied, seeds run (seed-partners, seed-point-rates, seed-rewards,
  seed-articles), one mitra account (verified location) and one user account
  registered via POST /api/v1/accounts.

  Passwords are prompted and never printed. Tokens are never printed. The
  script is re-runnable: it uses unique idempotency keys and compares
  balances relative to the starting point. Each run consumes 1 stock of the
  500-point reward and creates 2 deposits.

.EXAMPLE
  ./scripts/smoke-test.ps1 -ApiKey AIzaSy...
#>
param(
    [Parameter(Mandatory = $true)][string]$ApiKey,
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$MitraEmail = "mitra.demo@ecosnap.test",
    [string]$UserEmail = "user.demo@ecosnap.test"
)
$ErrorActionPreference = "Stop"
$script:failed = 0
$script:passed = 0

function Read-Plain($prompt) {
    $secure = Read-Host $prompt -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try { [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
}

function Login($email, $password) {
    Invoke-RestMethod -Method Post -ContentType "application/json" `
        -Uri "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=$ApiKey" `
        -Body (@{ email = $email; password = $password; returnSecureToken = $true } | ConvertTo-Json)
}

# Call returns Status/Json/Raw and never throws on HTTP errors, so expected
# rejections (403, 409, ...) can be asserted.
function Call($method, $path, $token, $body, $extraHeaders) {
    $headers = @{ Authorization = "Bearer $token" }
    if ($extraHeaders) { foreach ($k in $extraHeaders.Keys) { $headers[$k] = $extraHeaders[$k] } }
    $params = @{ Method = $method; Uri = "$BaseUrl$path"; Headers = $headers; UseBasicParsing = $true }
    if ($null -ne $body) {
        $params.Body = ($body | ConvertTo-Json -Depth 6)
        $params.ContentType = "application/json"
    }
    try {
        $r = Invoke-WebRequest @params
        $status = [int]$r.StatusCode
        $text = $r.Content
    }
    catch {
        $resp = $_.Exception.Response
        if (-not $resp) { throw }
        $status = [int]$resp.StatusCode
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) { $text = $_.ErrorDetails.Message }
        else { $text = (New-Object IO.StreamReader($resp.GetResponseStream())).ReadToEnd() }
    }
    $json = $null
    if ($text) { try { $json = $text | ConvertFrom-Json } catch { } }
    [pscustomobject]@{ Status = $status; Json = $json; Raw = $text }
}

function Check($name, $ok, $detail) {
    if ($ok) {
        $script:passed++
        Write-Host ("  [PASS] " + $name) -ForegroundColor Green
    }
    else {
        $script:failed++
        Write-Host ("  [FAIL] " + $name) -ForegroundColor Red
        if ($detail) { Write-Host ("         " + $detail) -ForegroundColor Yellow }
    }
}
function Describe($r) {
    $code = ""
    if ($r.Json -and $r.Json.error) { $code = " " + $r.Json.error.code }
    "HTTP " + $r.Status + $code
}
function Step($title) { Write-Host ""; Write-Host $title -ForegroundColor Cyan }

$run = Get-Date -Format "yyyyMMddHHmmss"

Step "0. Server dan login"
try {
    $health = Invoke-WebRequest -Uri "$BaseUrl/health" -UseBasicParsing
    Check "server hidup (/health)" ($health.StatusCode -eq 200) $null
}
catch { Check "server hidup (/health)" $false "Jalankan 'go run ./cmd/app' di terminal lain."; exit 1 }
$mitra = Login $MitraEmail (Read-Plain "Password $MitraEmail")
$user = Login $UserEmail (Read-Plain "Password $UserEmail")
Check "login Firebase mitra dan user" ($mitra.idToken -and $user.idToken) $null
$mt = $mitra.idToken
$ut = $user.idToken

Step "1. Akun dan data demo"
$me = Call GET "/api/v1/me" $mt
Check "mitra terdaftar dengan role mitra" ($me.Status -eq 200 -and $me.Json.data.role -eq "mitra") (Describe $me)
$me = Call GET "/api/v1/me" $ut
Check "user terdaftar dengan role user" ($me.Status -eq 200 -and $me.Json.data.role -eq "user") (Describe $me)
$cats = Call GET "/api/v1/waste-categories" $ut
$plastic = ($cats.Json.data | Where-Object { $_.slug -eq "plastic" }).id
$paper = ($cats.Json.data | Where-Object { $_.slug -eq "paper" }).id
Check "kategori plastic dan paper ada (seed-partners)" ($plastic -and $paper) (Describe $cats)
$partners = Call GET "/api/v1/partners?material=plastic" $ut
$partner = $partners.Json.data | Select-Object -First 1
Check "lokasi mitra demo terlihat (seed-partners)" ($null -ne $partner) (Describe $partners)
if (-not ($plastic -and $paper -and $partner)) { Write-Host "Seed belum lengkap; hentikan." -ForegroundColor Red; exit 1 }
$w0 = (Call GET "/api/v1/me/wallet" $ut).Json.data
$start = [int64]$w0.balance
$spent0 = [int64]$w0.total_spent
Write-Host ("  saldo awal user: " + $start + " poin")

Step "2. Estimasi dan setoran oleh mitra"
$items = @(@{ category_id = $plastic; weight_kg = "1.000" }, @{ category_id = $paper; weight_kg = "0.500" })
$est = Call POST "/api/v1/point-estimates" $ut @{ partner_id = $partner.id; items = $items }
Check "estimasi 1 kg plastik + 0,5 kg kertas = 140 poin" ($est.Status -eq 200 -and $est.Json.data.estimated_points -eq 140) (Describe $est)
$key1 = "smoke-$run-a"
$dep = Call POST "/api/v1/deposits" $mt @{ items = $items } @{ "Idempotency-Key" = $key1 }
Check "mitra membuat setoran (201), total dihitung server" ($dep.Status -eq 201 -and $dep.Json.data.total_points -eq 140 -and $dep.Json.data.status -eq "created") (Describe $dep)
$depId = $dep.Json.data.id
$again = Call POST "/api/v1/deposits" $mt @{ items = $items } @{ "Idempotency-Key" = $key1 }
Check "retry key sama -> setoran yang sama (200)" ($again.Status -eq 200 -and $again.Json.data.id -eq $depId) (Describe $again)
$bad = Call POST "/api/v1/deposits" $ut @{ items = $items } @{ "Idempotency-Key" = "smoke-$run-user" }
Check "user biasa tidak boleh membuat setoran (403)" ($bad.Status -eq 403) (Describe $bad)
$cheat = Call POST "/api/v1/deposits" $mt @{ items = $items; total_points = 99999 } @{ "Idempotency-Key" = "smoke-$run-cheat" }
Check "total dari client ditolak (400)" ($cheat.Status -eq 400) (Describe $cheat)

Step "3. QR, preview, dan klaim"
$qr = Call POST "/api/v1/deposits/$depId/qr" $mt
Check "mitra menerbitkan QR (201) dengan warning dan server_time" ($qr.Status -eq 201 -and $qr.Json.data.token.Length -eq 43 -and $qr.Json.data.warning -and $qr.Json.data.server_time) (Describe $qr)
$token = $qr.Json.data.token
$secs = ([datetime]$qr.Json.data.expires_at - [datetime]$qr.Json.data.server_time).TotalSeconds
Check "QR berlaku 10 menit (600 detik)" ([math]::Abs($secs - 600) -lt 2) ("selisih " + $secs + " detik")
$prev = Call POST "/api/v1/deposit-claims/preview" $ut @{ token = $token }
Check "user preview: 140 poin dari Bank Sampah Tunas" ($prev.Status -eq 200 -and $prev.Json.data.total_points -eq 140) (Describe $prev)
$mclaim = Call POST "/api/v1/deposit-claims" $mt @{ token = $token }
Check "mitra tidak boleh klaim (403)" ($mclaim.Status -eq 403) (Describe $mclaim)
$claim = Call POST "/api/v1/deposit-claims" $ut @{ token = $token }
Check "user klaim (201), saldo naik 140" ($claim.Status -eq 201 -and $claim.Json.data.points -eq 140 -and [int64]$claim.Json.data.wallet_balance -eq ($start + 140)) (Describe $claim)
$replay = Call POST "/api/v1/deposit-claims" $ut @{ token = $token }
Check "klaim ulang oleh user sama -> 200 tanpa kredit ganda" ($replay.Status -eq 200 -and [int64]$replay.Json.data.wallet_balance -eq ($start + 140)) (Describe $replay)
$reissue = Call POST "/api/v1/deposits/$depId/qr" $mt
Check "QR tidak bisa diterbitkan ulang setelah diklaim (409)" ($reissue.Status -eq 409) (Describe $reissue)
$mdep = Call GET "/api/v1/deposits/$depId" $mt
Check "detail setoran mitra: status claimed" ($mdep.Status -eq 200 -and $mdep.Json.data.status -eq "claimed") (Describe $mdep)
$udep = Call GET "/api/v1/me/deposits/$depId" $ut
Check "detail setoran untuk pengklaim (200)" ($udep.Status -eq 200 -and $udep.Json.data.items.Count -eq 2) (Describe $udep)

Step "4. Setoran kedua (500 poin) dan penukaran reward"
$items2 = @(@{ category_id = $plastic; weight_kg = "5.000" })
$dep2 = Call POST "/api/v1/deposits" $mt @{ items = $items2 } @{ "Idempotency-Key" = "smoke-$run-b" }
Check "setoran 5 kg plastik = 500 poin" ($dep2.Status -eq 201 -and $dep2.Json.data.total_points -eq 500) (Describe $dep2)
$qr2 = Call POST "/api/v1/deposits/$($dep2.Json.data.id)/qr" $mt
$claim2 = Call POST "/api/v1/deposit-claims" $ut @{ token = $qr2.Json.data.token }
Check "klaim setoran kedua, saldo awal + 640" ($claim2.Status -eq 201 -and [int64]$claim2.Json.data.wallet_balance -eq ($start + 640)) (Describe $claim2)
$rewards = Call GET "/api/v1/rewards" $ut
$cheap = $rewards.Json.data | Where-Object { $_.cost_points -eq 500 } | Select-Object -First 1
$pricey = $rewards.Json.data | Where-Object { $_.cost_points -gt ($start + 640) } | Select-Object -First 1
Check "katalog reward ada (seed-rewards)" ($null -ne $cheap) (Describe $rewards)
$rk = "smoke-$run-redeem"
$red = Call POST "/api/v1/rewards/$($cheap.id)/redeem" $ut $null @{ "Idempotency-Key" = $rk }
Check "tukar reward 500 poin (201), saldo awal + 140" ($red.Status -eq 201 -and [int64]$red.Json.data.wallet_balance -eq ($start + 140) -and $red.Json.data.voucher.code -like "ECO-*") (Describe $red)
$redAgain = Call POST "/api/v1/rewards/$($cheap.id)/redeem" $ut $null @{ "Idempotency-Key" = $rk }
Check "retry penukaran -> voucher sama, tidak dipotong lagi (200)" ($redAgain.Status -eq 200 -and $redAgain.Json.data.voucher.id -eq $red.Json.data.voucher.id) (Describe $redAgain)
if ($pricey) {
    $poor = Call POST "/api/v1/rewards/$($pricey.id)/redeem" $ut $null @{ "Idempotency-Key" = "smoke-$run-poor" }
    Check "saldo kurang ditolak (409 insufficient_points)" ($poor.Status -eq 409 -and $poor.Json.error.code -eq "insufficient_points") (Describe $poor)
}
$mred = Call POST "/api/v1/rewards/$($cheap.id)/redeem" $mt $null @{ "Idempotency-Key" = "smoke-$run-mitra" }
Check "mitra tidak boleh menukar reward (403)" ($mred.Status -eq 403) (Describe $mred)
$vid = $red.Json.data.voucher.id
$vmitra = Call GET "/api/v1/me/vouchers/$vid" $mt
Check "voucher user tidak terlihat oleh akun lain (404)" ($vmitra.Status -eq 404) (Describe $vmitra)
$use = Call POST "/api/v1/me/vouchers/$vid/use" $ut
Check "tandai voucher dipakai" ($use.Status -eq 200 -and $use.Json.data.status -eq "used") (Describe $use)

Step "5. Saldo, riwayat, beranda, artikel"
$w1 = (Call GET "/api/v1/me/wallet" $ut).Json.data
Check "saldo akhir = awal + 140" ([int64]$w1.balance -eq ($start + 140)) ("saldo " + $w1.balance)
Check "total_spent naik 500" ([int64]$w1.total_spent -eq ($spent0 + 500)) ("total_spent " + $w1.total_spent)
$hist = Call GET "/api/v1/me/point-history?limit=3" $ut
$sum = 0; foreach ($e in $hist.Json.data) { $sum += [int64]$e.amount }
Check "riwayat poin terbaru: -500, +500, +140" ($hist.Status -eq 200 -and $sum -eq 140 -and $hist.Json.data[0].source -eq "redemption") (Describe $hist)
$uhome = Call GET "/api/v1/me/home" $ut
Check "beranda user" ($uhome.Status -eq 200 -and $uhome.Json.data.deposits_claimed -ge 2) (Describe $uhome)
$phome = Call GET "/api/v1/partner/home" $mt
Check "beranda mitra: lokasi verified" ($phome.Status -eq 200 -and $phome.Json.data.partner_status -eq "verified") (Describe $phome)
$arts = Call GET "/api/v1/articles?limit=10" $ut
Check "artikel demo tampil (seed-articles)" ($arts.Status -eq 200 -and $arts.Json.data.Count -ge 5) (Describe $arts)
$noauth = Call GET "/api/v1/me/wallet" "token-palsu"
Check "token palsu ditolak (401)" ($noauth.Status -eq 401) (Describe $noauth)

Write-Host ""
Write-Host ("Hasil: " + $script:passed + " lulus, " + $script:failed + " gagal") -ForegroundColor $(if ($script:failed -eq 0) { "Green" } else { "Red" })
if ($script:failed -gt 0) { exit 1 }
