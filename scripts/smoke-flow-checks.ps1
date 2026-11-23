# Helper library for scripts/smoke-test.ps1 (dot-sourced; PowerShell 5.1 compatible).
# Also dot-sourced by scripts/test-smoke-flows.ps1, which replaces Call with an
# in-memory server to prove the pure validators and the preflight/restore logic.
#
# Contract with the caller: $BaseUrl (script variable) and $script:passed /
# $script:failed counters. Tokens, QR tokens, voucher codes and SQL are never
# printed by anything in this file.

$ErrorActionPreference = "Stop"

# --- Reporting and HTTP -------------------------------------------------------
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

function New-ProblemList { New-Object 'System.Collections.Generic.List[string]' }
function Add-IfNot($list, $cond, $msg) { if (-not $cond) { $list.Add([string]$msg) } }
# Records one Check from a problem list; returns $true when there is none.
function Assert-Problems($name, $problems) {
    $arr = @($problems | Where-Object { $_ })
    Check $name ($arr.Count -eq 0) ($arr -join "; ")
    $arr.Count -eq 0
}

function Convert-HeaderMap($h) {
    $map = @{}
    try {
        if ($h) {
            foreach ($k in $h.Keys) {
                $v = $h[$k]
                if ($v -is [array]) { $v = $v -join ", " }
                $map[([string]$k).ToLowerInvariant()] = [string]$v
            }
        }
    }
    catch { }
    $map
}
function Read-StreamBytes($stream) {
    $ms = New-Object IO.MemoryStream
    $stream.CopyTo($ms)
    , $ms.ToArray()
}
function New-CallResult($status, $text, $bytes, $headers) {
    $json = $null
    if ($text) { try { $json = $text | ConvertFrom-Json } catch { } }
    $h = @{}
    if ($headers) { $h = $headers }
    [pscustomobject]@{ Status = $status; Json = $json; Raw = $text; Bytes = $bytes; Headers = $h }
}

# Call returns Status/Json/Raw/Bytes/Headers and never throws on HTTP errors,
# so expected rejections (403, 409, ...) can be asserted. Headers keys are
# lower-case. Json is null for non-JSON (for example PDF) bodies.
function Call($method, $path, $token, $body, $extraHeaders) {
    $headers = @{ Authorization = "Bearer $token" }
    if ($extraHeaders) { foreach ($k in $extraHeaders.Keys) { $headers[$k] = $extraHeaders[$k] } }
    $params = @{ Method = $method; Uri = "$BaseUrl$path"; Headers = $headers; UseBasicParsing = $true }
    if ($null -ne $body) {
        $params.Body = ($body | ConvertTo-Json -Depth 6)
        $params.ContentType = "application/json"
    }
    $bytes = $null
    $text = $null
    $hmap = @{}
    try {
        $r = Invoke-WebRequest @params
        $status = [int]$r.StatusCode
        $hmap = Convert-HeaderMap $r.Headers
        if ($r.RawContentStream) {
            $r.RawContentStream.Position = 0
            $bytes = Read-StreamBytes $r.RawContentStream
        }
        if ($hmap["content-type"] -notlike "application/pdf*") {
            if ($r.Content -is [string]) { $text = $r.Content }
            elseif ($bytes) { $text = [Text.Encoding]::UTF8.GetString($bytes) }
        }
    }
    catch {
        $resp = $_.Exception.Response
        if (-not $resp) { throw }
        $status = [int]$resp.StatusCode
        try { $hmap = Convert-HeaderMap $resp.Headers } catch { }
        if ($_.ErrorDetails -and $_.ErrorDetails.Message) { $text = $_.ErrorDetails.Message }
        else { $text = (New-Object IO.StreamReader($resp.GetResponseStream())).ReadToEnd() }
    }
    New-CallResult $status $text $bytes $hmap
}

# --- Small value helpers ------------------------------------------------------
function Test-Whole($v) { ($v -is [int]) -or ($v -is [long]) }
function Test-HasProp($o, $name) { $null -ne $o -and ($o.PSObject.Properties.Name -contains $name) }
function ConvertTo-Instant($v) {
    if ($null -eq $v) { return $null }
    if ($v -is [DateTimeOffset]) { return $v }
    if ($v -is [datetime]) { return [DateTimeOffset]$v }
    try { return [DateTimeOffset]::Parse([string]$v, [Globalization.CultureInfo]::InvariantCulture, [Globalization.DateTimeStyles]::AssumeUniversal) }
    catch { return $null }
}
function Test-SameInstant($a, $b) {
    $x = ConvertTo-Instant $a
    $y = ConvertTo-Instant $b
    $null -ne $x -and $null -ne $y -and $x.UtcTicks -eq $y.UtcTicks
}
function ConvertTo-Decimal($v) {
    if ($null -eq $v) { return $null }
    try { return [decimal]::Parse([string]$v, [Globalization.CultureInfo]::InvariantCulture) } catch { return $null }
}
function Format-Kg($grams) {
    $g = [int64]$grams
    $k = [int64][math]::Floor([decimal]$g / 1000)
    $k.ToString() + "." + ($g - $k * 1000).ToString("000")
}
function ConvertTo-Grams($kgString) {
    $d = ConvertTo-Decimal $kgString
    if ($null -eq $d) { return $null }
    [int64][math]::Round($d * 1000)
}
function Get-WibYmd($instantValue) {
    $i = ConvertTo-Instant $instantValue
    if ($null -eq $i) { return $null }
    $i.ToOffset([TimeSpan]::FromHours(7)).ToString("yyyyMMdd")
}

# --- Home -------------------------------------------------------------------
$script:MissionStatuses = @("in_progress", "claimable", "claimed")

function Get-MissionListProblems($m, $prefix) {
    $p = New-ProblemList
    if ($null -eq $m) { $p.Add("$prefix missing"); return $p.ToArray() }
    Add-IfNot $p ((ConvertTo-Instant $m.period_start) -and (ConvertTo-Instant $m.period_end)) "$prefix period_start/period_end"
    Add-IfNot $p ((Test-Whole $m.weekly_xp) -and (Test-Whole $m.total_xp)) "$prefix weekly_xp/total_xp"
    Add-IfNot $p ($m.missions -is [array] -and $m.missions.Count -gt 0) "$prefix missions must be a non-empty array"
    foreach ($x in @($m.missions)) {
        Add-IfNot $p ($x.id -is [string] -and $x.id) "$prefix mission id"
        Add-IfNot $p (@("deposit", "gram") -contains $x.unit) ("$prefix mission " + $x.id + " unit")
        Add-IfNot $p ((Test-Whole $x.target) -and $x.target -gt 0 -and (Test-Whole $x.progress) -and $x.progress -ge 0 -and (Test-Whole $x.reward_xp) -and $x.reward_xp -gt 0) ("$prefix mission " + $x.id + " numbers")
        Add-IfNot $p ($script:MissionStatuses -contains $x.status) ("$prefix mission " + $x.id + " status")
        if ($x.status -eq "claimed") { Add-IfNot $p ($null -ne $x.claimed_at) ("$prefix mission " + $x.id + " claimed without claimed_at") }
        else { Add-IfNot $p ($null -eq $x.claimed_at) ("$prefix mission " + $x.id + " claimed_at set while " + $x.status) }
        if ($x.status -eq "claimable") { Add-IfNot $p ($x.progress -ge $x.target) ("$prefix mission " + $x.id + " claimable below target") }
    }
    $p.ToArray()
}

# Shape and internal consistency of GET /me/home. $role is user or mitra.
function Get-HomeShapeProblems($d, $role) {
    $p = New-ProblemList
    if ($null -eq $d) { $p.Add("data missing"); return $p.ToArray() }
    Add-IfNot $p ($d.user_name -is [string] -and $d.user_name) "user_name"
    $st = ConvertTo-Instant $d.server_time
    Add-IfNot $p ($null -ne $st) "server_time"
    $from = ConvertTo-Instant $d.month.from
    $to = ConvertTo-Instant $d.month.to
    Add-IfNot $p ($d.month.timezone -eq "Asia/Jakarta") "month.timezone must be Asia/Jakarta"
    Add-IfNot $p ($null -ne $from -and $null -ne $to -and $from.UtcTicks -le $to.UtcTicks) "month.from/to"
    if ($null -ne $from) {
        $wib = $from.ToOffset([TimeSpan]::FromHours(7))
        Add-IfNot $p ($wib.Day -eq 1 -and $wib.TimeOfDay.Ticks -eq 0) "month.from must be day 1 00:00 WIB"
    }
    Add-IfNot $p (Test-SameInstant $d.month.to $d.server_time) "month.to must equal server_time"
    $w = $d.wallet
    Add-IfNot $p ($null -ne $w -and (Test-Whole $w.balance) -and (Test-Whole $w.total_earned) -and (Test-Whole $w.total_spent) -and ($w.balance -eq $w.total_earned - $w.total_spent)) "wallet numbers (balance = earned - spent)"
    Add-IfNot $p ((Test-Whole $d.deposits_claimed) -and (Test-Whole $d.monthly_deposits) -and $d.monthly_deposits -ge 0 -and $d.deposits_claimed -ge $d.monthly_deposits) "deposits_claimed (all-time) must be a whole number >= monthly_deposits"
    Add-IfNot $p ($d.total_weight_kg -is [string] -and $d.total_weight_kg -match '^\d+\.\d{3}$') "total_weight_kg"
    Add-IfNot $p ((Test-Whole $d.active_vouchers) -and $d.active_vouchers -ge 0) "active_vouchers"
    Add-IfNot $p ((Test-Whole $d.unread_count) -and $d.unread_count -ge 0) "unread_count must be a whole number"
    Add-IfNot $p ($d.recent_activity -is [array] -and $d.recent_activity.Count -le 3) "recent_activity must be an array of at most 3"
    foreach ($a in @($d.recent_activity)) {
        Add-IfNot $p ($a.id -and (Test-Whole $a.amount) -and $a.source) "recent_activity entry"
        if ($a.source -eq "deposit") { Add-IfNot $p ($a.deposit -and $a.deposit.id) "recent_activity deposit.id" }
    }
    Add-IfNot $p (Test-HasProp $d "missions") "missions key missing"
    if ($role -eq "mitra") { Add-IfNot $p ($null -eq $d.missions) "missions must be null for mitra" }
    else { foreach ($m in @(Get-MissionListProblems $d.missions "missions")) { $p.Add($m) } }
    Add-IfNot $p ((Test-HasProp $d "partner") -and (Test-HasProp $d "article")) "partner/article keys missing"
    if ($null -ne $d.partner) { Add-IfNot $p ($d.partner.id -and $d.partner.name) "partner.id/name" }
    if ($null -ne $d.article) { Add-IfNot $p ($d.article.id -and $d.article.title) "article.id/title" }
    $p.ToArray()
}

# Home mission card must be the same catalog as GET /me/missions.
function Get-HomeMissionProblems($homeMissions, $missionsData) {
    $p = New-ProblemList
    if ($null -eq $homeMissions -or $null -eq $missionsData) { $p.Add("mission data missing"); return $p.ToArray() }
    Add-IfNot $p (Test-SameInstant $homeMissions.period_start $missionsData.period_start) "period_start differs"
    Add-IfNot $p (Test-SameInstant $homeMissions.period_end $missionsData.period_end) "period_end differs"
    Add-IfNot $p ([int64]$homeMissions.weekly_xp -eq [int64]$missionsData.weekly_xp -and [int64]$homeMissions.total_xp -eq [int64]$missionsData.total_xp) "xp differs"
    $a = @($homeMissions.missions)
    $b = @($missionsData.missions)
    Add-IfNot $p ($a.Count -eq $b.Count) "catalog size differs"
    for ($i = 0; $i -lt [math]::Min($a.Count, $b.Count); $i++) {
        $same = $a[$i].id -eq $b[$i].id -and $a[$i].unit -eq $b[$i].unit -and [int64]$a[$i].target -eq [int64]$b[$i].target -and [int64]$a[$i].progress -eq [int64]$b[$i].progress -and [int64]$a[$i].reward_xp -eq [int64]$b[$i].reward_xp -and $a[$i].status -eq $b[$i].status
        Add-IfNot $p $same ("mission #" + $i + " (" + $a[$i].id + ") differs from /me/missions")
    }
    $p.ToArray()
}

# Counters change by exactly the claimed deposits: deposits_claimed and
# total_weight_kg are all-time, monthly_deposits only counts the WIB month.
function Get-HomeDeltaProblems($before, $after, $claims, $grams, $earned, $spent) {
    $p = New-ProblemList
    Add-IfNot $p ([int64]$after.deposits_claimed -eq [int64]$before.deposits_claimed + $claims) ("deposits_claimed (all-time) " + $before.deposits_claimed + " -> " + $after.deposits_claimed + ", expected +" + $claims)
    $g0 = ConvertTo-Grams $before.total_weight_kg
    $g1 = ConvertTo-Grams $after.total_weight_kg
    Add-IfNot $p ($null -ne $g0 -and $null -ne $g1 -and $g1 -eq $g0 + $grams) ("total_weight_kg (all-time) " + $before.total_weight_kg + " -> " + $after.total_weight_kg + ", expected +" + (Format-Kg $grams))
    if (Test-SameInstant $before.month.from $after.month.from) {
        Add-IfNot $p ([int64]$after.monthly_deposits -eq [int64]$before.monthly_deposits + $claims) ("monthly_deposits " + $before.monthly_deposits + " -> " + $after.monthly_deposits + ", expected +" + $claims)
    }
    Add-IfNot $p ([int64]$after.wallet.total_earned -eq [int64]$before.wallet.total_earned + $earned) "wallet.total_earned delta"
    Add-IfNot $p ([int64]$after.wallet.total_spent -eq [int64]$before.wallet.total_spent + $spent) "wallet.total_spent delta"
    Add-IfNot $p ([int64]$after.wallet.balance -eq [int64]$before.wallet.balance + $earned - $spent) "wallet.balance delta"
    $p.ToArray()
}

# Cross-checks that do not depend on a baseline: all-time counters equal the
# badge totals, the monthly counter equals the report for the same month, and
# wallet and unread_count equal their own endpoints.
function Get-HomeCrossProblems($h, $badges, $report, $wallet, $unread) {
    $p = New-ProblemList
    if ($null -eq $h -or $null -eq $badges -or $null -eq $report -or $null -eq $wallet -or $null -eq $unread) { $p.Add("cross-check source missing"); return $p.ToArray() }
    Add-IfNot $p ([int64]$h.deposits_claimed -eq [int64]$badges.claimed_deposits) ("deposits_claimed " + $h.deposits_claimed + " != all-time badges.claimed_deposits " + $badges.claimed_deposits)
    Add-IfNot $p ($h.total_weight_kg -eq (Format-Kg ([int64]$badges.claimed_weight_grams))) ("total_weight_kg " + $h.total_weight_kg + " != all-time badges weight " + (Format-Kg ([int64]$badges.claimed_weight_grams)))
    if (Test-SameInstant $report.period.from $h.month.from) {
        Add-IfNot $p ([int64]$h.monthly_deposits -eq [int64]$report.totals.claimed_deposits) ("monthly_deposits " + $h.monthly_deposits + " != activity-report claimed_deposits " + $report.totals.claimed_deposits + " for the same month")
    }
    else { $p.Add("report period.from differs from home month.from; cannot cross-check monthly_deposits") }
    Add-IfNot $p ([int64]$h.wallet.balance -eq [int64]$wallet.balance -and [int64]$h.wallet.total_earned -eq [int64]$wallet.total_earned -and [int64]$h.wallet.total_spent -eq [int64]$wallet.total_spent) "wallet differs from /me/wallet"
    Add-IfNot $p ([int64]$h.unread_count -eq [int64]$unread) ("unread_count " + $h.unread_count + " != /me/notifications unread_count " + $unread)
    $p.ToArray()
}

# Clickable cards must open real, caller-visible detail routes with the same
# IDs; another account must not read the user's own deposit detail.
function Test-HomeClickCards($label, $token, $otherToken, $h) {
    if ($null -ne $h.partner) {
        $r = Call GET ("/api/v1/partners/" + $h.partner.id) $token
        Check "[$label] home partner.id opens /partners/{id}" ($r.Status -eq 200 -and $r.Json.data.id -eq $h.partner.id -and $r.Json.data.name -eq $h.partner.name) (Describe $r)
    }
    else { Write-Host "  [INFO] [$label] home partner is null; partner card ID not exercised" }
    if ($null -ne $h.article) {
        $r = Call GET ("/api/v1/articles/" + $h.article.id) $token
        Check "[$label] home article.id opens /articles/{id}" ($r.Status -eq 200 -and $r.Json.data.id -eq $h.article.id) (Describe $r)
    }
    else { Write-Host "  [INFO] [$label] home article is null; article card ID not exercised" }
    $n = 0
    foreach ($a in @($h.recent_activity)) {
        if ($a.source -ne "deposit" -or -not $a.deposit.id) { continue }
        $n++
        $path = "/api/v1/me/deposits/" + $a.deposit.id
        $r = Call GET $path $token
        Check "[$label] recent_activity deposit.id opens /me/deposits/{id}" ($r.Status -eq 200 -and $r.Json.data.id -eq $a.deposit.id) (Describe $r)
        $f = Call GET $path $otherToken
        Check "[$label] other account cannot read that deposit detail (404)" ($f.Status -eq 404) (Describe $f)
    }
    if ($n -eq 0) { Write-Host "  [INFO] [$label] no deposit in recent_activity; deposit card ID not exercised" }
}

# --- Activity report ----------------------------------------------------------
function Get-ReportProblems($d, $role) {
    $p = New-ProblemList
    if ($null -eq $d) { $p.Add("data missing"); return $p.ToArray() }
    Add-IfNot $p ($d.account.role -eq $role) "account.role"
    Add-IfNot $p ($d.period.timezone -eq "Asia/Jakarta") "period.timezone"
    $from = ConvertTo-Instant $d.period.from
    $to = ConvertTo-Instant $d.period.to
    Add-IfNot $p ($null -ne $from -and $null -ne $to -and $from.UtcTicks -lt $to.UtcTicks) "period.from < period.to"
    $daily = @($d.daily)
    Add-IfNot $p ($d.daily -is [array] -and $daily.Count -ge 1 -and [int64]$d.period.days -eq $daily.Count) "period.days must equal daily entries"
    $prev = $null
    $sumDep = [int64]0; $sumG = [int64]0; $sumPts = [int64]0; $sumSpent = [int64]0
    $ptsName = "points_earned"
    if ($role -eq "mitra") { $ptsName = "points_credited" }
    foreach ($day in $daily) {
        try {
            $dt = [datetime]::ParseExact([string]$day.date, "yyyy-MM-dd", [Globalization.CultureInfo]::InvariantCulture)
            if ($null -ne $prev) { Add-IfNot $p ($dt -eq $prev.AddDays(1)) ("daily date gap before " + $day.date) }
            $prev = $dt
        }
        catch { $p.Add("daily date format " + $day.date) }
        $sumDep += [int64]$day.claimed_deposits
        $sumG += [int64]$day.total_weight_grams
        Add-IfNot $p ($day.total_weight_kg -eq (Format-Kg ([int64]$day.total_weight_grams))) ("daily " + $day.date + " kg string")
        Add-IfNot $p (Test-HasProp $day $ptsName) ("daily " + $day.date + " missing " + $ptsName)
        $sumPts += [int64]$day.$ptsName
        if ($role -eq "user") { Add-IfNot $p (Test-HasProp $day "points_spent") ("daily " + $day.date + " missing points_spent"); $sumSpent += [int64]$day.points_spent }
        else { Add-IfNot $p (-not (Test-HasProp $day "points_spent")) "mitra daily must not have points_spent" }
    }
    $t = $d.totals
    Add-IfNot $p ([int64]$t.claimed_deposits -eq $sumDep) "totals.claimed_deposits != sum(daily)"
    Add-IfNot $p ([int64]$t.total_weight_grams -eq $sumG) "totals.total_weight_grams != sum(daily)"
    Add-IfNot $p ($t.total_weight_kg -eq (Format-Kg ([int64]$t.total_weight_grams))) "totals.total_weight_kg string"
    Add-IfNot $p ((Test-HasProp $t $ptsName) -and [int64]$t.$ptsName -eq $sumPts) "totals.$ptsName != sum(daily)"
    if ($role -eq "user") { Add-IfNot $p ((Test-HasProp $t "points_spent") -and [int64]$t.points_spent -eq $sumSpent) "totals.points_spent != sum(daily)" }
    else { Add-IfNot $p (-not (Test-HasProp $t "points_spent")) "mitra totals must not have points_spent" }
    $catG = [int64]0
    $lastG = [int64]::MaxValue
    foreach ($c in @($d.categories)) {
        $catG += [int64]$c.total_weight_grams
        Add-IfNot $p ([int64]$c.deposit_count -le [int64]$t.claimed_deposits) ("category " + $c.category_name + " deposit_count > claimed deposits")
        Add-IfNot $p ([int64]$c.total_weight_grams -le $lastG) "categories not sorted by weight desc"
        $lastG = [int64]$c.total_weight_grams
    }
    Add-IfNot $p ($d.categories -is [array]) "categories must be an array"
    Add-IfNot $p ($catG -eq [int64]$t.total_weight_grams) "category weights != total weight"
    $p.ToArray()
}

function Get-ReportDeltaProblems($before, $after, $role, $claims, $grams, $pts, $spent) {
    $p = New-ProblemList
    if (-not (Test-SameInstant $before.period.from $after.period.from)) { $p.Add("report month changed during the run; delta not comparable"); return $p.ToArray() }
    $name = "points_earned"
    if ($role -eq "mitra") { $name = "points_credited" }
    Add-IfNot $p ([int64]$after.totals.claimed_deposits -eq [int64]$before.totals.claimed_deposits + $claims) ("claimed_deposits " + $before.totals.claimed_deposits + " -> " + $after.totals.claimed_deposits + ", expected +" + $claims)
    Add-IfNot $p ([int64]$after.totals.total_weight_grams -eq [int64]$before.totals.total_weight_grams + $grams) "total_weight_grams delta"
    Add-IfNot $p ([int64]$after.totals.$name -eq [int64]$before.totals.$name + $pts) ("$name " + $before.totals.$name + " -> " + $after.totals.$name + ", expected +" + $pts)
    if ($role -eq "user") { Add-IfNot $p ([int64]$after.totals.points_spent -eq [int64]$before.totals.points_spent + $spent) ("points_spent " + $before.totals.points_spent + " -> " + $after.totals.points_spent + ", expected +" + $spent) }
    $p.ToArray()
}

# Validates a PDF download structurally: headers, magic, trailer and that the
# startxref offset really points at an xref table. A JSON error or a truncated
# or corrupted body fails. $expectedStartYmd is optional (WIB, yyyyMMdd).
function Get-PdfProblems($r, $expectedStartYmd) {
    $p = New-ProblemList
    if ($r.Status -ne 200) { $p.Add("HTTP " + $r.Status); return $p.ToArray() }
    $h = $r.Headers
    Add-IfNot $p ($h["content-type"] -like "application/pdf*") ("Content-Type " + $h["content-type"])
    $cd = [string]$h["content-disposition"]
    Add-IfNot $p ($cd -match '^attachment; filename="ecosnap-activity-report-(\d{8})-(\d{8})\.pdf"$') "Content-Disposition attachment filename"
    if ($cd -match 'report-(\d{8})-(\d{8})\.pdf') {
        Add-IfNot $p ($Matches[2] -ge $Matches[1]) "filename end date before start date"
        if ($expectedStartYmd) { Add-IfNot $p ($Matches[1] -eq $expectedStartYmd) ("filename start " + $Matches[1] + " != period start " + $expectedStartYmd) }
    }
    Add-IfNot $p ($h["x-content-type-options"] -eq "nosniff") "X-Content-Type-Options nosniff"
    Add-IfNot $p ([string]$h["cache-control"] -like "*no-store*") "Cache-Control no-store"
    $b = $r.Bytes
    if (-not ($b -is [byte[]]) -or $b.Length -lt 200) { $p.Add("body is not a byte payload of plausible size"); return $p.ToArray() }
    Add-IfNot $p ($h["content-length"] -eq [string]$b.Length) ("Content-Length " + $h["content-length"] + " != body " + $b.Length)
    $latin = [Text.Encoding]::GetEncoding(28591)
    Add-IfNot $p ($latin.GetString($b, 0, 5) -eq "%PDF-") "missing %PDF- magic"
    $tailLen = [math]::Min(64, $b.Length)
    $tail = $latin.GetString($b, $b.Length - $tailLen, $tailLen)
    if ($tail -match 'startxref\s+(\d+)\s+%%EOF\s*$') {
        $off = [int64]$Matches[1]
        if ($off -lt 0 -or $off + 4 -gt $b.Length) { $p.Add("startxref offset outside file") }
        else { Add-IfNot $p ($latin.GetString($b, [int]$off, 4) -eq "xref") "startxref does not point at an xref table (corrupt or truncated)" }
    }
    else { $p.Add("missing startxref/%%EOF trailer (truncated)") }
    $p.ToArray()
}

# Tests JSON + PDF for one account. Error cases must be JSON, never PDF.
# Returns the JSON report data, or $null when unavailable.
function Test-ReportEndpoints($label, $token, $role) {
    $j = Call GET "/api/v1/me/activity-report" $token
    $data = $null
    if ($j.Status -eq 200) { $data = $j.Json.data }
    Check "[$label] activity-report JSON available (200)" ($j.Status -eq 200) ((Describe $j) + " (apply migrations and restart if the route is missing)")
    if ($null -eq $data) { return $null }
    Assert-Problems "[$label] activity-report JSON consistent (period/daily/totals/categories/role fields)" (Get-ReportProblems $data $role) | Out-Null
    $pdf = Call GET "/api/v1/me/activity-report.pdf" $token
    Assert-Problems "[$label] activity-report.pdf is a structurally valid PDF download" (Get-PdfProblems $pdf (Get-WibYmd $data.period.from)) | Out-Null
    $bad = Call GET "/api/v1/me/activity-report?from=2026-01-01T00:00:00Z" $token
    Check "[$label] report with only from rejected as JSON 400 invalid_query" ($bad.Status -eq 400 -and $bad.Json.error.code -eq "invalid_query" -and $bad.Headers["content-type"] -like "application/json*") (Describe $bad)
    $badPdf = Call GET "/api/v1/me/activity-report.pdf?bogus=1" $token
    Check "[$label] PDF error stays JSON (400, not PDF)" ($badPdf.Status -eq 400 -and $badPdf.Headers["content-type"] -like "application/json*" -and $null -ne $badPdf.Json) (Describe $badPdf)
    $na = Call GET "/api/v1/me/activity-report.pdf" "token-palsu"
    Check "[$label] PDF with invalid token rejected as JSON (401)" ($na.Status -eq 401 -and $null -ne $na.Json) (Describe $na)
    $data
}

# --- Inbox --------------------------------------------------------------------
$script:NotificationTypes = @("deposit_credited", "deposit_claimed", "voucher_issued", "mission_xp_claimed")

function Get-InboxSnapshot($token) {
    $r = Call GET "/api/v1/me/notifications?limit=100" $token
    if ($r.Status -ne 200 -or $null -eq $r.Json -or -not ($r.Json.data -is [array]) -or -not (Test-Whole $r.Json.unread_count)) { return $null }
    $ids = @{}
    foreach ($n in @($r.Json.data)) { $ids[[string]$n.id] = $true }
    [pscustomobject]@{ Items = @($r.Json.data); Ids = $ids; Unread = [int64]$r.Json.unread_count }
}

function Get-NewNotifications($items, $baselineIds) {
    @($items | Where-Object { -not $baselineIds.ContainsKey([string]$_.id) })
}

# $expected: list of @{ type; target_type; target_id }. New notifications (not
# in the baseline) must match it exactly, with multiplicity, unread, safe text.
function Get-InboxProblems($items, $unreadCount, $baselineIds, $baselineUnread, $expected, $forbidden) {
    $p = New-ProblemList
    $new = @(Get-NewNotifications $items $baselineIds)
    $want = @{}
    foreach ($e in @($expected)) { $k = $e.type + "|" + $e.target_type + "|" + $e.target_id; $want[$k] = 1 + [int]$want[$k] }
    $got = @{}
    foreach ($n in $new) {
        $k = [string]$n.type + "|" + $n.target.type + "|" + $n.target.id
        $got[$k] = 1 + [int]$got[$k]
        Add-IfNot $p ($script:NotificationTypes -contains $n.type) ("unknown type " + $n.type)
        Add-IfNot $p ($n.title -is [string] -and $n.title -and $n.body -is [string] -and $n.body) ("empty title/body for " + $n.type)
        Add-IfNot $p ((Test-HasProp $n "seen_at") -and $null -eq $n.seen_at) ("new " + $n.type + " must be unread")
        Add-IfNot $p ($null -ne (ConvertTo-Instant $n.created_at)) "created_at"
        foreach ($f in @($forbidden)) { if ($f -and (([string]$n.title + " " + [string]$n.body).Contains([string]$f))) { $p.Add("notification " + $n.type + " leaks a secret") } }
    }
    foreach ($k in $want.Keys) { if ([int]$got[$k] -ne [int]$want[$k]) { $p.Add("expected " + $want[$k] + " x " + $k + ", got " + [int]$got[$k]) } }
    foreach ($k in $got.Keys) { if (-not $want.ContainsKey($k)) { $p.Add("unexpected new notification " + $k) } }
    Add-IfNot $p ([int64]$unreadCount -eq [int64]$baselineUnread + $new.Count) ("unread_count " + $baselineUnread + " -> " + $unreadCount + ", expected +" + $new.Count)
    $p.ToArray()
}

function Get-MarkReadProblems($first, $second, $id) {
    $p = New-ProblemList
    Add-IfNot $p ($first.Json.data.id -eq $id -and $null -ne $first.Json.data.seen_at) "first read must set seen_at"
    Add-IfNot $p ($second.Json.data.id -eq $id -and (Test-SameInstant $first.Json.data.seen_at $second.Json.data.seen_at)) "second read must keep the first seen_at"
    $p.ToArray()
}

# Fetches the inbox and asserts the expected new events. Returns the new items.
function Test-InboxEvents($label, $token, $snap, $expected, $forbidden) {
    $r = Call GET "/api/v1/me/notifications?limit=100" $token
    if ($r.Status -ne 200) { Check "[$label] inbox readable" $false (Describe $r); return @() }
    Assert-Problems "[$label] inbox has exactly the events from this run (type + target ID, unread, no secrets)" (Get-InboxProblems @($r.Json.data) $r.Json.unread_count $snap.Ids $snap.Unread $expected $forbidden) | Out-Null
    Get-NewNotifications @($r.Json.data) $snap.Ids
}

# Marks one of this run's own notifications read; checks idempotency, foreign
# 404 and Home unread_count refresh. Only the first new item is touched.
function Test-InboxMarkRead($label, $token, $otherToken, $newItems) {
    $items = @($newItems)
    if ($items.Count -eq 0) { Check "[$label] a new notification exists to mark read" $false "none created"; return }
    $id = [string]$items[0].id
    $path = "/api/v1/me/notifications/$id/read"
    $before = Call GET "/api/v1/me/notifications?limit=1" $token
    $foreign = Call PATCH $path $otherToken
    Check "[$label] other account cannot mark it read (404 notification_not_found)" ($foreign.Status -eq 404 -and $foreign.Json.error.code -eq "notification_not_found") (Describe $foreign)
    $stillUnread = Call GET "/api/v1/me/notifications?unread=true&limit=100" $token
    Check "[$label] foreign attempt left it unread" ($stillUnread.Status -eq 200 -and @($stillUnread.Json.data | Where-Object { $_.id -eq $id }).Count -eq 1) (Describe $stillUnread)
    $bad = Call PATCH "/api/v1/me/notifications/not-a-uuid/read" $token
    Check "[$label] malformed notification ID is 404" ($bad.Status -eq 404) (Describe $bad)
    $r1 = Call PATCH $path $token
    $r2 = Call PATCH $path $token
    Check "[$label] mark read is idempotent (seen_at set once)" ($r1.Status -eq 200 -and $r2.Status -eq 200 -and (Get-MarkReadProblems $r1 $r2 $id).Count -eq 0) ((Describe $r1) + " / " + (Describe $r2))
    $after = Call GET "/api/v1/me/notifications?limit=1" $token
    Check "[$label] unread_count dropped by exactly 1" ($before.Status -eq 200 -and $after.Status -eq 200 -and [int64]$after.Json.unread_count -eq [int64]$before.Json.unread_count - 1) ((Describe $before) + " / " + (Describe $after))
    $homeNow = Call GET "/api/v1/me/home" $token
    Check "[$label] home unread_count follows mark read" ($homeNow.Status -eq 200 -and $after.Status -eq 200 -and [int64]$homeNow.Json.data.unread_count -eq [int64]$after.Json.unread_count) (Describe $homeNow)
    $unread = Call GET "/api/v1/me/notifications?unread=true&limit=100" $token
    Check "[$label] read notification leaves unread=true list" ($unread.Status -eq 200 -and @($unread.Json.data | Where-Object { $_.id -eq $id }).Count -eq 0) (Describe $unread)
}

# Temporarily enables notification_enabled when the baseline is false so
# events are created. The state object exists BEFORE the PATCH and Changed is
# set first, so Restore-NotificationPref always runs in a finally block.
function New-NotificationPrefState($label, $token, $baseline) {
    [pscustomobject]@{ Label = $label; Token = $token; Original = [bool]$baseline.notification_enabled; Changed = $false }
}
function Enable-NotificationPref($state) {
    if ($state.Original) { return }
    $state.Changed = $true
    $r = Call PATCH "/api/v1/me/settings" $state.Token @{ notification_enabled = $true }
    if ($r.Status -ne 200) { throw ("Cannot temporarily enable notifications for " + $state.Label + ": " + (Describe $r)) }
    Write-Host ("  [INFO] [" + $state.Label + "] notification_enabled was false; enabled temporarily, restored in finally")
}
function Restore-NotificationPref($state) {
    if ($null -eq $state -or -not $state.Changed) { return }
    $r = Call PATCH "/api/v1/me/settings" $state.Token @{ notification_enabled = $state.Original }
    $g = Call GET "/api/v1/me/settings" $state.Token
    Check ("[" + $state.Label + "] notification_enabled restored to original after event checks") ($r.Status -eq 200 -and $g.Status -eq 200 -and $g.Json.data.notification_enabled -eq $state.Original) ((Describe $r) + " / " + (Describe $g))
    if ($r.Status -eq 200) { $state.Changed = $false }
}

# --- Support ------------------------------------------------------------------
function Get-FaqProblems($r) {
    $p = New-ProblemList
    if ($r.Status -ne 200) { $p.Add("HTTP " + $r.Status); return $p.ToArray() }
    $d = $r.Json.data
    Add-IfNot $p ($d -is [array] -and $d.Count -ge 1) "data must be a non-empty array"
    $seen = @{}
    foreach ($f in @($d)) {
        Add-IfNot $p ($f.id -is [string] -and $f.id -and $f.category -and $f.question -and $f.answer) "faq fields"
        Add-IfNot $p ($f.is_demo -is [bool]) ("faq " + $f.id + " is_demo bool")
        Add-IfNot $p (-not $seen.ContainsKey([string]$f.id)) ("duplicate faq id " + $f.id)
        $seen[[string]$f.id] = $true
    }
    $p.ToArray()
}

function Get-TicketProblems($t, $kind, $subject, $message) {
    $p = New-ProblemList
    if ($null -eq $t) { $p.Add("ticket missing"); return $p.ToArray() }
    Add-IfNot $p ($t.id -is [string] -and $t.id) "id"
    Add-IfNot $p ($t.kind -eq $kind -and $t.subject -eq $subject -and $t.message -eq $message) "kind/subject/message differ from request"
    Add-IfNot $p ($t.status -eq "open") ("status " + $t.status)
    Add-IfNot $p ((Test-HasProp $t "staff_response") -and $null -eq $t.staff_response -and (Test-HasProp $t "responded_at") -and $null -eq $t.responded_at) "staff_response/responded_at must be present and null"
    Add-IfNot $p ((ConvertTo-Instant $t.created_at) -and (ConvertTo-Instant $t.updated_at)) "created_at/updated_at"
    $allowed = @("id", "kind", "subject", "message", "status", "staff_response", "responded_at", "created_at", "updated_at")
    foreach ($n in $t.PSObject.Properties.Name) { if ($allowed -notcontains $n) { $p.Add("unexpected field " + $n) } }
    $p.ToArray()
}

# Open + in_progress tickets of the caller (the account-wide cap is 20).
# Returns $null when the route is unavailable.
function Get-UnresolvedTicketCount($token) {
    $count = 0
    foreach ($status in @("open", "in_progress")) {
        $cursor = $null
        for ($page = 0; $page -lt 5; $page++) {
            $path = "/api/v1/me/support-tickets?limit=100&status=$status"
            if ($cursor) { $path += "&cursor=" + [uri]::EscapeDataString([string]$cursor) }
            $r = Call GET $path $token
            if ($r.Status -ne 200 -or -not ($r.Json.data -is [array])) { return $null }
            $count += @($r.Json.data).Count
            $cursor = $r.Json.pagination.next_cursor
            if (-not $cursor) { break }
        }
    }
    $count
}

# FAQ, ticket list/detail contract and validation rejections (none of these
# create a ticket). With $create, also creates ONE synthetic problem ticket and
# checks idempotency, list/detail and foreign 404. Tickets cannot be deleted by
# users and count toward the 20 unresolved cap until staff resolves them.
function Test-SupportFlows($ut, $mt, $run, $create, $unresolvedBefore) {
    $faq = Call GET "/api/v1/help/faqs" $ut
    Assert-Problems "[user] FAQ list valid (200, unique IDs, is_demo flags)" (Get-FaqProblems $faq) | Out-Null
    $faqM = Call GET "/api/v1/help/faqs" $mt
    Check "[mitra] FAQ list available (200)" ($faqM.Status -eq 200) (Describe $faqM)
    $faqNa = Call GET "/api/v1/help/faqs" "token-palsu"
    Check "FAQ with invalid token rejected (401)" ($faqNa.Status -eq 401) (Describe $faqNa)

    $path = "/api/v1/me/support-tickets"
    $valid = @{ kind = "problem"; subject = "Smoke test"; message = "Synthetic validation probe." }
    $noKey = Call POST $path $ut $valid
    Check "ticket without Idempotency-Key rejected (400)" ($noKey.Status -eq 400) (Describe $noKey)
    $vk = @{ "Idempotency-Key" = "smoke-$run-invalid" }
    $badKind = Call POST $path $ut @{ kind = "complaint"; subject = "Smoke test"; message = "Synthetic validation probe." } $vk
    Check "ticket with unknown kind rejected (400)" ($badKind.Status -eq 400) (Describe $badKind)
    $extra = Call POST $path $ut @{ kind = "problem"; subject = "Smoke test"; message = "Synthetic validation probe."; status = "resolved" } $vk
    Check "ticket with client-supplied status rejected (400)" ($extra.Status -eq 400) (Describe $extra)
    $list = Call GET "$path`?limit=5" $ut
    Check "[user] ticket list shape (data array, pagination.limit)" ($list.Status -eq 200 -and $list.Json.data -is [array] -and (Test-Whole $list.Json.pagination.limit)) (Describe $list)
    $badQ = Call GET "$path`?status=bogus" $ut
    Check "ticket list with invalid status rejected (400)" ($badQ.Status -eq 400) (Describe $badQ)
    $ghost = Call GET ("$path/" + [guid]::NewGuid().ToString()) $ut
    Check "unknown ticket ID is 404 ticket_not_found" ($ghost.Status -eq 404 -and $ghost.Json.error.code -eq "ticket_not_found") (Describe $ghost)
    $malformed = Call GET "$path/not-a-uuid" $ut
    Check "malformed ticket ID is 404" ($malformed.Status -eq 404) (Describe $malformed)
    $na = Call GET $path "token-palsu"
    Check "ticket list with invalid token rejected (401)" ($na.Status -eq 401) (Describe $na)

    if (-not $create) { Write-Host "  [INFO] ticket creation skipped; pass -CreateSupportTicket to create one synthetic problem ticket (cannot be deleted; counts toward the 20 unresolved cap)"; return }
    $key = "smoke-$run-ticket"
    $hdr = @{ "Idempotency-Key" = $key }
    $subject = "Smoke test sintetis " + $run.Substring(0, 8)
    $message = "Tiket sintetis dari scripts/smoke-test.ps1. Aman diabaikan atau ditandai selesai."
    $body = @{ kind = "problem"; subject = $subject; message = $message }
    $c = Call POST $path $ut $body $hdr
    Check "ticket created (201) with open status and no staff response" ($c.Status -eq 201 -and (Get-TicketProblems $c.Json.data "problem" $subject $message).Count -eq 0) ((Describe $c) + " " + ((Get-TicketProblems $c.Json.data "problem" $subject $message) -join "; "))
    if ($c.Status -ne 201) { return }
    $tid = $c.Json.data.id
    $again = Call POST $path $ut $body $hdr
    Check "same key and body returns the same ticket (200, no duplicate)" ($again.Status -eq 200 -and $again.Json.data.id -eq $tid) (Describe $again)
    $conflict = Call POST $path $ut @{ kind = "problem"; subject = $subject; message = ($message + " berbeda") } $hdr
    Check "same key with different body is 409 idempotency_conflict" ($conflict.Status -eq 409 -and $conflict.Json.error.code -eq "idempotency_conflict") (Describe $conflict)
    if ($null -ne $unresolvedBefore) {
        $after = Get-UnresolvedTicketCount $ut
        Check "retry and conflict created no extra ticket (unresolved +1 only)" ($after -eq $unresolvedBefore + 1) ("before=" + $unresolvedBefore + " after=" + $after)
    }
    $l = Call GET "$path`?limit=5" $ut
    Check "ticket list contains the new ticket exactly once" ($l.Status -eq 200 -and @($l.Json.data | Where-Object { $_.id -eq $tid }).Count -eq 1) (Describe $l)
    $lo = Call GET "$path`?status=open&limit=100" $ut
    Check "status=open list contains it" ($lo.Status -eq 200 -and @($lo.Json.data | Where-Object { $_.id -eq $tid }).Count -eq 1) (Describe $lo)
    $d = Call GET "$path/$tid" $ut
    Check "ticket detail matches (200)" ($d.Status -eq 200 -and (Get-TicketProblems $d.Json.data "problem" $subject $message).Count -eq 0) (Describe $d)
    $f = Call GET "$path/$tid" $mt
    Check "other account reading the ticket gets 404 ticket_not_found" ($f.Status -eq 404 -and $f.Json.error.code -eq "ticket_not_found") (Describe $f)
    $fl = Call GET "$path`?limit=100" $mt
    Check "other account's list does not contain it" ($fl.Status -eq 200 -and @($fl.Json.data | Where-Object { $_.id -eq $tid }).Count -eq 0) (Describe $fl)
    $dn = Call GET "$path/$tid" "token-palsu"
    Check "ticket detail with invalid token rejected (401)" ($dn.Status -eq 401) (Describe $dn)
}

# --- Estimates and rewards ----------------------------------------------------
# $items: @{category_id; weight_kg}. Rates come from the response; the total
# must be floor(sum(weight * rate)) and every exact_points must match.
function Get-EstimateProblems($d, $items) {
    $p = New-ProblemList
    if ($null -eq $d) { $p.Add("data missing"); return $p.ToArray() }
    $rows = @($d.items)
    Add-IfNot $p ($rows.Count -eq @($items).Count) "item count"
    $sumExact = [decimal]0
    $sumW = [decimal]0
    foreach ($it in @($items)) {
        $row = $rows | Where-Object { $_.category_id -eq $it.category_id } | Select-Object -First 1
        if ($null -eq $row) { $p.Add("category " + $it.category_id + " missing"); continue }
        $w = ConvertTo-Decimal $row.weight_kg
        $rate = ConvertTo-Decimal $row.points_per_kg
        $exact = ConvertTo-Decimal $row.exact_points
        Add-IfNot $p ($null -ne $w -and $w -eq (ConvertTo-Decimal $it.weight_kg)) ("weight " + $row.category_name)
        Add-IfNot $p ($null -ne $rate -and $rate -gt 0) ("rate " + $row.category_name)
        if ($null -ne $w -and $null -ne $rate -and $null -ne $exact) {
            Add-IfNot $p ($exact -eq [math]::Round($w * $rate, 5)) ("exact_points " + $row.category_name)
            $sumExact += $w * $rate
            $sumW += $w
        }
        else { $p.Add("unparsable numbers for " + $row.category_name) }
    }
    Add-IfNot $p ((Test-Whole $d.estimated_points) -and [int64]$d.estimated_points -eq [int64][math]::Floor($sumExact)) ("estimated_points " + $d.estimated_points + " != floor(" + $sumExact + ")")
    Add-IfNot $p ((ConvertTo-Decimal $d.total_weight_kg) -eq $sumW) "total_weight_kg"
    $p.ToArray()
}

# Cheapest-stable choice: prefer the seed's 500-point reward, else the cheapest
# available one that the final wallet can fund.
function Select-Reward($rewards, $funds) {
    $ok = @($rewards | Where-Object { $_.available -eq $true -and [int64]$_.cost_points -gt 0 -and [int64]$_.cost_points -le $funds -and ($null -eq $_.stock -or [int64]$_.stock -gt 0) } | Sort-Object { [int64]$_.cost_points })
    if ($ok.Count -eq 0) { return $null }
    $pref = $ok | Where-Object { [int64]$_.cost_points -eq 500 } | Select-Object -First 1
    if ($pref) { return $pref }
    $ok[0]
}
function Select-ExpensiveReward($rewards, $funds) {
    @($rewards | Where-Object { $_.available -eq $true -and [int64]$_.cost_points -gt $funds } | Sort-Object { [int64]$_.cost_points }) | Select-Object -First 1
}

# --- Preflight ------------------------------------------------------------------
# Read-only (GET) contract check of every new route for both accounts, run
# BEFORE any deposit, reward or ticket mutation. The only non-GET call is a
# foreign mark-read that must answer 404 and changes nothing. Missing
# migrations (00011 notifications, 00012 support) surface as non-200 and fail
# the preflight instead of being read as zero.
function Invoke-FlowPreflight($ut, $mt, $createTicket) {
    $failedBefore = $script:failed
    $hint = " (migrations 00011/00012 applied and server restarted?)"
    $out = [ordered]@{ Ok = $false; HomeUser = $null; HomeMitra = $null; ReportUser = $null; ReportMitra = $null; InboxUser = $null; InboxMitra = $null; Unresolved = $null; Missions = $null; Badges = $null; Wallet = $null }

    $hu = Call GET "/api/v1/me/home" $ut
    Check "[user] /me/home 200 with consistent shape (monthly vs all-time, month WIB, missions)" ($hu.Status -eq 200 -and (Get-HomeShapeProblems $hu.Json.data "user").Count -eq 0) ((Describe $hu) + $hint + " " + ((Get-HomeShapeProblems $hu.Json.data "user") -join "; "))
    $hm = Call GET "/api/v1/me/home" $mt
    Check "[mitra] /me/home 200 with missions null" ($hm.Status -eq 200 -and (Get-HomeShapeProblems $hm.Json.data "mitra").Count -eq 0) ((Describe $hm) + $hint + " " + ((Get-HomeShapeProblems $hm.Json.data "mitra") -join "; "))
    $ms = Call GET "/api/v1/me/missions" $ut
    $bg = Call GET "/api/v1/me/badges" $ut
    $wl = Call GET "/api/v1/me/wallet" $ut
    Check "[user] missions, badges and wallet routes available" ($ms.Status -eq 200 -and $bg.Status -eq 200 -and $wl.Status -eq 200) ((Describe $ms) + " / " + (Describe $bg) + " / " + (Describe $wl))
    if ($hu.Status -eq 200 -and $ms.Status -eq 200) { Assert-Problems "[user] home mission card equals /me/missions" (Get-HomeMissionProblems $hu.Json.data.missions $ms.Json.data) | Out-Null }

    $ru = Test-ReportEndpoints "user" $ut "user"
    $rm = Test-ReportEndpoints "mitra" $mt "mitra"
    $iu = Get-InboxSnapshot $ut
    $im = Get-InboxSnapshot $mt
    Check "[user] /me/notifications available with unread_count" ($null -ne $iu) ("route missing or malformed" + $hint)
    Check "[mitra] /me/notifications available with unread_count" ($null -ne $im) ("route missing or malformed" + $hint)
    if ($null -ne $iu -and $hu.Status -eq 200) { Check "[user] home unread_count equals inbox unread_count" ([int64]$hu.Json.data.unread_count -eq $iu.Unread) ("home " + $hu.Json.data.unread_count + " inbox " + $iu.Unread) }
    if ($null -ne $im -and $hm.Status -eq 200) { Check "[mitra] home unread_count equals inbox unread_count" ([int64]$hm.Json.data.unread_count -eq $im.Unread) ("home " + $hm.Json.data.unread_count + " inbox " + $im.Unread) }
    if ($null -ne $iu -and $iu.Items.Count -gt 0) {
        $foreign = Call PATCH ("/api/v1/me/notifications/" + $iu.Items[0].id + "/read") $mt
        Check "[mitra] cannot mark the user's notification read (404, nothing changed)" ($foreign.Status -eq 404) (Describe $foreign)
    }
    if ($hu.Status -eq 200 -and $bg.Status -eq 200 -and $wl.Status -eq 200 -and $null -ne $ru -and $null -ne $iu) {
        Assert-Problems "[user] home counters match badges (all-time), report (month), wallet and inbox" (Get-HomeCrossProblems $hu.Json.data $bg.Json.data $ru $wl.Json.data $iu.Unread) | Out-Null
    }
    if ($hu.Status -eq 200) { Test-HomeClickCards "user" $ut $mt $hu.Json.data }

    $faq = Call GET "/api/v1/help/faqs" $ut
    Assert-Problems "[user] FAQ route available and valid" (Get-FaqProblems $faq) | Out-Null
    $unresolved = Get-UnresolvedTicketCount $ut
    Check "[user] support ticket list available" ($null -ne $unresolved) ("route missing" + $hint)
    if ($createTicket -and $null -ne $unresolved) {
        Check ("support ticket cap: " + $unresolved + " of 20 unresolved, one more fits") ($unresolved -lt 20) "the account already has 20 unresolved tickets; resolve some (staff CLI) or run without -CreateSupportTicket"
    }

    $out.HomeUser = $hu.Json.data; $out.HomeMitra = $hm.Json.data; $out.ReportUser = $ru; $out.ReportMitra = $rm
    $out.InboxUser = $iu; $out.InboxMitra = $im; $out.Unresolved = $unresolved
    $out.Missions = $ms.Json.data; $out.Badges = $bg.Json.data; $out.Wallet = $wl.Json.data
    $out.Ok = ($script:failed -eq $failedBefore)
    [pscustomobject]$out
}
