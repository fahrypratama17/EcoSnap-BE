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

func TestWalletHistoryIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	depositRepo := postgres.NewDepositRepository(pool)
	deposits := service.NewDepositService(depositRepo)
	qr := service.NewDepositQRService(depositRepo)
	wallets := service.NewWalletService(postgres.NewWalletRepository(pool))
	n := 0
	// deposit creates one 1 kg plastic deposit (150 points at owner-a) and
	// optionally issues a QR and lets uid claim it.
	deposit := func(t *testing.T, uid string, issue bool) entity.Deposit {
		t.Helper()
		n++
		d, _, err := deposits.Create(ctx, "owner-a", fmt.Sprintf("wallet-key-%04d", n), entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "1"}}})
		if err != nil {
			t.Fatal(err)
		}
		if issue {
			code, err := qr.Issue(ctx, "owner-a", d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if uid != "" {
				if _, _, err = qr.Claim(ctx, uid, code.Token); err != nil {
					t.Fatal(err)
				}
			}
		}
		return d
	}
	for range 5 {
		deposit(t, "plain-user", true)
	}
	deposit(t, "user-2", true)
	deposit(t, "", true)  // qr_issued, unclaimed
	deposit(t, "", false) // created

	allPages := func(t *testing.T, limit int, between func()) []entity.PointHistoryEntry {
		t.Helper()
		var out []entity.PointHistoryEntry
		cursor := ""
		for {
			page, err := wallets.PointHistory(ctx, "plain-user", service.HistoryQuery{Limit: limit, Cursor: cursor})
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, page.Items...)
			if page.NextCursor == "" {
				return out
			}
			cursor = page.NextCursor
			if between != nil {
				between()
				between = nil
			}
		}
	}

	t.Run("wallet_matches_own_ledger", func(t *testing.T) {
		w, err := wallets.Wallet(ctx, "plain-user")
		if err != nil || w.Balance != 750 || w.TotalEarned != 750 || w.TotalSpent != 0 {
			t.Fatalf("wallet: %+v %v", w, err)
		}
		entries := allPages(t, 100, nil)
		var sum int64
		for _, e := range entries {
			sum += e.Amount
			if e.Source != "deposit" || e.Deposit == nil || e.Deposit.PartnerName != "Tunas" || e.Deposit.TotalWeightKG != "1.000" || e.Amount != 150 {
				t.Fatalf("entry: %+v", e)
			}
		}
		if len(entries) != 5 || sum != w.Balance {
			t.Fatalf("entries=%d sum=%d", len(entries), sum)
		}
		for i := 1; i < len(entries); i++ {
			if entries[i].CreatedAt.After(entries[i-1].CreatedAt) {
				t.Fatal("history not newest first")
			}
		}
	})
	t.Run("pages_are_stable_with_ties_and_new_entries", func(t *testing.T) {
		// Identical timestamps force the id tie-break.
		if _, err := pool.Exec(ctx, `UPDATE public.point_ledger SET created_at='2026-01-01T00:00:00Z' WHERE user_uid='plain-user'`); err != nil {
			t.Fatal(err)
		}
		entries := allPages(t, 2, func() { deposit(t, "plain-user", true) })
		seen := map[string]bool{}
		for _, e := range entries {
			if seen[e.ID] {
				t.Fatalf("duplicate %s", e.ID)
			}
			seen[e.ID] = true
		}
		// The new claim is newer than the cursor, so it is not in this pass;
		// all five older entries are, exactly once.
		if len(entries) != 5 {
			t.Fatalf("got %d entries", len(entries))
		}
		if got := allPages(t, 2, nil); len(got) != 6 {
			t.Fatalf("fresh pass: %d", len(got))
		}
	})
	t.Run("history_is_per_user", func(t *testing.T) {
		page, err := wallets.PointHistory(ctx, "user-2", service.HistoryQuery{Limit: 100})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("user-2: %d %v", len(page.Items), err)
		}
		page, err = wallets.PointHistory(ctx, "user-3", service.HistoryQuery{Limit: 100})
		if err != nil || len(page.Items) != 0 || page.Items == nil {
			t.Fatalf("user-3: %+v %v", page.Items, err)
		}
	})
	t.Run("partner_deposits_filter_and_ownership", func(t *testing.T) {
		count := map[string]int{}
		cursor := ""
		for {
			page, err := wallets.PartnerDeposits(ctx, "owner-a", service.HistoryQuery{Limit: 3, Cursor: cursor})
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range page.Items {
				count[d.Status]++
				if (d.Status == "claimed") != (d.ClaimedAt != nil) {
					t.Fatalf("claimed_at: %+v", d)
				}
			}
			if cursor = page.NextCursor; cursor == "" {
				break
			}
		}
		if count["claimed"] != 7 || count["qr_issued"] != 1 || count["created"] != 1 {
			t.Fatalf("counts: %v", count)
		}
		page, err := wallets.PartnerDeposits(ctx, "owner-a", service.HistoryQuery{Limit: 100, Status: "qr_issued"})
		if err != nil || len(page.Items) != 1 || page.Items[0].Status != "qr_issued" {
			t.Fatalf("filter: %+v %v", page.Items, err)
		}
		if page, err = wallets.PartnerDeposits(ctx, "owner-b", service.HistoryQuery{Limit: 100}); err != nil || len(page.Items) != 0 {
			t.Fatalf("owner-b sees: %d %v", len(page.Items), err)
		}
		for uid, want := range map[string]error{"plain-user": service.ErrDepositForbidden, "owner-pending": service.ErrDepositForbidden, "missing": service.ErrAccountNotFound} {
			if _, err := wallets.PartnerDeposits(ctx, uid, service.HistoryQuery{Limit: 10}); !errors.Is(err, want) {
				t.Fatalf("%s: %v", uid, err)
			}
		}
	})
	t.Run("invalid_queries_and_inactive_accounts", func(t *testing.T) {
		for _, q := range []service.HistoryQuery{{Limit: 0}, {Limit: 101}, {Limit: 10, Cursor: "!!"}, {Limit: 10, Cursor: "Zm9v"}, {Limit: 10, Status: "claimed"}} {
			if _, err := wallets.PointHistory(ctx, "plain-user", q); !errors.Is(err, service.ErrInvalidHistoryQuery) {
				t.Fatalf("%+v: %v", q, err)
			}
		}
		if _, err := wallets.PartnerDeposits(ctx, "owner-a", service.HistoryQuery{Limit: 10, Status: "cancelled"}); !errors.Is(err, service.ErrInvalidHistoryQuery) {
			t.Fatalf("status: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='user-2'`); err != nil {
			t.Fatal(err)
		}
		if _, err := wallets.Wallet(ctx, "user-2"); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("suspended wallet: %v", err)
		}
		if _, err := wallets.PointHistory(ctx, "user-2", service.HistoryQuery{Limit: 10}); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("suspended history: %v", err)
		}
	})
}
