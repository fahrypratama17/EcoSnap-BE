<#
.SYNOPSIS
  End-to-end smoke test against a RUNNING EcoSnap server with real Firebase
  tokens: estimate -> deposits -> claims -> gamification -> wallet -> reward.


.DESCRIPTION
  Prerequisites: server running (go run ./cmd/app), migrations 00001-00010
  applied, seeds run (seed-partners, seed-point-rates, seed-rewards,
  seed-articles), one mitra account (verified location) and one user account
  registered via POST /api/v1/accounts.

  Migration 00010 and a server restart are required before this script mutates
  deposits. Mission claims award XP only; they do not change EcoPoints.
  Passwords are prompted and never printed. Tokens are never printed. The
  script is re-runnable: it uses unique idempotency keys and compares
  balances relative to the starting point. Each run consumes 1 stock of the
  500-point reward and creates 2 deposits.
  Use a dedicated user account and do not run concurrent smoke tests for it;
  weekly progress, badges, and mission claims are account-wide.

  Settings checks (GET/PATCH /api/v1/me/settings) run right after login and
  profile GET, before any deposit/seed/reward/gamification step, for both the
  mitra and user accounts. Only notification_enabled and onboarding_completed
  are mutated; both are restored in a finally block and verified. Use
  -SettingsOnly to run just those checks and exit.

.EXAMPLE
  ./scripts/smoke-test.ps1 -ApiKey AIzaSy...
#>
param(
    [Parameter(Mandatory = $true)][string]$ApiKey,
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$MitraEmail = "mitra.demo@ecosnap.test",
    [string]$UserEmail = "user.demo@ecosnap.test",
    [switch]$SettingsOnly
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

# --- Account settings checks -------------------------------------------------
# Settings endpoint payload is exactly data; /me embeds settings separately.
function Get-SettingsFields($r) {
    if ($r.Json) { return $r.Json.data }
    $null
}
function Test-SettingsShape($s) {
    $null -ne $s -and $s.notification_enabled -is [bool] -and $s.onboarding_completed -is [bool] -and $s.show_on_leaderboard -is [bool]
}
function Test-SettingsEqual($a, $b) {
    $null -ne $a -and $null -ne $b -and
    $a.notification_enabled -eq $b.notification_enabled -and
    $a.onboarding_completed -eq $b.onboarding_completed -and
    $a.show_on_leaderboard -eq $b.show_on_leaderboard
}
function Format-Settings($s) {
    if ($null -eq $s) { return "<none>" }
    "notification_enabled=" + $s.notification_enabled + " onboarding_completed=" + $s.onboarding_completed + " show_on_leaderboard=" + $s.show_on_leaderboard
}
function Get-WalletSnapshot($token) {
    $w = Call GET "/api/v1/me/wallet" $token
    if ($w.Status -ne 200 -or -not $w.Json.data) { throw ("Wallet snapshot unavailable: " + (Describe $w)) }
    "balance=" + $w.Json.data.balance + " total_earned=" + $w.Json.data.total_earned + " total_spent=" + $w.Json.data.total_spent
}
# Fails fast (before any mutation) when the settings route is missing or
# returns something other than 3 booleans. Returns the baseline settings.
function Assert-SettingsRoute($label, $token) {
    $r = Call GET "/api/v1/me/settings" $token
    $s = Get-SettingsFields $r
    if ($r.Status -ne 200 -or -not (Test-SettingsShape $s)) {
        Write-Host ("GET /api/v1/me/settings ($label) unavailable or malformed: " + (Describe $r) + "; aborting before any mutation. Deploy the settings route and restart the server.") -ForegroundColor Red
        exit 1
    }
    $s
}
# Runs all settings checks for one account. $otherToken is the other account,
# whose settings and the own wallet must stay untouched. $original is the
# baseline from Assert-SettingsRoute. The two mutable prefs are restored in
# finally and verified through GET and /me.
function Test-AccountSettings($label, $token, $otherToken, $original) {
    $path = "/api/v1/me/settings"
    $origN = [bool]$original.notification_enabled
    $origO = [bool]$original.onboarding_completed
    $origV = [bool]$original.show_on_leaderboard
    $otherBefore = Get-SettingsFields (Call GET $path $otherToken)
    $walletBefore = Get-WalletSnapshot $token
    try {
        $get = Call GET $path $token
        Check "[$label] GET settings: 200 with 3 booleans" ($get.Status -eq 200 -and (Test-SettingsShape (Get-SettingsFields $get))) (Describe $get)
        $meBase = Call GET "/api/v1/me" $token
        Check "[$label] /me settings match GET settings" ($meBase.Status -eq 200 -and (Test-SettingsEqual $meBase.Json.data.settings $original)) ((Describe $meBase) + " " + (Format-Settings $meBase.Json.data.settings))

        # notification toggle, partial body
        $newN = -not $origN
        $p1 = Call PATCH $path $token @{ notification_enabled = $newN }
        $s1 = Get-SettingsFields $p1
        Check "[$label] PATCH notification_enabled toggles it, keeps onboarding/leaderboard" ($p1.Status -eq 200 -and $s1.notification_enabled -eq $newN -and $s1.onboarding_completed -eq $origO -and $s1.show_on_leaderboard -eq $origV) ((Describe $p1) + " " + (Format-Settings $s1))
        $p1b = Call PATCH $path $token @{ notification_enabled = $newN }
        $s1b = Get-SettingsFields $p1b
        Check "[$label] repeating the same PATCH is idempotent" ($p1b.Status -eq 200 -and (Test-SettingsEqual $s1b $s1)) ((Describe $p1b) + " " + (Format-Settings $s1b))

        # onboarding toggle, partial body
        $newO = -not $origO
        $p2 = Call PATCH $path $token @{ onboarding_completed = $newO }
        $s2 = Get-SettingsFields $p2
        Check "[$label] PATCH onboarding_completed toggles it, keeps notification/leaderboard" ($p2.Status -eq 200 -and $s2.onboarding_completed -eq $newO -and $s2.notification_enabled -eq $newN -and $s2.show_on_leaderboard -eq $origV) ((Describe $p2) + " " + (Format-Settings $s2))

        # persistence via GET and profile
        $g2 = Call GET $path $token
        Check "[$label] GET settings reflects both changes" ($g2.Status -eq 200 -and (Test-SettingsEqual (Get-SettingsFields $g2) $s2)) ((Describe $g2) + " " + (Format-Settings (Get-SettingsFields $g2)))
        $m2 = Call GET "/api/v1/me" $token
        Check "[$label] /me profile reflects both changes" ($m2.Status -eq 200 -and (Test-SettingsEqual $m2.Json.data.settings $s2)) ((Describe $m2) + " " + (Format-Settings $m2.Json.data.settings))

        # invalid bodies: 400 and no state change
        $invalid = [ordered]@{
            "null notification_enabled"   = @{ notification_enabled = $null }
            "null onboarding_completed"   = @{ onboarding_completed = $null }
            "empty body"                  = @{}
            "uid field"                   = @{ uid = "someone-else" }
            "read-only show_on_leaderboard" = @{ show_on_leaderboard = (-not $origV) }
            "show_on_leaderboard with valid field" = @{ notification_enabled = $origN; show_on_leaderboard = (-not $origV) }
        }
        foreach ($name in $invalid.Keys) {
            $bad = Call PATCH $path $token $invalid[$name]
            Check "[$label] PATCH $name rejected (400)" ($bad.Status -eq 400) (Describe $bad)
        }
        $g3 = Call GET $path $token
        Check "[$label] rejected requests changed nothing" ($g3.Status -eq 200 -and (Test-SettingsEqual (Get-SettingsFields $g3) $s2)) ((Describe $g3) + " " + (Format-Settings (Get-SettingsFields $g3)))

        # authentication
        $na1 = Call GET $path "token-palsu"
        Check "[$label] GET settings with invalid token (401)" ($na1.Status -eq 401) (Describe $na1)
        $na2 = Call PATCH $path "token-palsu" @{ notification_enabled = $origN }
        Check "[$label] PATCH settings with invalid token (401)" ($na2.Status -eq 401) (Describe $na2)

        # isolation
        $otherAfter = Get-SettingsFields (Call GET $path $otherToken)
        Check "[$label] other account's settings unaffected" (Test-SettingsEqual $otherBefore $otherAfter) ((Format-Settings $otherBefore) + " -> " + (Format-Settings $otherAfter))
        $walletAfter = Get-WalletSnapshot $token
        Check "[$label] wallet unaffected by settings changes" ($walletBefore -eq $walletAfter) ($walletBefore + " -> " + $walletAfter)
    }
    finally {
        $rest = Call PATCH $path $token @{ notification_enabled = $origN; onboarding_completed = $origO }
        Check "[$label] restore notification_enabled/onboarding_completed (PATCH)" ($rest.Status -eq 200) (Describe $rest)
        $gr = Call GET $path $token
        Check "[$label] GET settings equals original after restore" ($gr.Status -eq 200 -and (Test-SettingsEqual (Get-SettingsFields $gr) $original)) ((Describe $gr) + " " + (Format-Settings (Get-SettingsFields $gr)))
        $mr = Call GET "/api/v1/me" $token
        Check "[$label] /me settings equal original after restore" ($mr.Status -eq 200 -and (Test-SettingsEqual $mr.Json.data.settings $original)) ((Describe $mr) + " " + (Format-Settings $mr.Json.data.settings))
    }
}


$run = [guid]::NewGuid().ToString("N")

Step "0. Server dan login"
try { $null = Invoke-RestMethod -Uri "$BaseUrl/health" -TimeoutSec 5 }
catch { Check "server hidup (/health)" $false "Jalankan 'go run ./cmd/app' di terminal lain."; exit 1 }
$mitra = Login $MitraEmail (Read-Plain "Password $MitraEmail")
$user = Login $UserEmail (Read-Plain "Password $UserEmail")
Check "login Firebase mitra dan user" ($mitra.idToken -and $user.idToken) $null
$mt = $mitra.idToken
$ut = $user.idToken

Step "1. Akun dan data demo"
$me = Call GET "/api/v1/me" $mt
Check "mitra terdaftar dengan role mitra" ($me.Status -eq 200 -and $me.Json.data.role -eq "mitra") (Describe $me)
$mitraProfile = $me.Json.data
$me = Call GET "/api/v1/me" $ut
Check "user terdaftar dengan role user" ($me.Status -eq 200 -and $me.Json.data.role -eq "user") (Describe $me)
$userProfile = $me.Json.data
$originalVisibility = $userProfile.settings.show_on_leaderboard
if ($originalVisibility -isnot [bool]) { Write-Host "Profile is missing a boolean settings.show_on_leaderboard; aborting before deposits." -ForegroundColor Red; exit 1 }

Step "1b. Pengaturan akun (settings)"
$mitraSettings0 = Assert-SettingsRoute "mitra" $mt
$userSettings0 = Assert-SettingsRoute "user" $ut
Check "profile settings sama dengan GET settings (mitra dan user)" ((Test-SettingsEqual $mitraProfile.settings $mitraSettings0) -and (Test-SettingsEqual $userProfile.settings $userSettings0)) ("mitra: " + (Format-Settings $mitraProfile.settings) + "; user: " + (Format-Settings $userProfile.settings))
Test-AccountSettings "mitra" $mt $ut $mitraSettings0
Test-AccountSettings "user" $ut $mt $userSettings0
$afterSettings = Call GET "/api/v1/me/settings" $ut
Check "original leaderboard visibility unaffected by settings checks" ($afterSettings.Status -eq 200 -and (Get-SettingsFields $afterSettings).show_on_leaderboard -eq $originalVisibility) (Describe $afterSettings)
if ($SettingsOnly) {
    Write-Host ""
    Write-Host ("Settings-only: " + $script:passed + " lulus, " + $script:failed + " gagal") -ForegroundColor $(if ($script:failed -eq 0) { "Green" } else { "Red" })
    if ($script:failed -gt 0) { exit 1 }
    exit 0
}
$cats = Call GET "/api/v1/waste-categories" $ut
$plastic = ($cats.Json.data | Where-Object { $_.slug -eq "plastic" }).id
$paper = ($cats.Json.data | Where-Object { $_.slug -eq "paper" }).id
Check "kategori plastic dan paper ada (seed-partners)" ($plastic -and $paper) (Describe $cats)
$partners = Call GET "/api/v1/partners?material=plastic" $ut
$partner = $partners.Json.data | Select-Object -First 1
Check "lokasi mitra demo terlihat (seed-partners)" ($null -ne $partner) (Describe $partners)
if (-not ($plastic -and $paper -and $partner)) { Write-Host "Seed belum lengkap; hentikan." -ForegroundColor Red; exit 1 }

# Fail before any deposit mutation if migration 00010/server deployment is absent.
$missions0 = Call GET "/api/v1/me/missions" $ut
$badges0 = Call GET "/api/v1/me/badges" $ut
if ($missions0.Status -ne 200 -or $badges0.Status -ne 200) {
    Write-Host "Gamification API unavailable. Apply migration 00010 and restart the server before rerunning; no deposits were created." -ForegroundColor Red
    exit 1
}
$periodStart = [string]$missions0.Json.data.period_start
$periodEnd = [string]$missions0.Json.data.period_end
$weeklyXp0 = [int64]$missions0.Json.data.weekly_xp
$totalXp0 = [int64]$missions0.Json.data.total_xp
$missionsById0 = @{}
foreach ($m in $missions0.Json.data.missions) { $missionsById0[$m.id] = $m }
$badgeById0 = @{}
foreach ($b in $badges0.Json.data.badges) { $badgeById0[$b.id] = $b }
$badgeClaimed0 = [int64]$badges0.Json.data.claimed_deposits
$badgeWeight0 = [int64]$badges0.Json.data.claimed_weight_grams

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
$missionsBeforeFirstClaim = Call GET "/api/v1/me/missions" $ut
Check "unclaimed deposit does not increase mission progress" ($missionsBeforeFirstClaim.Status -eq 200 -and $missionsBeforeFirstClaim.Json.data.period_start -eq $periodStart -and $missionsBeforeFirstClaim.Json.data.missions[0].progress -eq [int64]$missionsById0.first_deposit.progress -and $missionsBeforeFirstClaim.Json.data.missions[1].progress -eq [int64]$missionsById0.three_kilograms.progress) (Describe $missionsBeforeFirstClaim)
$mclaim = Call POST "/api/v1/deposit-claims" $mt @{ token = $token }
Check "mitra tidak boleh klaim (403)" ($mclaim.Status -eq 403) (Describe $mclaim)
$claim = Call POST "/api/v1/deposit-claims" $ut @{ token = $token }
Check "user klaim (201), saldo naik 140" ($claim.Status -eq 201 -and $claim.Json.data.points -eq 140 -and [int64]$claim.Json.data.wallet_balance -eq ($start + 140)) (Describe $claim)
$missionsAfterFirst = Call GET "/api/v1/me/missions" $ut
Check "first claimed deposit adds 1 deposit / 1500 g" ($missionsAfterFirst.Status -eq 200 -and $missionsAfterFirst.Json.data.period_start -eq $periodStart -and $missionsAfterFirst.Json.data.period_end -eq $periodEnd -and $missionsAfterFirst.Json.data.missions[0].progress -eq ([int64]$missionsById0.first_deposit.progress + 1) -and $missionsAfterFirst.Json.data.missions[1].progress -eq ([int64]$missionsById0.three_kilograms.progress + 1500)) (Describe $missionsAfterFirst)
Check "deposit progress alone awards no XP" ($missionsAfterFirst.Status -eq 200 -and [int64]$missionsAfterFirst.Json.data.weekly_xp -eq $weeklyXp0 -and [int64]$missionsAfterFirst.Json.data.total_xp -eq $totalXp0) (Describe $missionsAfterFirst)
$badgesAfterFirst = Call GET "/api/v1/me/badges" $ut
$badgeFirstAfter = $badgesAfterFirst.Json.data.badges | Where-Object { $_.id -eq "deposit_1" }
Check "first deposit earns permanent deposit_1 badge" ($badgesAfterFirst.Status -eq 200 -and [int64]$badgesAfterFirst.Json.data.claimed_deposits -eq ($badgeClaimed0 + 1) -and [int64]$badgesAfterFirst.Json.data.claimed_weight_grams -eq ($badgeWeight0 + 1500) -and $badgeFirstAfter.earned) (Describe $badgesAfterFirst)
$replay = Call POST "/api/v1/deposit-claims" $ut @{ token = $token }
Check "klaim ulang oleh user sama -> 200 tanpa kredit ganda" ($replay.Status -eq 200 -and [int64]$replay.Json.data.wallet_balance -eq ($start + 140)) (Describe $replay)
$reissue = Call POST "/api/v1/deposits/$depId/qr" $mt
Check "QR tidak bisa diterbitkan ulang setelah diklaim (409)" ($reissue.Status -eq 409) (Describe $reissue)
$mdep = Call GET "/api/v1/deposits/$depId" $mt
Check "detail setoran mitra: status claimed" ($mdep.Status -eq 200 -and $mdep.Json.data.status -eq "claimed") (Describe $mdep)
$udep = Call GET "/api/v1/me/deposits/$depId" $ut
Check "detail setoran untuk pengklaim (200)" ($udep.Status -eq 200 -and $udep.Json.data.items.Count -eq 2) (Describe $udep)

Step "3b. Setoran kedua (500 poin)"
$items2 = @(@{ category_id = $plastic; weight_kg = "5.000" })
$dep2 = Call POST "/api/v1/deposits" $mt @{ items = $items2 } @{ "Idempotency-Key" = "smoke-$run-b" }
Check "setoran 5 kg plastik = 500 poin" ($dep2.Status -eq 201 -and $dep2.Json.data.total_points -eq 500) (Describe $dep2)
$qr2 = Call POST "/api/v1/deposits/$($dep2.Json.data.id)/qr" $mt
$claim2 = Call POST "/api/v1/deposit-claims" $ut @{ token = $qr2.Json.data.token }
Check "klaim setoran kedua, saldo awal + 640" ($claim2.Status -eq 201 -and [int64]$claim2.Json.data.wallet_balance -eq ($start + 640)) (Describe $claim2)
Step "4. Gamification"
$missionsAfterSecond = Call GET "/api/v1/me/missions" $ut
$samePeriod = $missionsAfterSecond.Status -eq 200 -and $missionsAfterSecond.Json.data.period_start -eq $periodStart -and $missionsAfterSecond.Json.data.period_end -eq $periodEnd
Check "weekly period remains unchanged during smoke test" $samePeriod $(if (-not $samePeriod) { "Period reset detected; mission claims skipped to avoid a flaky cross-week assertion." } else { Describe $missionsAfterSecond })
Check "two claimed deposits add 2 deposits / 6500 g to weekly progress" ($samePeriod -and $missionsAfterSecond.Json.data.missions[0].progress -eq ([int64]$missionsById0.first_deposit.progress + 2) -and $missionsAfterSecond.Json.data.missions[1].progress -eq ([int64]$missionsById0.three_kilograms.progress + 6500)) (Describe $missionsAfterSecond)
$walletBeforeXP = (Call GET "/api/v1/me/wallet" $ut).Json.data
$xpBeforeClaims = [int64]$missionsAfterSecond.Json.data.total_xp
$weeklyBeforeClaims = [int64]$missionsAfterSecond.Json.data.weekly_xp
$newAwards = [int64]0
$oldPeriod = ([DateTimeOffset]::Parse($periodStart)).AddDays(-7).ToString("o")
$expired = Call POST "/api/v1/me/missions/first_deposit/claim" $ut @{ period_start = $oldPeriod }
Check "old weekly period rejected (410)" ($expired.Status -eq 410) (Describe $expired)
$mitraMission = Call POST "/api/v1/me/missions/first_deposit/claim" $mt @{ period_start = $periodStart }
Check "mitra cannot claim missions (403)" ($mitraMission.Status -eq 403) (Describe $mitraMission)
if ($samePeriod) {
    foreach ($missionId in @("first_deposit", "three_kilograms")) {
        $prior = $missionsById0[$missionId]
        $missionReward = if ($missionId -eq "first_deposit") { [int64]50 } else { [int64]100 }
        $expectedStatus = if ($prior.status -eq "claimed") { 200 } else { 201 }
        $missionClaim = Call POST "/api/v1/me/missions/$missionId/claim" $ut @{ period_start = $periodStart }
        $expectedClaimXP = $xpBeforeClaims + $newAwards + $(if ($expectedStatus -eq 201) { $missionReward } else { 0 })
        $expectedClaimWeeklyXP = $weeklyBeforeClaims + $newAwards + $(if ($expectedStatus -eq 201) { $missionReward } else { 0 })
        Check "$missionId claim status, reward, and XP delta match baseline" ($missionClaim.Status -eq $expectedStatus -and $missionClaim.Json.data.mission.status -eq "claimed" -and [int64]$missionClaim.Json.data.xp_awarded -eq $missionReward -and [int64]$missionClaim.Json.data.total_xp -eq $expectedClaimXP -and [int64]$missionClaim.Json.data.weekly_xp -eq $expectedClaimWeeklyXP) $(if ($missionClaim.Status -eq 410) { "Weekly period reset while the smoke test was running; subsequent mission claims were skipped." } else { Describe $missionClaim })
        if ($missionClaim.Status -eq 410) { break }
        if ($expectedStatus -eq 201) { $newAwards += $missionReward }
        $missionReplay = Call POST "/api/v1/me/missions/$missionId/claim" $ut @{ period_start = $periodStart }
        Check "$missionId replay returns 200 with no additional XP" ($missionReplay.Status -eq 200 -and [int64]$missionReplay.Json.data.xp_awarded -eq $missionReward -and [int64]$missionReplay.Json.data.total_xp -eq ($xpBeforeClaims + $newAwards) -and [int64]$missionReplay.Json.data.weekly_xp -eq ($weeklyBeforeClaims + $newAwards)) (Describe $missionReplay)
    }
    $missionsAfterXP = Call GET "/api/v1/me/missions" $ut
    Check "only newly claimed mission rewards increase XP" ($missionsAfterXP.Status -eq 200 -and [int64]$missionsAfterXP.Json.data.total_xp -eq ($xpBeforeClaims + $newAwards) -and [int64]$missionsAfterXP.Json.data.weekly_xp -eq ($weeklyBeforeClaims + $newAwards)) (Describe $missionsAfterXP)
    $walletAfterXP = (Call GET "/api/v1/me/wallet" $ut).Json.data
    Check "mission XP leaves EcoPoints wallet unchanged" ([int64]$walletAfterXP.balance -eq [int64]$walletBeforeXP.balance -and [int64]$walletAfterXP.total_earned -eq [int64]$walletBeforeXP.total_earned -and [int64]$walletAfterXP.total_spent -eq [int64]$walletBeforeXP.total_spent) $null
}
$badgesAfterTwo = Call GET "/api/v1/me/badges" $ut
$badgeFirstFinal = $badgesAfterTwo.Json.data.badges | Where-Object { $_.id -eq "deposit_1" }
Check "badge totals include both claimed deposits and 6500 g" ($badgesAfterTwo.Status -eq 200 -and [int64]$badgesAfterTwo.Json.data.claimed_deposits -eq ($badgeClaimed0 + 2) -and [int64]$badgesAfterTwo.Json.data.claimed_weight_grams -eq ($badgeWeight0 + 6500)) (Describe $badgesAfterTwo)
$badgeTimesStable = $true
foreach ($badge in $badgesAfterTwo.Json.data.badges) {
    $oldBadge = $badgeById0[$badge.id]
    if ($oldBadge.earned -and (-not $badge.earned -or $badge.unlocked_at -ne $oldBadge.unlocked_at)) { $badgeTimesStable = $false }
    $firstBadge = $badgesAfterFirst.Json.data.badges | Where-Object { $_.id -eq $badge.id }
    if ($firstBadge.earned -and (-not $badge.earned -or $badge.unlocked_at -ne $firstBadge.unlocked_at)) { $badgeTimesStable = $false }
}
Check "earned badges keep their original unlocked_at timestamps" ($badgeTimesStable -and $badgeFirstFinal.earned) (Describe $badgesAfterTwo)
Step "5. Leaderboard privacy"
try {
    $hidden = Call PATCH "/api/v1/me/leaderboard-visibility" $ut @{ visible = $false }
    Check "hide user from leaderboard" ($hidden.Status -eq 200 -and $hidden.Json.data.visible -eq $false) (Describe $hidden)
    $hiddenBoard = Call GET "/api/v1/leaderboard" $ut
    Check "hidden user has null leaderboard me" ($hiddenBoard.Status -eq 200 -and $null -eq $hiddenBoard.Json.data.me) (Describe $hiddenBoard)
    $visible = Call PATCH "/api/v1/me/leaderboard-visibility" $ut @{ visible = $true }
    Check "show user on leaderboard" ($visible.Status -eq 200 -and $visible.Json.data.visible -eq $true) (Describe $visible)
    $publicBoard = Call GET "/api/v1/leaderboard" $ut
    $publicFieldsOnly = $true
    $allowedFields = @("rank", "display_name", "photo_path", "xp")
    foreach ($entry in $publicBoard.Json.data.entries) {
        foreach ($field in $entry.PSObject.Properties.Name) {
            if ($allowedFields -notcontains $field) { $publicFieldsOnly = $false }
        }
    }
    Check "leaderboard entries expose only public fields (no UID/email)" ($publicBoard.Status -eq 200 -and $publicFieldsOnly) (Describe $publicBoard)
    $leaderboardMe = $publicBoard.Json.data.me
    if (($xpBeforeClaims + $newAwards) -gt 0 -and $samePeriod) {
        Check "opted-in user with weekly XP has valid rank" ($null -ne $leaderboardMe -and $leaderboardMe.rank -ge 1 -and $leaderboardMe.xp -gt 0) (Describe $publicBoard)
    }
}
finally {
    $restored = Call PATCH "/api/v1/me/leaderboard-visibility" $ut @{ visible = [bool]$originalVisibility }
    Check "restore original leaderboard visibility" ($restored.Status -eq 200 -and $restored.Json.data.visible -eq [bool]$originalVisibility) (Describe $restored)
}
Step "6. Reward"
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

Step "7. Saldo, riwayat, beranda, artikel"
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
