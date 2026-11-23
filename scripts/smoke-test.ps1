<#
.SYNOPSIS
  End-to-end smoke test against a RUNNING EcoSnap server with real Firebase
  tokens: estimate -> deposits -> claims -> gamification -> wallet -> reward.


.DESCRIPTION
  Prerequisites: server running (go run ./cmd/app), all migrations through
  00012 applied (00011 notifications and 00012 support), seeds run
  (seed-partners, seed-point-rates, seed-rewards, seed-articles), one verified
  mitra account and one user account registered via POST /api/v1/accounts.

  All new flow routes are preflighted before any deposit/reward/ticket mutation;
  missing migrations/routes abort rather than treating failed reads as zero.
  Mission claims award XP only; they do not change EcoPoints. Passwords are
  prompted and never printed. Tokens are never printed. The script uses unique
  idempotency keys and compares balances against each account's starting point.
  Each full run consumes one reward stock and creates two deposits. The reward
  cost and deposit points are selected from live estimates/catalog data.
  Use a dedicated user account; weekly progress, badges, and mission claims are
  account-wide.

  Settings checks (GET/PATCH /api/v1/me/settings) run right after login and
  profile GET, before any deposit/seed/reward/gamification step, for both the
  mitra and user accounts. Only notification_enabled and onboarding_completed
  are mutated; both are restored in a finally block and verified. Use
  -SettingsOnly to run just those checks and exit.

  Flow checks (scripts/smoke-flow-checks.ps1): before ANY deposit, reward or
  ticket mutation, a read-only preflight validates GET /me/home (both roles),
  /me/activity-report JSON + PDF (headers, %PDF magic, startxref/xref), the
  inbox, FAQ and support-ticket routes. A missing route or migration
  (00011 notifications, 00012 support) aborts the run; it is never read as zero.
  Rates, estimates, the own-mitra location (GET /partner/home then
  /partners/{id}), the reward and mission XP come from live responses, not
  constants. Full runs create 2 deposits, redeem 1 reward (stock -1; prefers
  the 500-point seed reward, otherwise the cheapest affordable one) and claim
  at most 2 missions (XP only). They verify Home counters (all-time vs WIB
  month) against badges/report/wallet, the report totals, and the inbox events
  from this run (deposit_credited x2, deposit_claimed x2 for the mitra,
  voucher_issued, mission_xp_claimed per new claim); one own notification is
  then marked read. If notification_enabled is false it is enabled only for
  the mutating phase and restored in finally, together with the other prefs.

  -FlowsOnly: preflight + FAQ/ticket validation only; no deposit, reward,
  mission, mark-read or leaderboard mutation (settings checks still run before).
  -CreateSupportTicket: opt-in; creates ONE synthetic "problem" ticket and
  checks idempotency/list/detail/foreign 404. Users cannot delete tickets and
  open + in_progress tickets are capped at 20 per account, so every run with
  this switch consumes one slot until staff resolves it. The preflight rejects
  the run before any deposit when 20 are already unresolved.

.EXAMPLE
  ./scripts/smoke-test.ps1 -ApiKey AIzaSy...
#>
param(
    [Parameter(Mandatory = $true)][string]$ApiKey,
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$MitraEmail = "mitra.demo@ecosnap.test",
    [string]$UserEmail = "user.demo@ecosnap.test",
    [switch]$SettingsOnly,
    [switch]$FlowsOnly,
    [switch]$CreateSupportTicket
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

. (Join-Path $PSScriptRoot "smoke-flow-checks.ps1")

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
Step "2. Prasyarat data, mitra sendiri, dan rute baru (tanpa mutasi)"
$cats = Call GET "/api/v1/waste-categories" $ut
$plastic = ($cats.Json.data | Where-Object { $_.slug -eq "plastic" }).id
$paper = ($cats.Json.data | Where-Object { $_.slug -eq "paper" }).id
Check "kategori plastic dan paper ada (seed-partners)" ($plastic -and $paper) (Describe $cats)
$phome0 = Call GET "/api/v1/partner/home" $mt
$ownPartnerId = $null
if ($phome0.Status -eq 200) { $ownPartnerId = [string]$phome0.Json.data.partner_id }
Check "lokasi mitra sendiri terdeteksi via /partner/home (verified)" ($phome0.Status -eq 200 -and $ownPartnerId -and $phome0.Json.data.partner_status -eq "verified") (Describe $phome0)
$partner = $null
if ($ownPartnerId) {
    $pd = Call GET "/api/v1/partners/$ownPartnerId" $ut
    $slugs = @($pd.Json.data.accepted_materials | ForEach-Object { $_.slug })
    Check "lokasi mitra sendiri terlihat user via /partners/{id} dan menerima plastic + paper" ($pd.Status -eq 200 -and $pd.Json.data.id -eq $ownPartnerId -and ($slugs -contains "plastic") -and ($slugs -contains "paper")) (Describe $pd)
    if ($pd.Status -eq 200) { $partner = $pd.Json.data }
}
if (-not ($plastic -and $paper -and $partner)) { Write-Host "Seed/mitra belum lengkap; hentikan sebelum mutasi." -ForegroundColor Red; exit 1 }

# Fail before any deposit mutation if migration 00010/server deployment is absent.
$missions0 = Call GET "/api/v1/me/missions" $ut
$badges0 = Call GET "/api/v1/me/badges" $ut
if ($missions0.Status -ne 200 -or $badges0.Status -ne 200) {
    Write-Host "Gamification API unavailable. Apply migration 00010 and restart the server before rerunning; no deposits were created." -ForegroundColor Red
    exit 1
}
$preflight = Invoke-FlowPreflight $ut $mt ([bool]$CreateSupportTicket)
if (-not $preflight.Ok) {
    Write-Host "Flow preflight failed (rute/migrasi/kontrak). Tidak ada setoran, penukaran, misi, atau tiket yang dibuat." -ForegroundColor Red
    Write-Host ("Hasil: " + $script:passed + " lulus, " + $script:failed + " gagal") -ForegroundColor Red
    exit 1
}
if ($FlowsOnly) {
    Step "3. Bantuan (FAQ dan tiket)"
    Test-SupportFlows $ut $mt $run ([bool]$CreateSupportTicket) $preflight.Unresolved
    Write-Host ""
    Write-Host ("Flows-only: " + $script:passed + " lulus, " + $script:failed + " gagal") -ForegroundColor $(if ($script:failed -eq 0) { "Green" } else { "Red" })
    if ($script:failed -gt 0) { exit 1 }
    exit 0
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

Step "2b. Estimasi (tarif aktual mitra sendiri) dan pilihan reward, sebelum mutasi"
$items = @(@{ category_id = $plastic; weight_kg = "1.000" }, @{ category_id = $paper; weight_kg = "0.500" })
$items2 = @(@{ category_id = $plastic; weight_kg = "5.000" })
$grams1 = [int64]1500
$grams2 = [int64]5000
$est = Call POST "/api/v1/point-estimates" $ut @{ partner_id = $partner.id; items = $items }
$est2 = Call POST "/api/v1/point-estimates" $ut @{ partner_id = $partner.id; items = $items2 }
Check "estimasi setoran 1 dan 2 tersedia (200)" ($est.Status -eq 200 -and $est2.Status -eq 200) ((Describe $est) + " / " + (Describe $est2))
$estOk = $false
if ($est.Status -eq 200 -and $est2.Status -eq 200) {
    $a1 = Assert-Problems "estimasi 1 kg plastik + 0,5 kg kertas konsisten dengan tarif respons" (Get-EstimateProblems $est.Json.data $items)
    $a2 = Assert-Problems "estimasi 5 kg plastik konsisten dengan tarif respons" (Get-EstimateProblems $est2.Json.data $items2)
    $estOk = $a1 -and $a2 -and [int64]$est.Json.data.estimated_points -gt 0 -and [int64]$est2.Json.data.estimated_points -gt 0
}
if (-not $estOk) { Write-Host "Estimasi tidak valid; hentikan sebelum mutasi." -ForegroundColor Red; exit 1 }
$pts1 = [int64]$est.Json.data.estimated_points
$pts2 = [int64]$est2.Json.data.estimated_points
$totalPts = $pts1 + $pts2
$funds = $start + $totalPts
$rewards = Call GET "/api/v1/rewards" $ut
$cheap = Select-Reward $rewards.Json.data $funds
Check "katalog reward punya reward tersedia yang terjangkau saldo akhir ($funds poin)" ($null -ne $cheap) (Describe $rewards)
if ($null -eq $cheap) { Write-Host "Tidak ada reward terjangkau; hentikan sebelum setoran." -ForegroundColor Red; exit 1 }
$cost = [int64]$cheap.cost_points
Write-Host ("  estimasi: setoran 1 = " + $pts1 + ", setoran 2 = " + $pts2 + " poin; reward dipilih seharga " + $cost + " poin")

$home0 = $preflight.HomeUser
$phomeBase = $phome0.Json.data
$prefUser = New-NotificationPrefState "user" $ut $userSettings0
$prefMitra = New-NotificationPrefState "mitra" $mt $mitraSettings0
$newMissionClaims = New-Object System.Collections.ArrayList
try {
Enable-NotificationPref $prefUser
Enable-NotificationPref $prefMitra

Step "3. Setoran oleh mitra sendiri"
$key1 = "smoke-$run-a"
$dep = Call POST "/api/v1/deposits" $mt @{ items = $items } @{ "Idempotency-Key" = $key1 }
Check "mitra membuat setoran (201), total dihitung server = estimasi ($pts1)" ($dep.Status -eq 201 -and $dep.Json.data.total_points -eq $pts1 -and $dep.Json.data.status -eq "created") (Describe $dep)
$depId = $dep.Json.data.id
$again = Call POST "/api/v1/deposits" $mt @{ items = $items } @{ "Idempotency-Key" = $key1 }
Check "retry key sama -> setoran yang sama (200)" ($again.Status -eq 200 -and $again.Json.data.id -eq $depId) (Describe $again)
$bad = Call POST "/api/v1/deposits" $ut @{ items = $items } @{ "Idempotency-Key" = "smoke-$run-user" }
Check "user biasa tidak boleh membuat setoran (403)" ($bad.Status -eq 403) (Describe $bad)
$cheat = Call POST "/api/v1/deposits" $mt @{ items = $items; total_points = 99999 } @{ "Idempotency-Key" = "smoke-$run-cheat" }
Check "total dari client ditolak (400)" ($cheat.Status -eq 400) (Describe $cheat)

Step "3b. QR, preview, dan klaim"
$qr = Call POST "/api/v1/deposits/$depId/qr" $mt
Check "mitra menerbitkan QR (201) dengan warning dan server_time" ($qr.Status -eq 201 -and $qr.Json.data.token.Length -eq 43 -and $qr.Json.data.warning -and $qr.Json.data.server_time) (Describe $qr)
$token = $qr.Json.data.token
$secs = ([datetime]$qr.Json.data.expires_at - [datetime]$qr.Json.data.server_time).TotalSeconds
Check "QR berlaku 10 menit (600 detik)" ([math]::Abs($secs - 600) -lt 2) ("selisih " + $secs + " detik")
$prev = Call POST "/api/v1/deposit-claims/preview" $ut @{ token = $token }
Check "user preview: $pts1 poin" ($prev.Status -eq 200 -and $prev.Json.data.total_points -eq $pts1) (Describe $prev)
$missionsBeforeFirstClaim = Call GET "/api/v1/me/missions" $ut
Check "unclaimed deposit does not increase mission progress" ($missionsBeforeFirstClaim.Status -eq 200 -and $missionsBeforeFirstClaim.Json.data.period_start -eq $periodStart -and $missionsBeforeFirstClaim.Json.data.missions[0].progress -eq [int64]$missionsById0.first_deposit.progress -and $missionsBeforeFirstClaim.Json.data.missions[1].progress -eq [int64]$missionsById0.three_kilograms.progress) (Describe $missionsBeforeFirstClaim)
$mclaim = Call POST "/api/v1/deposit-claims" $mt @{ token = $token }
Check "mitra tidak boleh klaim (403)" ($mclaim.Status -eq 403) (Describe $mclaim)
$claim = Call POST "/api/v1/deposit-claims" $ut @{ token = $token }
Check "user klaim (201), saldo naik $pts1" ($claim.Status -eq 201 -and $claim.Json.data.points -eq $pts1 -and [int64]$claim.Json.data.wallet_balance -eq ($start + $pts1)) (Describe $claim)
$missionsAfterFirst = Call GET "/api/v1/me/missions" $ut
Check "first claimed deposit adds 1 deposit / 1500 g" ($missionsAfterFirst.Status -eq 200 -and $missionsAfterFirst.Json.data.period_start -eq $periodStart -and $missionsAfterFirst.Json.data.period_end -eq $periodEnd -and $missionsAfterFirst.Json.data.missions[0].progress -eq ([int64]$missionsById0.first_deposit.progress + 1) -and $missionsAfterFirst.Json.data.missions[1].progress -eq ([int64]$missionsById0.three_kilograms.progress + 1500)) (Describe $missionsAfterFirst)
Check "deposit progress alone awards no XP" ($missionsAfterFirst.Status -eq 200 -and [int64]$missionsAfterFirst.Json.data.weekly_xp -eq $weeklyXp0 -and [int64]$missionsAfterFirst.Json.data.total_xp -eq $totalXp0) (Describe $missionsAfterFirst)
$badgesAfterFirst = Call GET "/api/v1/me/badges" $ut
$badgeFirstAfter = $badgesAfterFirst.Json.data.badges | Where-Object { $_.id -eq "deposit_1" }
Check "first deposit earns permanent deposit_1 badge" ($badgesAfterFirst.Status -eq 200 -and [int64]$badgesAfterFirst.Json.data.claimed_deposits -eq ($badgeClaimed0 + 1) -and [int64]$badgesAfterFirst.Json.data.claimed_weight_grams -eq ($badgeWeight0 + $grams1) -and $badgeFirstAfter.earned) (Describe $badgesAfterFirst)
$replay = Call POST "/api/v1/deposit-claims" $ut @{ token = $token }
Check "klaim ulang oleh user sama -> 200 tanpa kredit ganda" ($replay.Status -eq 200 -and [int64]$replay.Json.data.wallet_balance -eq ($start + $pts1)) (Describe $replay)
$reissue = Call POST "/api/v1/deposits/$depId/qr" $mt
Check "QR tidak bisa diterbitkan ulang setelah diklaim (409)" ($reissue.Status -eq 409) (Describe $reissue)
$mdep = Call GET "/api/v1/deposits/$depId" $mt
Check "detail setoran mitra: status claimed" ($mdep.Status -eq 200 -and $mdep.Json.data.status -eq "claimed") (Describe $mdep)
$udep = Call GET "/api/v1/me/deposits/$depId" $ut
Check "detail setoran untuk pengklaim (200)" ($udep.Status -eq 200 -and $udep.Json.data.items.Count -eq 2) (Describe $udep)

Step "3c. Setoran kedua"
$dep2 = Call POST "/api/v1/deposits" $mt @{ items = $items2 } @{ "Idempotency-Key" = "smoke-$run-b" }
Check "setoran 5 kg plastik = estimasi ($pts2 poin)" ($dep2.Status -eq 201 -and $dep2.Json.data.total_points -eq $pts2) (Describe $dep2)
$dep2Id = $dep2.Json.data.id
$qr2 = Call POST "/api/v1/deposits/$dep2Id/qr" $mt
$token2 = $qr2.Json.data.token
$claim2 = Call POST "/api/v1/deposit-claims" $ut @{ token = $token2 }
Check "klaim setoran kedua, saldo awal + $totalPts" ($claim2.Status -eq 201 -and [int64]$claim2.Json.data.wallet_balance -eq ($start + $totalPts)) (Describe $claim2)
Step "4. Gamification"
$missionsAfterSecond = Call GET "/api/v1/me/missions" $ut
$samePeriod = $missionsAfterSecond.Status -eq 200 -and $missionsAfterSecond.Json.data.period_start -eq $periodStart -and $missionsAfterSecond.Json.data.period_end -eq $periodEnd
Check "weekly period remains unchanged during smoke test" $samePeriod $(if (-not $samePeriod) { "Period reset detected; mission claims skipped to avoid a flaky cross-week assertion." } else { Describe $missionsAfterSecond })
Check "two claimed deposits add 2 deposits / 6500 g to weekly progress" ($samePeriod -and $missionsAfterSecond.Json.data.missions[0].progress -eq ([int64]$missionsById0.first_deposit.progress + 2) -and $missionsAfterSecond.Json.data.missions[1].progress -eq ([int64]$missionsById0.three_kilograms.progress + ($grams1 + $grams2))) (Describe $missionsAfterSecond)
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
        $missionReward = [int64]$prior.reward_xp
        $expectedStatus = if ($prior.status -eq "claimed") { 200 } else { 201 }
        $missionClaim = Call POST "/api/v1/me/missions/$missionId/claim" $ut @{ period_start = $periodStart }
        $expectedClaimXP = $xpBeforeClaims + $newAwards + $(if ($expectedStatus -eq 201) { $missionReward } else { 0 })
        $expectedClaimWeeklyXP = $weeklyBeforeClaims + $newAwards + $(if ($expectedStatus -eq 201) { $missionReward } else { 0 })
        Check "$missionId claim status, reward, and XP delta match baseline" ($missionClaim.Status -eq $expectedStatus -and $missionClaim.Json.data.mission.status -eq "claimed" -and [int64]$missionClaim.Json.data.xp_awarded -eq $missionReward -and [int64]$missionClaim.Json.data.total_xp -eq $expectedClaimXP -and [int64]$missionClaim.Json.data.weekly_xp -eq $expectedClaimWeeklyXP) $(if ($missionClaim.Status -eq 410) { "Weekly period reset while the smoke test was running; subsequent mission claims were skipped." } else { Describe $missionClaim })
        if ($missionClaim.Status -eq 410) { break }
        if ($expectedStatus -eq 201) { $newAwards += $missionReward; $null = $newMissionClaims.Add($missionId) }
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
Check "badge totals include both claimed deposits and 6500 g" ($badgesAfterTwo.Status -eq 200 -and [int64]$badgesAfterTwo.Json.data.claimed_deposits -eq ($badgeClaimed0 + 2) -and [int64]$badgesAfterTwo.Json.data.claimed_weight_grams -eq ($badgeWeight0 + $grams1 + $grams2)) (Describe $badgesAfterTwo)
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
$rk = "smoke-$run-redeem"
$red = Call POST "/api/v1/rewards/$($cheap.id)/redeem" $ut $null @{ "Idempotency-Key" = $rk }
Check "tukar reward $cost poin (201), saldo awal + $($totalPts - $cost)" ($red.Status -eq 201 -and [int64]$red.Json.data.wallet_balance -eq ($start + $totalPts - $cost) -and $red.Json.data.voucher.code -like "ECO-*") (Describe $red)
$redAgain = Call POST "/api/v1/rewards/$($cheap.id)/redeem" $ut $null @{ "Idempotency-Key" = $rk }
Check "retry penukaran -> voucher sama, tidak dipotong lagi (200)" ($redAgain.Status -eq 200 -and $redAgain.Json.data.voucher.id -eq $red.Json.data.voucher.id) (Describe $redAgain)
$pricey = Select-ExpensiveReward $rewards.Json.data $funds
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

Step "6b. Kotak masuk: peristiwa dari run ini"
$forbidden = @($token, $token2, [string]$red.Json.data.voucher.code)
$expectedUser = New-Object System.Collections.ArrayList
$null = $expectedUser.Add(@{ type = "deposit_credited"; target_type = "deposit"; target_id = $depId })
$null = $expectedUser.Add(@{ type = "deposit_credited"; target_type = "deposit"; target_id = $dep2Id })
$null = $expectedUser.Add(@{ type = "voucher_issued"; target_type = "voucher"; target_id = $vid })
foreach ($mid in $newMissionClaims) { $null = $expectedUser.Add(@{ type = "mission_xp_claimed"; target_type = "mission"; target_id = $mid }) }
$expectedMitra = @(@{ type = "deposit_claimed"; target_type = "deposit"; target_id = $depId }, @{ type = "deposit_claimed"; target_type = "deposit"; target_id = $dep2Id })
$newUserNotes = @(Test-InboxEvents "user" $ut $preflight.InboxUser $expectedUser $forbidden)
$newMitraNotes = @(Test-InboxEvents "mitra" $mt $preflight.InboxMitra $expectedMitra $forbidden)

Step "7. Saldo, riwayat, beranda, laporan, artikel"
$w1 = (Call GET "/api/v1/me/wallet" $ut).Json.data
Check "saldo akhir = awal + $($totalPts - $cost)" ([int64]$w1.balance -eq ($start + $totalPts - $cost)) ("saldo " + $w1.balance)
Check "total_spent naik $cost" ([int64]$w1.total_spent -eq ($spent0 + $cost)) ("total_spent " + $w1.total_spent)
$hist = Call GET "/api/v1/me/point-history?limit=3" $ut
$hd = @($hist.Json.data)
Check "riwayat poin terbaru: -$cost, +$pts2, +$pts1" ($hist.Status -eq 200 -and $hd.Count -eq 3 -and [int64]$hd[0].amount -eq -$cost -and $hd[0].source -eq "redemption" -and [int64]$hd[1].amount -eq $pts2 -and [int64]$hd[2].amount -eq $pts1) (Describe $hist)
$uhome = Call GET "/api/v1/me/home" $ut
$h1 = $uhome.Json.data
Check "beranda user: bentuk konsisten" ($uhome.Status -eq 200 -and (Get-HomeShapeProblems $h1 "user").Count -eq 0) ((Describe $uhome) + " " + ((Get-HomeShapeProblems $h1 "user") -join "; "))
if ($uhome.Status -eq 200) {
    Assert-Problems "beranda: deposits_claimed/total_weight_kg all-time dan monthly_deposits naik tepat 2 klaim, wallet sesuai" (Get-HomeDeltaProblems $home0 $h1 2 ($grams1 + $grams2) $totalPts $cost) | Out-Null
    $reportNowU = Call GET "/api/v1/me/activity-report" $ut
    $walletNow = Call GET "/api/v1/me/wallet" $ut
    $inboxNow = Get-InboxSnapshot $ut
    $missionsNow = Call GET "/api/v1/me/missions" $ut
    $crossReady = $reportNowU.Status -eq 200 -and $walletNow.Status -eq 200 -and $null -ne $inboxNow -and $badgesAfterTwo.Status -eq 200
    Check "sumber silang beranda tersedia" $crossReady "report/wallet/inbox/badges"
    if ($crossReady) { Assert-Problems "beranda cocok dengan badges (all-time), laporan bulan sama, wallet, dan unread inbox" (Get-HomeCrossProblems $h1 $badgesAfterTwo.Json.data $reportNowU.Json.data $walletNow.Json.data $inboxNow.Unread) | Out-Null }
    if ($missionsNow.Status -eq 200) { Assert-Problems "kartu misi beranda sama dengan /me/missions setelah klaim" (Get-HomeMissionProblems $h1.missions $missionsNow.Json.data) | Out-Null }
    Test-HomeClickCards "user" $ut $mt $h1
}
$mhome = Call GET "/api/v1/me/home" $mt
Check "beranda via /me/home untuk mitra: missions null" ($mhome.Status -eq 200 -and (Get-HomeShapeProblems $mhome.Json.data "mitra").Count -eq 0) (Describe $mhome)
$phome = Call GET "/api/v1/partner/home" $mt
$pg0 = ConvertTo-Grams $phomeBase.claimed_weight_kg
$pg1 = ConvertTo-Grams $phome.Json.data.claimed_weight_kg
Check "beranda mitra: verified, 2 setoran diklaim, poin dan berat naik sesuai" ($phome.Status -eq 200 -and $phome.Json.data.partner_status -eq "verified" -and [int64]$phome.Json.data.deposits.claimed -eq ([int64]$phomeBase.deposits.claimed + 2) -and [int64]$phome.Json.data.claimed_points -eq ([int64]$phomeBase.claimed_points + $totalPts) -and $null -ne $pg0 -and $pg1 -eq ($pg0 + $grams1 + $grams2)) (Describe $phome)

Step "7b. Laporan aktivitas setelah mutasi"
$reportAfterU = Test-ReportEndpoints "user (setelah)" $ut "user"
if ($null -ne $reportAfterU) { Assert-Problems "[user] laporan: 2 setoran, 6500 g, poin diperoleh dan dipakai naik sesuai" (Get-ReportDeltaProblems $preflight.ReportUser $reportAfterU "user" 2 ($grams1 + $grams2) $totalPts $cost) | Out-Null }
$reportAfterM = Test-ReportEndpoints "mitra (setelah)" $mt "mitra"
if ($null -ne $reportAfterM) { Assert-Problems "[mitra] laporan: 2 setoran, 6500 g, poin dikreditkan naik sesuai" (Get-ReportDeltaProblems $preflight.ReportMitra $reportAfterM "mitra" 2 ($grams1 + $grams2) $totalPts 0) | Out-Null }

Step "7c. Tandai notifikasi dibaca"
Test-InboxMarkRead "user" $ut $mt $newUserNotes

Step "7d. Bantuan (FAQ dan tiket)"
Test-SupportFlows $ut $mt $run ([bool]$CreateSupportTicket) $preflight.Unresolved

Step "8. Artikel dan autentikasi"
$arts = Call GET "/api/v1/articles?limit=10" $ut
Check "artikel demo tampil (seed-articles)" ($arts.Status -eq 200 -and $arts.Json.data.Count -ge 1) (Describe $arts)
$noauth = Call GET "/api/v1/me/wallet" "token-palsu"
Check "token palsu ditolak (401)" ($noauth.Status -eq 401) (Describe $noauth)
}
finally {
    # Always restore the temporarily enabled notification preference, then
    # verify both accounts' settings equal the baselines (never masks the error).
    try { Restore-NotificationPref $prefUser } catch { Write-Host "restore notification_enabled (user) failed" -ForegroundColor Red; $script:failed++ }
    try { Restore-NotificationPref $prefMitra } catch { Write-Host "restore notification_enabled (mitra) failed" -ForegroundColor Red; $script:failed++ }
    try {
        $fu = Call GET "/api/v1/me/settings" $ut
        $fm = Call GET "/api/v1/me/settings" $mt
        Check "akhir run: settings user dan mitra sama dengan baseline" ($fu.Status -eq 200 -and $fm.Status -eq 200 -and (Test-SettingsEqual (Get-SettingsFields $fu) $userSettings0) -and (Test-SettingsEqual (Get-SettingsFields $fm) $mitraSettings0)) ((Describe $fu) + " / " + (Describe $fm))
    }
    catch { Write-Host "final settings verification failed" -ForegroundColor Red; $script:failed++ }
}

Write-Host ""
Write-Host ("Hasil: " + $script:passed + " lulus, " + $script:failed + " gagal") -ForegroundColor $(if ($script:failed -eq 0) { "Green" } else { "Red" })
if ($script:failed -gt 0) { exit 1 }
