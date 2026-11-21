package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

func TestActivityReportIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	depositRepo := postgres.NewDepositRepository(pool)
	deposits := service.NewDepositService(depositRepo)
	qr := service.NewDepositQRService(depositRepo)
	rewards := service.NewRewardService(postgres.NewRewardRepository(pool))
	now := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	reports := service.NewActivityReportService(postgres.NewActivityReportRepository(pool)).WithClock(func() time.Time { return now })

	n := 0
	deposit := func(items ...entity.PointEstimateInput) entity.Deposit {
		t.Helper()
		n++
		d, _, err := deposits.Create(ctx, "owner-a", fmt.Sprintf("report-key-%04d", n), entity.DepositCreateRequest{Items: items})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	// deposit_claims is immutable (migration 00010), so a historical claim must
	// carry its timestamp at INSERT. This mirrors DepositRepository.ClaimQR
	// (claim, ledger, wallet, token use, deposit status in one transaction)
	// with the claim instant supplied instead of clock_timestamp().
	claim := func(d entity.Deposit, uid string, at string) {
		t.Helper()
		if _, err := qr.Issue(ctx, "owner-a", d.ID); err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
		var tokenID string
		var points int64
		if err = tx.QueryRow(ctx, `SELECT t.id::text,d.total_points FROM public.deposit_qr_tokens t JOIN public.deposits d ON d.id=t.deposit_id
WHERE t.deposit_id=$1::uuid AND t.revoked_at IS NULL AND t.used_at IS NULL FOR UPDATE OF t,d`, d.ID).Scan(&tokenID, &points); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.deposit_claims(deposit_id,qr_token_id,claimer_uid,points,claimed_at) VALUES($1::uuid,$2::uuid,$3,$4,$5::timestamptz)`, d.ID, tokenID, uid, points, at); err != nil {
			t.Fatal(err)
		}
		if points > 0 {
			if _, err = tx.Exec(ctx, `INSERT INTO public.point_ledger(id,user_uid,amount,source,deposit_id,idempotency_key,created_at) VALUES(gen_random_uuid(),$1,$2,'deposit',$3::uuid,'deposit:'||$3,$4::timestamptz)`, uid, points, d.ID, at); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE public.wallets SET balance=balance+$2,total_earned=total_earned+$2,updated_at=$3::timestamptz WHERE user_uid=$1`, uid, points, at); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE public.deposit_qr_tokens SET issued_at=$2::timestamptz-interval '1 minute',used_at=$2::timestamptz WHERE id=$1::uuid`, tokenID, at); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE public.deposits SET status='claimed' WHERE id=$1::uuid`, d.ID); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// d1: two items (plastic 150 + paper 40 = 190 pts, 1.500 kg), claimed at
	// the very last microsecond of 2026-11-01 WIB. Created years earlier to
	// prove the report follows claim time, not creation time.
	d1 := deposit(entity.PointEstimateInput{CategoryID: plastic, WeightKG: "1"}, entity.PointEstimateInput{CategoryID: paper, WeightKG: "0.5"})
	claim(d1, "plain-user", "2026-11-01T16:59:59.999999Z")
	if _, err := pool.Exec(ctx, `UPDATE public.deposits SET created_at='2020-01-01T00:00:00Z' WHERE id=$1::uuid`, d1.ID); err != nil {
		t.Fatal(err)
	}
	// d2: 2 kg plastic = 300 pts, claimed at exactly 2026-11-02 00:00 WIB.
	d2 := deposit(entity.PointEstimateInput{CategoryID: plastic, WeightKG: "2"})
	claim(d2, "plain-user", "2026-11-01T17:00:00Z")
	// d3: another user, 150 pts, claimed 2026-11-05 07:00 WIB.
	d3 := deposit(entity.PointEstimateInput{CategoryID: plastic, WeightKG: "1"})
	claim(d3, "user-2", "2026-11-05T00:00:00Z")
	// Never claimed: must not appear anywhere.
	d4 := deposit(entity.PointEstimateInput{CategoryID: plastic, WeightKG: "9"})
	if _, err := qr.Issue(ctx, "owner-a", d4.ID); err != nil { // qr_issued, unclaimed
		t.Fatal(err)
	}
	deposit(entity.PointEstimateInput{CategoryID: paper, WeightKG: "7"}) // created, unclaimed

	// A redemption of 100 points on 2026-11-02 WIB.
	if _, err := pool.Exec(ctx, `INSERT INTO public.rewards(id,name,cost_points,stock,is_active,is_demo) VALUES ('80000000-0000-4000-8000-0000000000f1','Pulsa',100,5,true,true)`); err != nil {
		t.Fatal(err)
	}
	red, _, err := rewards.Redeem(ctx, "plain-user", "80000000-0000-4000-8000-0000000000f1", "report-redeem-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.point_ledger SET created_at='2026-11-02T10:00:00Z' WHERE redemption_id=$1::uuid`, red.Voucher.ID); err != nil {
		t.Fatal(err)
	}
	// Renaming a category later must not rewrite history.
	if _, err = pool.Exec(ctx, `UPDATE public.waste_categories SET name='Plastik Baru' WHERE id=$1::uuid`, plastic); err != nil {
		t.Fatal(err)
	}

	wibT := func(s string) *time.Time {
		tm, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return &tm
	}
	q := service.ActivityReportQuery{From: wibT("2026-11-01T00:00:00+07:00"), To: wibT("2026-11-04T00:00:00+07:00")}
	get := func(t *testing.T, uid string, q service.ActivityReportQuery) entity.ActivityReport {
		t.Helper()
		r, err := reports.Report(ctx, uid, q)
		if err != nil {
			t.Fatalf("%s: %v", uid, err)
		}
		return r
	}
	sums := func(r entity.ActivityReport) (int64, int64) {
		var c, g int64
		for _, d := range r.Daily {
			c, g = c+d.ClaimedDeposits, g+d.TotalWeightGram
		}
		return c, g
	}

	t.Run("user_totals_daily_categories_and_boundaries", func(t *testing.T) {
		r := get(t, "plain-user", q)
		if r.Account.Role != "user" || r.Totals.ClaimedDeposits != 2 || r.Totals.TotalWeightGram != 3500 || r.Totals.TotalWeightKG != "3.500" ||
			*r.Totals.PointsEarned != 490 || *r.Totals.PointsSpent != 100 || r.Totals.PointsCredited != nil {
			t.Fatalf("totals: %+v", r.Totals)
		}
		if c, g := sums(r); c != 2 || g != 3500 {
			t.Fatalf("daily does not add up: %d %d", c, g)
		}
		if len(r.Daily) != 3 {
			t.Fatalf("daily: %+v", r.Daily)
		}
		d1, d2, d3 := r.Daily[0], r.Daily[1], r.Daily[2]
		if d1.Date != "2026-11-01" || d1.ClaimedDeposits != 1 || d1.TotalWeightKG != "1.500" || *d1.PointsEarned != 190 || *d1.PointsSpent != 0 {
			t.Fatalf("day 1 (last microsecond of the day): %+v", d1)
		}
		if d2.Date != "2026-11-02" || d2.ClaimedDeposits != 1 || d2.TotalWeightKG != "2.000" || *d2.PointsEarned != 300 || *d2.PointsSpent != 100 {
			t.Fatalf("day 2 (midnight WIB belongs to the new day): %+v", d2)
		}
		if d3.ClaimedDeposits != 0 || d3.TotalWeightKG != "0.000" || *d3.PointsEarned != 0 {
			t.Fatalf("day 3 not zero filled: %+v", d3)
		}
		// Per-deposit totals despite the two-item deposit; categories keep
		// their snapshot names and count distinct deposits.
		if len(r.Categories) != 2 {
			t.Fatalf("categories: %+v", r.Categories)
		}
		p, k := r.Categories[0], r.Categories[1]
		if p.CategoryID != plastic || p.CategoryName != "Plastik" || p.DepositCount != 2 || p.TotalWeightG != 3000 || p.TotalWeightKG != "3.000" ||
			k.CategoryID != paper || k.CategoryName != "Kertas" || k.DepositCount != 1 || k.TotalWeightG != 500 || k.TotalWeightKG != "0.500" {
			t.Fatalf("categories: %+v", r.Categories)
		}
		w, err := service.NewWalletService(postgres.NewWalletRepository(pool)).Wallet(ctx, "plain-user")
		if err != nil || w.Balance != 390 {
			t.Fatalf("wallet %+v %v", w, err)
		}
	})
	t.Run("end_exclusive_and_start_inclusive_to_the_microsecond", func(t *testing.T) {
		r := get(t, "plain-user", service.ActivityReportQuery{From: wibT("2026-11-01T00:00:00+07:00"), To: wibT("2026-11-02T00:00:00+07:00")})
		if r.Totals.ClaimedDeposits != 1 || *r.Totals.PointsEarned != 190 || *r.Totals.PointsSpent != 0 || r.Period.Days != 1 {
			t.Fatalf("end exclusive: %+v", r.Totals)
		}
		r = get(t, "plain-user", service.ActivityReportQuery{From: wibT("2026-11-02T00:00:00+07:00"), To: wibT("2026-11-03T00:00:00+07:00")})
		if r.Totals.ClaimedDeposits != 1 || *r.Totals.PointsEarned != 300 {
			t.Fatalf("start inclusive: %+v", r.Totals)
		}
		// Period income is not the wallet: this window has no income at all.
		r = get(t, "plain-user", service.ActivityReportQuery{From: wibT("2026-11-03T00:00:00+07:00"), To: wibT("2026-11-10T00:00:00+07:00")})
		if r.Totals.ClaimedDeposits != 0 || *r.Totals.PointsEarned != 0 || len(r.Categories) != 0 || r.Categories == nil {
			t.Fatalf("empty window: %+v", r)
		}
		// Same instants expressed in UTC give the same result.
		r = get(t, "plain-user", service.ActivityReportQuery{From: wibT("2026-10-31T17:00:00Z"), To: wibT("2026-11-03T17:00:00Z")})
		if r.Totals.ClaimedDeposits != 2 {
			t.Fatalf("utc offsets: %+v", r.Totals)
		}
	})
	t.Run("default_range_is_current_wib_month_to_now", func(t *testing.T) {
		r := get(t, "user-2", service.ActivityReportQuery{})
		if r.Totals.ClaimedDeposits != 1 || *r.Totals.PointsEarned != 150 || r.Daily[4].Date != "2026-11-05" || r.Daily[4].ClaimedDeposits != 1 || r.Period.Days != 21 {
			t.Fatalf("default: %+v", r)
		}
	})
	t.Run("users_are_isolated", func(t *testing.T) {
		if r := get(t, "user-2", q); r.Totals.ClaimedDeposits != 0 || *r.Totals.PointsEarned != 0 {
			t.Fatalf("user-2 sees plain-user: %+v", r.Totals)
		}
		if r := get(t, "user-3", q); r.Totals.ClaimedDeposits != 0 || len(r.Categories) != 0 {
			t.Fatalf("user-3: %+v", r.Totals)
		}
	})
	t.Run("mitra_reports_only_own_claimed_deposits", func(t *testing.T) {
		wide := service.ActivityReportQuery{From: wibT("2026-11-01T00:00:00+07:00"), To: wibT("2026-11-08T00:00:00+07:00")}
		r := get(t, "owner-a", wide)
		if r.Account.Role != "mitra" || r.Account.BusinessName == nil || *r.Account.BusinessName != "Tunas" ||
			r.Totals.ClaimedDeposits != 3 || r.Totals.TotalWeightGram != 4500 || *r.Totals.PointsCredited != 640 || r.Totals.PointsEarned != nil || r.Totals.PointsSpent != nil {
			t.Fatalf("owner-a: %+v %+v", r.Account, r.Totals)
		}
		if c, g := sums(r); c != 3 || g != 4500 {
			t.Fatalf("daily: %d %d", c, g)
		}
		if r.Daily[4].Date != "2026-11-05" || r.Daily[4].ClaimedDeposits != 1 || *r.Daily[4].PointsCredited != 150 {
			t.Fatalf("day 5: %+v", r.Daily[4])
		}
		if len(r.Categories) != 2 || r.Categories[0].CategoryName != "Plastik" || r.Categories[0].DepositCount != 3 || r.Categories[0].TotalWeightG != 4000 || r.Categories[1].DepositCount != 1 {
			t.Fatalf("categories: %+v", r.Categories)
		}
		for _, uid := range []string{"owner-b", "owner-pending"} {
			if r := get(t, uid, wide); r.Totals.ClaimedDeposits != 0 || *r.Totals.PointsCredited != 0 || len(r.Categories) != 0 {
				t.Fatalf("%s sees owner-a: %+v", uid, r.Totals)
			}
		}
		// D-21: a suspended location can still read its report.
		if _, err := pool.Exec(ctx, `UPDATE public.partners SET status='suspended' WHERE mitra_uid='owner-a'`); err != nil {
			t.Fatal(err)
		}
		if r := get(t, "owner-a", wide); r.Totals.ClaimedDeposits != 3 {
			t.Fatalf("suspended mitra: %+v", r.Totals)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.partners SET status='verified' WHERE mitra_uid='owner-a'`); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("source_data_is_not_modified", func(t *testing.T) {
		var claims, ledger int
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.deposit_claims),(SELECT count(*) FROM public.point_ledger)`).Scan(&claims, &ledger); err != nil {
			t.Fatal(err)
		}
		get(t, "plain-user", q)
		var claims2, ledger2 int
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.deposit_claims),(SELECT count(*) FROM public.point_ledger)`).Scan(&claims2, &ledger2); err != nil || claims != claims2 || ledger != ledger2 {
			t.Fatalf("source rows changed: %v", err)
		}
	})
	t.Run("account_errors", func(t *testing.T) {
		if _, err := reports.Report(ctx, "missing", q); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("missing: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid IN ('user-3','owner-b')`); err != nil {
			t.Fatal(err)
		}
		for _, uid := range []string{"user-3", "owner-b"} {
			if _, err := reports.Report(ctx, uid, q); !errors.Is(err, service.ErrAccountForbidden) {
				t.Fatalf("%s suspended: %v", uid, err)
			}
		}
	})
	t.Run("pdf_from_real_data", func(t *testing.T) {
		doc, err := reports.PDF(ctx, "plain-user", q)
		if err != nil || len(doc.Content) < 1000 || string(doc.Content[:5]) != "%PDF-" || doc.Filename != "ecosnap-activity-report-20261101-20261103.pdf" {
			t.Fatalf("pdf: %v %q %d", err, doc.Filename, len(doc.Content))
		}
	})
}
