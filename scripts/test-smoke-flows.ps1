<#
.SYNOPSIS
  Offline behavioral harness for scripts/smoke-flow-checks.ps1 (PowerShell 5.1).

.DESCRIPTION
  Dot-sources the REAL helper library used by smoke-test.ps1 and replaces only
  the HTTP transport (Call) with an in-memory server holding known responses.
  No network, no database, no Firebase, no credentials. It proves that the
  actual validators and flows:
    - accept a correct Home / report / PDF / inbox / ticket contract;
    - catch swapped monthly-vs-all-time counters, wrong cross-checks, foreign
      reads, corrupted or mislabelled PDFs, missing or extra inbox events,
      non-idempotent tickets and broken routes;
    - run the preflight with ZERO state mutations (only a foreign mark-read that
      must answer 404) and fail it on a missing route/migration or a full
      ticket cap;
    - restore notification_enabled even when the guarded work throws or the
      enabling PATCH itself fails.
  Exit code 0 when every expectation holds, 1 otherwise.

.EXAMPLE
  ./scripts/test-smoke-flows.ps1
#>
$ErrorActionPreference = "Stop"

# Silence the helper's console output; this harness prints its own results.
function Write-Host { }

. (Join-Path $PSScriptRoot "smoke-flow-checks.ps1")
$BaseUrl = "http://offline.invalid"
$script:passed = 0
$script:failed = 0
$script:tests = 0
$script:testFailures = 0

function Expect($name, $cond, $detail) {
    $script:tests++
    if ($cond) { [Console]::WriteLine("  [ok]   " + $name) }
    else {
        $script:testFailures++
        [Console]::WriteLine("  [FAIL] " + $name)
        if ($detail) { [Console]::WriteLine("         " + $detail) }
    }
}
function Reset-Counters { $script:passed = 0; $script:failed = 0 }
function ConvertTo-Obj($o) { ($o | ConvertTo-Json -Depth 12 -Compress) | ConvertFrom-Json }
function Section($t) { [Console]::WriteLine(""); [Console]::WriteLine($t) }

# --- In-memory server ---------------------------------------------------------
$JsonCt = @{ "content-type" = "application/json" }
function Ok($obj, $status) {
    if (-not $status) { $status = 200 }
    New-CallResult $status (@{ data = $obj } | ConvertTo-Json -Depth 12 -Compress) $null $JsonCt
}
function Raw($hash, $status) {
    if (-not $status) { $status = 200 }
    New-CallResult $status ($hash | ConvertTo-Json -Depth 12 -Compress) $null $JsonCt
}
function Err($status, $code) { Raw @{ error = @{ code = $code; message = "x" } } $status }

function New-PdfBytes($startxrefOverride) {
    $body = "%PDF-1.3`n"
    for ($i = 0; $i -lt 20; $i++) { $body += "1 0 obj`n<< /Type /Catalog >>`nendobj`n" }
    $off = $body.Length
    if ($null -ne $startxrefOverride) { $off = $startxrefOverride }
    $xref = "xref`n0 1`n0000000000 65535 f `ntrailer`n<< /Size 1 /Root 1 0 R >>`n"
    $body = $body.Substring(0, $body.Length) + $xref + "startxref`n$off`n%%EOF`n"
    $bytes = [Text.Encoding]::GetEncoding(28591).GetBytes($body)
    , $bytes
}
function New-PdfCall($bytes, $over) {
    $h = @{ "content-type" = "application/pdf"; "content-disposition" = 'attachment; filename="ecosnap-activity-report-20261101-20261123.pdf"'; "x-content-type-options" = "nosniff"; "cache-control" = "no-store"; "content-length" = [string]$bytes.Length }
    foreach ($k in @($over.Keys)) { if ($null -eq $over[$k]) { $h.Remove($k) } else { $h[$k] = $over[$k] } }
    New-CallResult 200 $null $bytes $h
}

function New-World {
    $notes = @{
        U = New-Object System.Collections.ArrayList
        M = New-Object System.Collections.ArrayList
    }
    $null = $notes.U.Add(@{ id = "n-u-1"; type = "deposit_credited"; title = "Poin masuk"; body = "Setoran diklaim."; target = @{ type = "deposit"; id = "dep-1" }; created_at = "2026-11-23T02:00:00Z"; seen_at = $null })
    $null = $notes.U.Add(@{ id = "n-u-2"; type = "voucher_issued"; title = "Voucher"; body = "Voucher terbit."; target = @{ type = "voucher"; id = "v-1" }; created_at = "2026-11-20T02:00:00Z"; seen_at = "2026-11-20T03:00:00Z" })
    $null = $notes.M.Add(@{ id = "n-m-1"; type = "deposit_claimed"; title = "Setoran diklaim"; body = "Setoran Anda diklaim."; target = @{ type = "deposit"; id = "dep-1" }; created_at = "2026-11-23T02:00:00Z"; seen_at = $null })
    $script:W = @{
        Log = New-Object 'System.Collections.Generic.List[string]'
        Mutations = 0
        Break = @{}
        Notes = $notes
        Settings = @{
            U = @{ notification_enabled = $true; onboarding_completed = $true; show_on_leaderboard = $true }
            M = @{ notification_enabled = $true; onboarding_completed = $true; show_on_leaderboard = $true }
        }
        Tickets = New-Object System.Collections.ArrayList
        Claimed = 3; Monthly = 2
        SwapCounters = $false
        PdfBytes = (New-PdfBytes $null); PdfOver = @{}
        FailPatchOnce = $false
        ForeignMarkLeak = $false; TicketForeignLeak = $false; DepositLeak = $false
        DupOnRetry = $false; NoConflict = $false; PartnerWrongId = $false
    }
}

function Get-Unseen($role) { @($W.Notes[$role] | Where-Object { $null -eq $_.seen_at }).Count }
function Build-Missions {
    @{
        period_start = "2026-11-23T00:00:00+07:00"; period_end = "2026-11-30T00:00:00+07:00"; server_time = "2026-11-23T03:00:00Z"; weekly_xp = 0; total_xp = 0
        missions = @(
            @{ id = "first_deposit"; title = "Setor pertama"; description = "d"; metric = "m"; unit = "deposit"; target = 1; progress = 1; reward_xp = 50; status = "claimable"; claimed_at = $null },
            @{ id = "three_kilograms"; title = "3 kg"; description = "d"; metric = "m"; unit = "gram"; target = 3000; progress = 1200; reward_xp = 100; status = "in_progress"; claimed_at = $null }
        )
    }
}
function Build-Home($role) {
    $month = @{ from = "2026-11-01T00:00:00+07:00"; to = "2026-11-23T10:00:00+07:00"; timezone = "Asia/Jakarta" }
    if ($role -eq "M") {
        return @{ user_name = "Mitra"; server_time = "2026-11-23T03:00:00Z"; month = $month; wallet = @{ balance = 0; total_earned = 0; total_spent = 0; updated_at = "2026-11-23T02:00:00Z" }
            deposits_claimed = 0; total_weight_kg = "0.000"; monthly_deposits = 0; active_vouchers = 0; recent_activity = @(); missions = $null; partner = $null; article = $null; unread_count = (Get-Unseen "M") }
    }
    $claimed = $W.Claimed; $monthly = $W.Monthly
    if ($W.SwapCounters) { $claimed = $W.Monthly; $monthly = $W.Claimed }
    @{ user_name = "Siti"; server_time = "2026-11-23T03:00:00Z"; month = $month
        wallet = @{ balance = 225; total_earned = 325; total_spent = 100; updated_at = "2026-11-23T02:00:00Z" }
        deposits_claimed = $claimed; total_weight_kg = "4.500"; monthly_deposits = $monthly; active_vouchers = 1
        recent_activity = @(
            @{ id = "h1"; amount = -100; source = "redemption"; created_at = "2026-11-23T02:50:00Z"; redemption = @{ id = "r1"; reward_name = "Kopi" } },
            @{ id = "h2"; amount = 125; source = "deposit"; created_at = "2026-11-23T02:40:00Z"; deposit = @{ id = "dep-1"; partner_id = "partner-1"; partner_name = "Tunas"; total_weight_kg = "1.500" } },
            @{ id = "h3"; amount = 100; source = "deposit"; created_at = "2026-11-22T02:40:00Z"; deposit = @{ id = "dep-2"; partner_id = "partner-1"; partner_name = "Tunas"; total_weight_kg = "1.500" } }
        )
        missions = (Build-Missions)
        partner = @{ id = "partner-1"; name = "Bank Sampah Tunas" }
        article = @{ id = "art-1"; title = "Artikel" }
        unread_count = (Get-Unseen "U") }
}
function Build-Report($role) {
    $period = @{ from = "2026-11-01T00:00:00+07:00"; to = "2026-11-23T10:00:00+07:00"; timezone = "Asia/Jakarta"; days = 2 }
    if ($role -eq "M") {
        return @{ account = @{ role = "mitra"; full_name = "Mitra" }; period = $period; generated_at = "2026-11-23T03:00:00Z"
            totals = @{ claimed_deposits = 2; total_weight_grams = 3000; total_weight_kg = "3.000"; points_credited = 225 }
            daily = @(@{ date = "2026-11-22"; claimed_deposits = 1; total_weight_grams = 1500; total_weight_kg = "1.500"; points_credited = 100 }, @{ date = "2026-11-23"; claimed_deposits = 1; total_weight_grams = 1500; total_weight_kg = "1.500"; points_credited = 125 })
            categories = @(@{ category_id = "c1"; category_name = "Plastik"; deposit_count = 2; total_weight_grams = 3000; total_weight_kg = "3.000" }) }
    }
    @{ account = @{ role = "user"; full_name = "Siti" }; period = $period; generated_at = "2026-11-23T03:00:00Z"
        totals = @{ claimed_deposits = $W.Monthly; total_weight_grams = 3000; total_weight_kg = "3.000"; points_earned = 225; points_spent = 100 }
        daily = @(@{ date = "2026-11-22"; claimed_deposits = 1; total_weight_grams = 1500; total_weight_kg = "1.500"; points_earned = 100; points_spent = 0 }, @{ date = "2026-11-23"; claimed_deposits = 1; total_weight_grams = 1500; total_weight_kg = "1.500"; points_earned = 125; points_spent = 100 })
        categories = @(@{ category_id = "c1"; category_name = "Plastik"; deposit_count = 2; total_weight_grams = 3000; total_weight_kg = "3.000" }) }
}
function Convert-Ticket($t) {
    @{ id = $t.id; kind = $t.kind; subject = $t.subject; message = $t.message; status = $t.status; staff_response = $null; responded_at = $null; created_at = "2026-11-23T03:00:00Z"; updated_at = "2026-11-23T03:00:00Z" }
}

function Call($method, $path, $token, $body, $extraHeaders) {
    $W.Log.Add("$method $path")
    if ($token -ne "U" -and $token -ne "M") { return (Err 401 "unauthorized") }
    $role = $token
    $parts = $path -split '\?', 2
    $p = $parts[0]
    $q = ""
    if ($parts.Count -gt 1) { $q = $parts[1] }
    if ($W.Break.ContainsKey($p)) { return (Err 500 "internal_error") }
    $key = "$method $p"
    switch -Regex ($key) {
        '^GET /api/v1/me/home$' { return (Ok (Build-Home $role)) }
        '^GET /api/v1/me/missions$' { return (Ok (Build-Missions)) }
        '^GET /api/v1/me/badges$' { return (Ok @{ claimed_deposits = 3; claimed_weight_grams = 4500; badges = @() }) }
        '^GET /api/v1/me/wallet$' { return (Ok @{ balance = 225; total_earned = 325; total_spent = 100 }) }
        '^GET /api/v1/me/activity-report$' {
            if ($q -match 'from=|to=') { return (Err 400 "invalid_query") }
            return (Ok (Build-Report $role))
        }
        '^GET /api/v1/me/activity-report\.pdf$' {
            if ($q) { return (Err 400 "invalid_query") }
            return (New-PdfCall $W.PdfBytes $W.PdfOver)
        }
        '^GET /api/v1/me/notifications$' {
            $items = @($W.Notes[$role])
            if ($q -match 'unread=true') { $items = @($items | Where-Object { $null -eq $_.seen_at }) }
            if ($q -match 'limit=(\d+)') { $items = @($items | Select-Object -First ([int]$Matches[1])) }
            $list = New-Object System.Collections.ArrayList
            foreach ($i in $items) { $null = $list.Add($i) }
            return (Raw @{ data = $list; unread_count = (Get-Unseen $role); pagination = @{ limit = 20; next_cursor = $null } })
        }
        '^PATCH /api/v1/me/notifications/(.+)/read$' {
            $id = $Matches[1]
            foreach ($r in @("U", "M")) {
                foreach ($n in $W.Notes[$r]) {
                    if ($n.id -ne $id) { continue }
                    if ($r -ne $role -and -not $W.ForeignMarkLeak) { return (Err 404 "notification_not_found") }
                    if ($null -eq $n.seen_at) { $n.seen_at = "2026-11-23T04:00:00Z"; $W.Mutations++ }
                    return (Ok $n)
                }
            }
            return (Err 404 "notification_not_found")
        }
        '^GET /api/v1/help/faqs$' { return (Ok @(@{ id = "qr-setoran"; category = "setoran"; question = "q"; answer = "a"; is_demo = $true }, @{ id = "poin-dan-xp"; category = "poin"; question = "q"; answer = "a"; is_demo = $true })) }
        '^GET /api/v1/me/support-tickets$' {
            if ($q -match 'status=(\w+)') { if (@("open", "in_progress", "resolved") -notcontains $Matches[1]) { return (Err 400 "invalid_query") } }
            if ($q -match 'bogus|foo=') { return (Err 400 "invalid_query") }
            $list = New-Object System.Collections.ArrayList
            foreach ($t in $W.Tickets) {
                if ($t.owner -ne $role) { continue }
                if ($q -match 'status=(\w+)' -and $t.status -ne $Matches[1]) { continue }
                $null = $list.Add((Convert-Ticket $t))
            }
            return (Raw @{ data = $list; pagination = @{ limit = 20; next_cursor = $null } })
        }
        '^POST /api/v1/me/support-tickets$' {
            $k = $null
            if ($extraHeaders) { $k = $extraHeaders["Idempotency-Key"] }
            if (-not $k) { return (Err 400 "invalid_idempotency_key") }
            if ($body.ContainsKey("status") -or $body.ContainsKey("contact_email")) { return (Err 400 "invalid_request") }
            if (@("problem", "feedback") -notcontains $body.kind) { return (Err 400 "invalid_ticket") }
            $same = $null
            foreach ($t in $W.Tickets) { if ($t.owner -eq $role -and $t.key -eq $k) { $same = $t } }
            if ($same) {
                $identical = ($same.kind -eq $body.kind -and $same.subject -eq $body.subject -and $same.message -eq $body.message)
                if ($identical -and -not $W.DupOnRetry) { return (Ok (Convert-Ticket $same) 200) }
                if (-not $identical -and -not $W.NoConflict) { return (Err 409 "idempotency_conflict") }
                if (-not $identical) { return (Ok (Convert-Ticket $same) 200) }
            }
            $open = @($W.Tickets | Where-Object { $_.owner -eq $role -and $_.status -ne "resolved" }).Count
            if ($open -ge 20) { return (Err 429 "ticket_limit_reached") }
            $t = @{ id = [guid]::NewGuid().ToString(); owner = $role; key = $k; kind = $body.kind; subject = $body.subject; message = $body.message; status = "open" }
            $null = $W.Tickets.Add($t)
            $W.Mutations++
            return (Ok (Convert-Ticket $t) 201)
        }
        '^GET /api/v1/me/support-tickets/(.+)$' {
            $id = $Matches[1]
            foreach ($t in $W.Tickets) {
                if ($t.id -ne $id) { continue }
                if ($t.owner -ne $role -and -not $W.TicketForeignLeak) { break }
                return (Ok (Convert-Ticket $t))
            }
            return (Err 404 "ticket_not_found")
        }
        '^GET /api/v1/me/settings$' { return (Ok $W.Settings[$role]) }
        '^PATCH /api/v1/me/settings$' {
            if ($W.FailPatchOnce) { $W.FailPatchOnce = $false; return (Err 500 "internal_error") }
            foreach ($k in $body.Keys) { $W.Settings[$role][$k] = $body[$k] }
            $W.Mutations++
            return (Ok $W.Settings[$role])
        }
        '^GET /api/v1/partners/(.+)$' {
            $id = $Matches[1]
            if ($W.PartnerWrongId) { $id = "other-partner" }
            return (Ok @{ id = $id; name = "Bank Sampah Tunas" })
        }
        '^GET /api/v1/articles/(.+)$' { return (Ok @{ id = $Matches[1]; title = "Artikel" }) }
        '^GET /api/v1/me/deposits/(.+)$' {
            if ($role -ne "U" -and -not $W.DepositLeak) { return (Err 404 "deposit_not_found") }
            return (Ok @{ id = $Matches[1] })
        }
    }
    Err 404 "not_found"
}

# --- 1. Home counters and contract ---------------------------------------------
Section "Home: monthly vs all-time mapping"
New-World
$before = ConvertTo-Obj (Build-Home "U")
$after = ConvertTo-Obj (Build-Home "U")
$after.deposits_claimed = 5; $after.monthly_deposits = 4; $after.total_weight_kg = "11.000"
$after.wallet.total_earned = 965; $after.wallet.total_spent = 600; $after.wallet.balance = 365
Expect "correct +2 claims/+6.500 kg/+640/-500 deltas pass" ((Get-HomeDeltaProblems $before $after 2 6500 640 500).Count -eq 0) ((Get-HomeDeltaProblems $before $after 2 6500 640 500) -join "; ")
$swapped = ConvertTo-Obj $after; $swapped.deposits_claimed = 4; $swapped.monthly_deposits = 5
Expect "swapped monthly/all-time is caught" ((Get-HomeDeltaProblems $before $swapped 2 6500 640 500).Count -gt 0)
$monthlyAsAll = ConvertTo-Obj $after; $monthlyAsAll.monthly_deposits = 5
Expect "monthly_deposits counting all-time is caught" ((Get-HomeDeltaProblems $before $monthlyAsAll 2 6500 640 500).Count -gt 0)
$wrongWallet = ConvertTo-Obj $after; $wrongWallet.wallet.balance = 366
Expect "wrong wallet balance delta is caught" ((Get-HomeDeltaProblems $before $wrongWallet 2 6500 640 500).Count -gt 0)

$badges = ConvertTo-Obj @{ claimed_deposits = 3; claimed_weight_grams = 4500 }
$report = ConvertTo-Obj (Build-Report "U")
$wallet = ConvertTo-Obj @{ balance = 225; total_earned = 325; total_spent = 100 }
Expect "cross-check passes for consistent sources" ((Get-HomeCrossProblems $before $badges $report $wallet 1).Count -eq 0) ((Get-HomeCrossProblems $before $badges $report $wallet 1) -join "; ")
$crossBad = ConvertTo-Obj $before; $crossBad.deposits_claimed = 2; $crossBad.monthly_deposits = 3
Expect "cross-check catches swapped counters against badges and report" ((Get-HomeCrossProblems $crossBad $badges $report $wallet 1).Count -ge 2)
Expect "cross-check catches wrong unread_count" ((Get-HomeCrossProblems $before $badges $report $wallet 7).Count -eq 1)
$otherMonthReport = ConvertTo-Obj $report; $otherMonthReport.period.from = "2026-10-01T00:00:00+07:00"
Expect "cross-check refuses a report from another month" ((Get-HomeCrossProblems $before $badges $otherMonthReport $wallet 1).Count -gt 0)

Expect "user home shape is valid" ((Get-HomeShapeProblems $before "user").Count -eq 0) ((Get-HomeShapeProblems $before "user") -join "; ")
$mHome = ConvertTo-Obj (Build-Home "M")
Expect "mitra home shape is valid (missions null)" ((Get-HomeShapeProblems $mHome "mitra").Count -eq 0) ((Get-HomeShapeProblems $mHome "mitra") -join "; ")
$mBad = ConvertTo-Obj $mHome; $mBad.missions = (Build-Missions)
Expect "mitra home with missions is rejected" ((Get-HomeShapeProblems (ConvertTo-Obj @{ x = 1 }) "mitra").Count -gt 0 -and (Get-HomeShapeProblems $mBad "mitra").Count -gt 0)
$bad = ConvertTo-Obj $before; $bad.month.to = "2026-11-23T11:00:00+07:00"
Expect "month.to different from server_time is rejected" ((Get-HomeShapeProblems $bad "user").Count -gt 0)
$bad = ConvertTo-Obj $before; $bad.PSObject.Properties.Remove("unread_count")
Expect "missing unread_count is rejected (no zero fallback)" ((Get-HomeShapeProblems $bad "user").Count -gt 0)
$bad = ConvertTo-Obj $before; $bad.deposits_claimed = 1
Expect "monthly_deposits above all-time is rejected" ((Get-HomeShapeProblems $bad "user").Count -gt 0)
$bad = ConvertTo-Obj $before; $bad.month.from = "2026-11-02T00:00:00+07:00"
Expect "month.from not at WIB day 1 is rejected" ((Get-HomeShapeProblems $bad "user").Count -gt 0)

$hm = ConvertTo-Obj (Build-Home "U")
$ms = ConvertTo-Obj (Build-Missions)
Expect "home mission card equal to /me/missions passes" ((Get-HomeMissionProblems $hm.missions $ms).Count -eq 0)
$ms2 = ConvertTo-Obj $ms; $ms2.missions[1].progress = 1300
Expect "home mission card out of sync is caught" ((Get-HomeMissionProblems $hm.missions $ms2).Count -gt 0)

# --- 2. Click cards and foreign reads ------------------------------------------
Section "Home click cards"
New-World; Reset-Counters
Test-HomeClickCards "user" "U" "M" (ConvertTo-Obj (Build-Home "U"))
Expect "partner, article and deposit IDs open their details; foreign deposit read is 404" ($script:failed -eq 0 -and $script:passed -ge 6) ("failed=" + $script:failed + " passed=" + $script:passed)
New-World; Reset-Counters; $W.DepositLeak = $true
Test-HomeClickCards "user" "U" "M" (ConvertTo-Obj (Build-Home "U"))
Expect "a foreign read of the deposit detail is detected" ($script:failed -gt 0)
New-World; Reset-Counters; $W.PartnerWrongId = $true
Test-HomeClickCards "user" "U" "M" (ConvertTo-Obj (Build-Home "U"))
Expect "a partner card ID that opens another partner is detected" ($script:failed -gt 0)

# --- 3. PDF ------------------------------------------------------------------------
Section "PDF validation"
$good = New-PdfBytes $null
Expect "valid PDF passes" ((Get-PdfProblems (New-PdfCall $good @{}) "20261101").Count -eq 0) ((Get-PdfProblems (New-PdfCall $good @{}) "20261101") -join "; ")
$trunc = [byte[]]($good[0..($good.Length - 25)])
Expect "truncated PDF is caught" ((Get-PdfProblems (New-PdfCall $trunc @{}) $null).Count -gt 0)
Expect "startxref pointing at non-xref data is caught" ((Get-PdfProblems (New-PdfCall (New-PdfBytes 3) @{}) $null).Count -gt 0)
$json = [Text.Encoding]::UTF8.GetBytes('{"error":{"code":"internal_error","message":"' + ('x' * 300) + '"}}')
Expect "JSON body served as application/pdf is caught" ((Get-PdfProblems (New-PdfCall $json @{}) $null).Count -gt 0)
Expect "Content-Length mismatch is caught" ((Get-PdfProblems (New-PdfCall $good @{ "content-length" = "10" }) $null).Count -gt 0)
Expect "wrong Content-Type is caught" ((Get-PdfProblems (New-PdfCall $good @{ "content-type" = "application/octet-stream" }) $null).Count -gt 0)
Expect "missing nosniff is caught" ((Get-PdfProblems (New-PdfCall $good @{ "x-content-type-options" = $null }) $null).Count -gt 0)
Expect "missing no-store is caught" ((Get-PdfProblems (New-PdfCall $good @{ "cache-control" = "public" }) $null).Count -gt 0)
Expect "inline Content-Disposition is caught" ((Get-PdfProblems (New-PdfCall $good @{ "content-disposition" = 'inline; filename="x.pdf"' }) $null).Count -gt 0)
Expect "filename start date different from the period start is caught" ((Get-PdfProblems (New-PdfCall $good @{}) "20261102").Count -gt 0)
$err = New-CallResult 500 '{"error":{"code":"internal_error"}}' $null @{ "content-type" = "application/json" }
Expect "non-200 is caught" ((Get-PdfProblems $err $null).Count -gt 0)

Section "Report JSON"
New-World
$ru = ConvertTo-Obj (Build-Report "U")
$rm = ConvertTo-Obj (Build-Report "M")
Expect "user report is consistent" ((Get-ReportProblems $ru "user").Count -eq 0) ((Get-ReportProblems $ru "user") -join "; ")
Expect "mitra report is consistent" ((Get-ReportProblems $rm "mitra").Count -eq 0) ((Get-ReportProblems $rm "mitra") -join "; ")
$bad = ConvertTo-Obj $ru; $bad.totals.claimed_deposits = 3
Expect "totals different from sum(daily) are caught" ((Get-ReportProblems $bad "user").Count -gt 0)
$bad = ConvertTo-Obj $ru; $bad.daily[1].date = "2026-11-25"
Expect "a gap in daily dates is caught" ((Get-ReportProblems $bad "user").Count -gt 0)
$bad = ConvertTo-Obj $rm; $bad.totals | Add-Member -NotePropertyName points_spent -NotePropertyValue 0
Expect "points_spent on a mitra report is caught" ((Get-ReportProblems $bad "mitra").Count -gt 0)
$bad = ConvertTo-Obj $ru; $bad.categories[0].total_weight_grams = 2900
Expect "category weight different from total is caught" ((Get-ReportProblems $bad "user").Count -gt 0)
$ru2 = ConvertTo-Obj $ru; $ru2.totals.claimed_deposits = 4; $ru2.totals.total_weight_grams = 9500; $ru2.totals.points_earned = 865; $ru2.totals.points_spent = 600
Expect "report delta (+2 claims, +6500 g, +640, +500 spent) passes" ((Get-ReportDeltaProblems $ru $ru2 "user" 2 6500 640 500).Count -eq 0) ((Get-ReportDeltaProblems $ru $ru2 "user" 2 6500 640 500) -join "; ")
Expect "report delta with wrong points is caught" ((Get-ReportDeltaProblems $ru $ru2 "user" 2 6500 600 500).Count -gt 0)

Section "Report endpoints"
New-World; Reset-Counters
$data = Test-ReportEndpoints "user" "U" "user"
Expect "report JSON + PDF + error cases + 401 pass" ($script:failed -eq 0 -and $null -ne $data) ("failed=" + $script:failed)
New-World; Reset-Counters; $W.PdfBytes = [byte[]]((New-PdfBytes $null)[0..100])
$null = Test-ReportEndpoints "user" "U" "user"
Expect "a corrupted PDF fails the endpoint check" ($script:failed -gt 0)

# --- 4. Inbox -------------------------------------------------------------------
Section "Inbox events"
$base = @{ "n-u-1" = $true; "n-u-2" = $true }
$items = ConvertTo-Obj @(
    @{ id = "new-1"; type = "deposit_credited"; title = "t"; body = "b"; target = @{ type = "deposit"; id = "D1" }; created_at = "2026-11-23T03:00:00Z"; seen_at = $null },
    @{ id = "new-2"; type = "deposit_credited"; title = "t"; body = "b"; target = @{ type = "deposit"; id = "D2" }; created_at = "2026-11-23T03:01:00Z"; seen_at = $null },
    @{ id = "new-3"; type = "voucher_issued"; title = "t"; body = "b"; target = @{ type = "voucher"; id = "V1" }; created_at = "2026-11-23T03:02:00Z"; seen_at = $null },
    @{ id = "n-u-1"; type = "deposit_credited"; title = "t"; body = "b"; target = @{ type = "deposit"; id = "OLD" }; created_at = "2026-11-22T03:02:00Z"; seen_at = $null }
)
$exp = @(@{ type = "deposit_credited"; target_type = "deposit"; target_id = "D1" }, @{ type = "deposit_credited"; target_type = "deposit"; target_id = "D2" }, @{ type = "voucher_issued"; target_type = "voucher"; target_id = "V1" })
Expect "exact expected events pass (type, target, unread, unread_count +3)" ((Get-InboxProblems $items 4 $base 1 $exp @("QRTOKEN")).Count -eq 0) ((Get-InboxProblems $items 4 $base 1 $exp @("QRTOKEN")) -join "; ")
Expect "missing event is caught" ((Get-InboxProblems $items 4 $base 1 ($exp + @{ type = "mission_xp_claimed"; target_type = "mission"; target_id = "first_deposit" }) @()).Count -gt 0)
Expect "extra event is caught" ((Get-InboxProblems $items 4 $base 1 $exp[0..1] @()).Count -gt 0)
$wrongTarget = @(@{ type = "deposit_credited"; target_type = "deposit"; target_id = "D1" }, @{ type = "deposit_credited"; target_type = "deposit"; target_id = "WRONG" }, $exp[2])
Expect "wrong target ID is caught" ((Get-InboxProblems $items 4 $base 1 $wrongTarget @()).Count -gt 0)
Expect "wrong unread_count is caught" ((Get-InboxProblems $items 9 $base 1 $exp @()).Count -gt 0)
$leak = ConvertTo-Obj $items; $leak[0].body = "kode QRTOKEN bocor"
Expect "a secret in title/body is caught" ((Get-InboxProblems $leak 4 $base 1 $exp @("QRTOKEN")).Count -gt 0)
$seen = ConvertTo-Obj $items; $seen[2].seen_at = "2026-11-23T03:05:00Z"
Expect "a new notification that is already read is caught" ((Get-InboxProblems $seen 4 $base 1 $exp @()).Count -gt 0)

$first = New-CallResult 200 '{"data":{"id":"n1","seen_at":"2026-11-23T04:00:00Z"}}' $null @{}
$second = New-CallResult 200 '{"data":{"id":"n1","seen_at":"2026-11-23T04:00:00Z"}}' $null @{}
$third = New-CallResult 200 '{"data":{"id":"n1","seen_at":"2026-11-23T04:00:09Z"}}' $null @{}
Expect "idempotent mark-read passes" ((Get-MarkReadProblems $first $second "n1").Count -eq 0)
Expect "mark-read that moves seen_at is caught" ((Get-MarkReadProblems $first $third "n1").Count -gt 0)

New-World; Reset-Counters
$snap = Get-InboxSnapshot "U"
Test-InboxMarkRead "user" "U" "M" @($snap.Items | Where-Object { $null -eq $_.seen_at })
Expect "mark-read flow passes: foreign 404, idempotent, unread -1, home follows" ($script:failed -eq 0 -and $W.Notes.U[0].seen_at -and $null -eq $W.Notes.M[0].seen_at) ("failed=" + $script:failed)
New-World; Reset-Counters; $W.ForeignMarkLeak = $true
$snap = Get-InboxSnapshot "U"
Test-InboxMarkRead "user" "U" "M" @($snap.Items | Where-Object { $null -eq $_.seen_at })
Expect "another account marking the user's notification read is detected" ($script:failed -gt 0)

# --- 5. Estimates and rewards ------------------------------------------------------
Section "Estimates and rewards"
$items1 = @(@{ category_id = "plastic"; weight_kg = "1.000" }, @{ category_id = "paper"; weight_kg = "0.500" })
$est = ConvertTo-Obj @{ estimated_points = 190; total_weight_kg = "1.500"; items = @(
        @{ category_id = "plastic"; category_name = "Plastik"; weight_kg = "1.000"; points_per_kg = "150.00"; exact_points = "150.00000" },
        @{ category_id = "paper"; category_name = "Kertas"; weight_kg = "0.500"; points_per_kg = "80.00"; exact_points = "40.00000" }) }
Expect "estimate from live rates (150 + 80/kg => 190) passes" ((Get-EstimateProblems $est $items1).Count -eq 0) ((Get-EstimateProblems $est $items1) -join "; ")
$bad = ConvertTo-Obj $est; $bad.estimated_points = 140
Expect "a stale hard-coded 140 total is caught" ((Get-EstimateProblems $bad $items1).Count -gt 0)
$frac = ConvertTo-Obj @{ estimated_points = 1; total_weight_kg = "0.007"; items = @(@{ category_id = "paper"; category_name = "Kertas"; weight_kg = "0.007"; points_per_kg = "80.00"; exact_points = "0.56000" }) }
Expect "rounding up instead of flooring is caught" ((Get-EstimateProblems $frac @(@{ category_id = "paper"; weight_kg = "0.007" })).Count -gt 0)
$rewards = ConvertTo-Obj @(
    @{ id = "r-big"; cost_points = 100000; stock = 5; available = $true },
    @{ id = "r-500"; cost_points = 500; stock = 20; available = $true },
    @{ id = "r-300-out"; cost_points = 300; stock = 0; available = $false },
    @{ id = "r-350"; cost_points = 350; stock = 3; available = $true })
Expect "prefers the 500-point seed reward when affordable" ((Select-Reward $rewards 1000).id -eq "r-500")
Expect "otherwise picks the cheapest affordable in-stock reward (skips sold out)" ((Select-Reward $rewards 400).id -eq "r-350")
Expect "no affordable reward returns null before any deposit" ($null -eq (Select-Reward $rewards 100))
Expect "insufficient-points candidate is the cheapest unaffordable reward" ((Select-ExpensiveReward $rewards 1000).id -eq "r-big")

# --- 6. Preferences restore ----------------------------------------------------------
Section "notification_enabled restore"
New-World; Reset-Counters
$W.Settings.U.notification_enabled = $false
$state = New-NotificationPrefState "user" "U" (ConvertTo-Obj $W.Settings.U)
try { Enable-NotificationPref $state; Expect "disabled baseline is temporarily enabled" ($W.Settings.U.notification_enabled -eq $true); throw "simulated failure mid-run" }
catch { }
finally { Restore-NotificationPref $state }
Expect "restored to false after a failure inside the guarded work" ($W.Settings.U.notification_enabled -eq $false -and $script:failed -eq 0)
New-World; Reset-Counters
$W.Settings.U.notification_enabled = $false
$state = New-NotificationPrefState "user" "U" (ConvertTo-Obj $W.Settings.U)
$W.FailPatchOnce = $true
try { Enable-NotificationPref $state } catch { }
Restore-NotificationPref $state
Expect "restore still runs when the enabling PATCH itself failed" ($W.Settings.U.notification_enabled -eq $false -and $script:failed -eq 0 -and $W.Log.Count -ge 3)
New-World; Reset-Counters
$state = New-NotificationPrefState "user" "U" (ConvertTo-Obj $W.Settings.U)
Enable-NotificationPref $state
Restore-NotificationPref $state
Expect "an enabled baseline is never touched (no PATCH)" ($W.Log.Count -eq 0 -and $W.Mutations -eq 0)

# --- 7. Preflight -----------------------------------------------------------------------
Section "Preflight (read-only, fail fast)"
function Get-NonGet { @($W.Log | Where-Object { $_ -notlike "GET *" }) }
New-World; Reset-Counters
$snapNotes = ($W.Notes.U | ForEach-Object { [string]$_.seen_at }) -join ","
$pf = Invoke-FlowPreflight "U" "M" $true
Expect "healthy preflight is OK" ($pf.Ok -and $script:failed -eq 0) ("failed=" + $script:failed)
Expect "healthy preflight mutated nothing" ($W.Mutations -eq 0 -and $W.Tickets.Count -eq 0 -and (($W.Notes.U | ForEach-Object { [string]$_.seen_at }) -join ",") -eq $snapNotes -and $W.Settings.U.notification_enabled)
Expect "the only non-GET call is the foreign mark-read that answers 404" ((Get-NonGet).Count -eq 1 -and @((Get-NonGet) | Where-Object { $_ -match '^PATCH /api/v1/me/notifications/[^/]+/read$' }).Count -eq 1)
Expect "preflight returns baselines for later deltas" ($pf.HomeUser.deposits_claimed -eq 3 -and $pf.InboxUser.Unread -eq 1 -and $pf.Unresolved -eq 0 -and $null -ne $pf.ReportMitra)

foreach ($route in @("/api/v1/me/notifications", "/api/v1/me/home", "/api/v1/help/faqs", "/api/v1/me/support-tickets", "/api/v1/me/activity-report", "/api/v1/me/activity-report.pdf")) {
    New-World; Reset-Counters
    $W.Break[$route] = $true
    $pf = Invoke-FlowPreflight "U" "M" $false
    Expect ("missing/broken route $route fails the preflight with no mutation") (-not $pf.Ok -and $W.Mutations -eq 0 -and (Get-NonGet).Count -le 1)
}
New-World; Reset-Counters; $W.SwapCounters = $true
$pf = Invoke-FlowPreflight "U" "M" $false
Expect "a Home that swaps monthly and all-time fails the preflight" (-not $pf.Ok)
New-World; Reset-Counters; $W.PdfOver = @{ "x-content-type-options" = $null }
$pf = Invoke-FlowPreflight "U" "M" $false
Expect "a PDF without nosniff fails the preflight" (-not $pf.Ok)
New-World; Reset-Counters
for ($i = 0; $i -lt 20; $i++) { $null = $W.Tickets.Add(@{ id = [guid]::NewGuid().ToString(); owner = "U"; key = "k$i"; kind = "problem"; subject = "s"; message = "m"; status = "open" }) }
$pf = Invoke-FlowPreflight "U" "M" $true
Expect "20 unresolved tickets reject the run before any mutation when a ticket will be created" (-not $pf.Ok -and $pf.Unresolved -eq 20 -and $W.Mutations -eq 0)
New-World; Reset-Counters
for ($i = 0; $i -lt 20; $i++) { $null = $W.Tickets.Add(@{ id = [guid]::NewGuid().ToString(); owner = "U"; key = "k$i"; kind = "problem"; subject = "s"; message = "m"; status = "open" }) }
$pf = Invoke-FlowPreflight "U" "M" $false
Expect "the cap is irrelevant when no ticket will be created" ($pf.Ok)

# --- 8. Support ----------------------------------------------------------------------------
Section "Support flows"
$run = "0123456789abcdef0123456789abcdef"
New-World; Reset-Counters
Test-SupportFlows "U" "M" $run $false 0
Expect "read-only support checks pass and create no ticket" ($script:failed -eq 0 -and $W.Tickets.Count -eq 0 -and $W.Mutations -eq 0) ("failed=" + $script:failed)
New-World; Reset-Counters
Test-SupportFlows "U" "M" $run $true 0
Expect "ticket lifecycle passes with exactly one ticket created" ($script:failed -eq 0 -and $W.Tickets.Count -eq 1 -and $W.Mutations -eq 1) ("failed=" + $script:failed + " tickets=" + $W.Tickets.Count)
New-World; Reset-Counters; $W.DupOnRetry = $true
Test-SupportFlows "U" "M" $run $true 0
Expect "a retry that creates a duplicate ticket is detected" ($script:failed -gt 0)
New-World; Reset-Counters; $W.NoConflict = $true
Test-SupportFlows "U" "M" $run $true 0
Expect "a missing idempotency conflict (409) is detected" ($script:failed -gt 0)
New-World; Reset-Counters; $W.TicketForeignLeak = $true
Test-SupportFlows "U" "M" $run $true 0
Expect "a foreign ticket read that returns 200 is detected" ($script:failed -gt 0)
$tk = ConvertTo-Obj (Convert-Ticket @{ id = "t1"; kind = "problem"; subject = "S"; message = "M"; status = "open" })
Expect "ticket shape and open status validated" ((Get-TicketProblems $tk "problem" "S" "M").Count -eq 0)
$tk2 = ConvertTo-Obj $tk; $tk2 | Add-Member -NotePropertyName contact_email -NotePropertyValue "a@b.c"
Expect "a leaked extra field (contact_email) is caught" ((Get-TicketProblems $tk2 "problem" "S" "M").Count -gt 0)

[Console]::WriteLine("")
[Console]::WriteLine("Hasil harness: " + ($script:tests - $script:testFailures) + " lulus, " + $script:testFailures + " gagal dari " + $script:tests)
if ($script:testFailures -gt 0) { exit 1 }
exit 0
