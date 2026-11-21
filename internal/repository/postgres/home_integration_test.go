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

func TestHistoryAndHomeIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	depositRepo := postgres.NewDepositRepository(pool)
	deposits := service.NewDepositService(depositRepo)
	qr := service.NewDepositQRService(depositRepo)
	wallets := service.NewWalletService(postgres.NewWalletRepository(pool))
	rewards := service.NewRewardService(postgres.NewRewardRepository(pool))
	n := 0
	// deposit makes a 1 kg plastic deposit at owner-a (150 points); stage
	// "created", "qr_issued", or "claimed" by uid.
	deposit := func(t *testing.T, stage, uid string) entity.Deposit {
		t.Helper()
		n++
		d, _, err := deposits.Create(ctx, "owner-a", fmt.Sprintf("home-key-%04d", n), entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "1"}}})
		if err != nil || stage == "created" {
			if err != nil {
				t.Fatal(err)
			}
			return d
		}
		code, err := qr.Issue(ctx, "owner-a", d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if stage == "claimed" {
			if _, _, err = qr.Claim(ctx, uid, code.Token); err != nil {
				t.Fatal(err)
			}
		}
		return d
	}
	first := deposit(t, "claimed", "plain-user")
	deposit(t, "claimed", "plain-user")
	unclaimed := deposit(t, "qr_issued", "")
	deposit(t, "created", "")
	if _, err := pool.Exec(ctx, `INSERT INTO public.rewards(id,name,cost_points,stock,is_demo) VALUES('80000000-0000-4000-8000-0000000000b1','Pulsa',100,5,true)`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := rewards.Redeem(ctx, "plain-user", "80000000-0000-4000-8000-0000000000b1", "home-redeem-1"); err != nil {
		t.Fatal(err)
	}

	t.Run("history_filters_by_type_and_time", func(t *testing.T) {
		count := func(q service.HistoryQuery) int {
			t.Helper()
			q.Limit = 100
			page, err := wallets.PointHistory(ctx, "plain-user", q)
			if err != nil {
				t.Fatal(err)
			}
			return len(page.Items)
		}
		if count(service.HistoryQuery{Type: "deposit"}) != 2 || count(service.HistoryQuery{Type: "redemption"}) != 1 || count(service.HistoryQuery{}) != 3 {
			t.Fatal("type filter")
		}
		// Move the first claim into January; a range from June excludes it.
		if _, err := pool.Exec(ctx, `UPDATE public.point_ledger SET created_at='2026-01-15T00:00:00Z' WHERE deposit_id=$1::uuid`, first.ID); err != nil {
			t.Fatal(err)
		}
		jun, feb := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		jan := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
		if count(service.HistoryQuery{From: &jun}) != 2 || count(service.HistoryQuery{To: &feb}) != 1 || count(service.HistoryQuery{From: &jan, To: &feb, Type: "deposit"}) != 1 {
			t.Fatal("time filter (from inclusive, to exclusive)")
		}
		page, err := wallets.PartnerDeposits(ctx, "owner-a", service.HistoryQuery{Limit: 100, From: &jun})
		if err != nil || len(page.Items) != 4 {
			t.Fatalf("partner from: %d %v", len(page.Items), err)
		}
	})
	t.Run("claimer_sees_deposit_detail_only", func(t *testing.T) {
		d, err := wallets.ClaimedDeposit(ctx, "plain-user", first.ID)
		if err != nil || d.Status != "claimed" || len(d.Items) != 1 || d.TotalPoints != 150 || d.Items[0].CategoryName != "Plastik" {
			t.Fatalf("detail: %+v %v", d, err)
		}
		for uid, id := range map[string]string{"user-2": first.ID, "plain-user": unclaimed.ID, "owner-a": first.ID} {
			if _, err := wallets.ClaimedDeposit(ctx, uid, id); !errors.Is(err, service.ErrDepositNotFound) {
				t.Fatalf("%s/%s: %v", uid, id, err)
			}
		}
	})
	t.Run("user_home_totals_match_sources", func(t *testing.T) {
		h, err := wallets.UserHome(ctx, "plain-user")
		if err != nil || h.Wallet.Balance != 200 || h.Wallet.TotalSpent != 100 || h.DepositsClaimed != 2 || h.TotalWeightKG != "2.000" || h.ActiveVouchers != 1 || len(h.RecentActivity) != 3 || h.RecentActivity[0].Source != "redemption" {
			t.Fatalf("home: %+v %v", h, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.redemptions SET created_at=clock_timestamp()-interval '8 days',expires_at=clock_timestamp()-interval '1 day' WHERE user_uid='plain-user'`); err != nil {
			t.Fatal(err)
		}
		if h, err = wallets.UserHome(ctx, "plain-user"); err != nil || h.ActiveVouchers != 0 {
			t.Fatalf("expired voucher counted: %+v %v", h, err)
		}
		if h, err = wallets.UserHome(ctx, "user-3"); err != nil || h.DepositsClaimed != 0 || h.TotalWeightKG != "0.000" || h.RecentActivity == nil || len(h.RecentActivity) != 0 {
			t.Fatalf("empty home: %+v %v", h, err)
		}
	})
	t.Run("partner_home_counts_by_status", func(t *testing.T) {
		h, err := wallets.PartnerHome(ctx, "owner-a")
		if err != nil || h.PartnerStatus != "verified" || h.Deposits != (entity.PartnerDepositCounts{Created: 1, QRIssued: 1, Claimed: 2}) || h.ClaimedWeightKG != "2.000" || h.ClaimedPoints != 300 || len(h.RecentDeposits) != 3 {
			t.Fatalf("partner home: %+v %v", h, err)
		}
		h, err = wallets.PartnerHome(ctx, "owner-pending")
		if err != nil || h.PartnerStatus != "pending" || h.Deposits != (entity.PartnerDepositCounts{}) || h.ClaimedWeightKG != "0.000" {
			t.Fatalf("pending home: %+v %v", h, err)
		}
		if _, err := wallets.PartnerHome(ctx, "plain-user"); !errors.Is(err, service.ErrDepositForbidden) {
			t.Fatalf("user partner home: %v", err)
		}
	})
}
