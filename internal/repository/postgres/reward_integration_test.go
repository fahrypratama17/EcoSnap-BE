package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

const (
	rewardCheap   = "80000000-0000-4000-8000-0000000000a1" // 100 pts, stock 50
	rewardScarce  = "80000000-0000-4000-8000-0000000000a2" // 100 pts, stock 1
	rewardEmpty   = "80000000-0000-4000-8000-0000000000a3" // stock 0
	rewardHidden  = "80000000-0000-4000-8000-0000000000a4" // inactive
	rewardPricey  = "80000000-0000-4000-8000-0000000000a5" // 100000 pts
	rewardLimited = "80000000-0000-4000-8000-0000000000a6" // 100 pts, stock 3
)

func TestRewardRedemptionIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	depositRepo := postgres.NewDepositRepository(pool)
	deposits := service.NewDepositService(depositRepo)
	qr := service.NewDepositQRService(depositRepo)
	rewards := service.NewRewardService(postgres.NewRewardRepository(pool))
	wallets := service.NewWalletService(postgres.NewWalletRepository(pool))
	n := 0
	// credit gives uid 150 points per call through the real claim path.
	credit := func(t *testing.T, uid string, times int) {
		t.Helper()
		for range times {
			n++
			d, _, err := deposits.Create(ctx, "owner-a", fmt.Sprintf("reward-key-%04d", n), entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "1"}}})
			if err != nil {
				t.Fatal(err)
			}
			code, err := qr.Issue(ctx, "owner-a", d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err = qr.Claim(ctx, uid, code.Token); err != nil {
				t.Fatal(err)
			}
		}
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.rewards(id,name,cost_points,stock,is_active,is_demo) VALUES
 ('`+rewardCheap+`','Pulsa',100,50,true,true),
 ('`+rewardScarce+`','Langka',100,1,true,true),
 ('`+rewardEmpty+`','Habis',100,0,true,true),
 ('`+rewardHidden+`','Arsip',100,9,false,true),
 ('`+rewardPricey+`','Mahal',100000,5,true,true),
 ('`+rewardLimited+`','Terbatas',100,3,true,true)`)
	if err != nil {
		t.Fatal(err)
	}
	credit(t, "plain-user", 2) // 300
	wallet := func(t *testing.T, uid string) entity.Wallet {
		t.Helper()
		w, err := wallets.Wallet(ctx, uid)
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	stock := func(t *testing.T, id string) int {
		t.Helper()
		var s int
		if err := pool.QueryRow(ctx, `SELECT stock FROM public.rewards WHERE id=$1::uuid`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	t.Run("catalog_shows_active_rewards_with_availability", func(t *testing.T) {
		items, err := rewards.Rewards(ctx)
		if err != nil || len(items) != 5 {
			t.Fatalf("catalog: %d %v", len(items), err)
		}
		for _, r := range items {
			if r.ID == rewardHidden || r.Available != (r.Stock > 0) {
				t.Fatalf("item: %+v", r)
			}
		}
		if _, err := rewards.Reward(ctx, rewardHidden); !errors.Is(err, service.ErrRewardNotFound) {
			t.Fatalf("inactive detail: %v", err)
		}
	})
	var first entity.Redemption
	t.Run("redeem_debits_wallet_stock_and_ledger_together", func(t *testing.T) {
		var created bool
		first, created, err = rewards.Redeem(ctx, "plain-user", rewardCheap, "redeem-0001")
		v := first.Voucher
		if err != nil || !created || first.WalletBalance != 200 || v.Status != "issued" || v.ExpiresAt.Sub(v.CreatedAt) != service.VoucherTTL || !regexp.MustCompile(`^ECO-[0-9A-HJKMNP-TV-Z]{4}-[0-9A-HJKMNP-TV-Z]{4}$`).MatchString(v.Code) {
			t.Fatalf("redeem: %+v %v %v", first, created, err)
		}
		w := wallet(t, "plain-user")
		if w.Balance != 200 || w.TotalSpent != 100 || w.TotalEarned != 300 || stock(t, rewardCheap) != 49 {
			t.Fatalf("after redeem: %+v stock=%d", w, stock(t, rewardCheap))
		}
		page, err := wallets.PointHistory(ctx, "plain-user", service.HistoryQuery{Limit: 1})
		if err != nil || page.Items[0].Amount != -100 || page.Items[0].Source != "redemption" || page.Items[0].Redemption == nil || page.Items[0].Redemption.RewardName != "Pulsa" {
			t.Fatalf("history: %+v %v", page.Items, err)
		}
	})
	t.Run("retry_replays_and_reused_key_conflicts", func(t *testing.T) {
		again, created, err := rewards.Redeem(ctx, "plain-user", rewardCheap, "redeem-0001")
		if err != nil || created || again.Voucher.ID != first.Voucher.ID || again.WalletBalance != 200 || stock(t, rewardCheap) != 49 {
			t.Fatalf("replay: %+v %v %v", again, created, err)
		}
		if _, _, err := rewards.Redeem(ctx, "plain-user", rewardScarce, "redeem-0001"); !errors.Is(err, service.ErrRedeemConflict) {
			t.Fatalf("conflict: %v", err)
		}
	})
	t.Run("failures_take_no_points_or_stock", func(t *testing.T) {
		before := wallet(t, "plain-user")
		for _, tt := range []struct {
			uid, reward string
			want        error
		}{
			{"plain-user", rewardPricey, service.ErrInsufficientPoints},
			{"plain-user", rewardEmpty, service.ErrRewardOutOfStock},
			{"plain-user", rewardHidden, service.ErrRewardNotFound},
			{"owner-a", rewardCheap, service.ErrRedeemForbidden},
		} {
			if _, _, err := rewards.Redeem(ctx, tt.uid, tt.reward, "redeem-fail-1"); !errors.Is(err, tt.want) {
				t.Fatalf("%s/%s: %v", tt.uid, tt.reward, err)
			}
		}
		if wallet(t, "plain-user") != before || stock(t, rewardPricey) != 5 || stock(t, rewardEmpty) != 0 {
			t.Fatal("failed redemption changed state")
		}
	})
	t.Run("parallel_redemptions_never_overdraw", func(t *testing.T) {
		credit(t, "user-2", 1) // 150: enough for one 100-point reward
		credit(t, "user-3", 2) // 300
		var wg sync.WaitGroup
		var mu sync.Mutex
		ok := map[string]int{}
		for i := range 8 {
			uid := []string{"user-2", "user-3"}[i%2]
			reward := []string{rewardLimited, rewardScarce}[i/4]
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err := rewards.Redeem(ctx, uid, reward, fmt.Sprintf("parallel-%04d", i))
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					ok[uid]++
				case errors.Is(err, service.ErrInsufficientPoints), errors.Is(err, service.ErrRewardOutOfStock):
				default:
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		w2, w3 := wallet(t, "user-2"), wallet(t, "user-3")
		if ok["user-2"] != 1 || ok["user-3"] > 3 || w2.Balance != 50 || w3.Balance != 300-int64(ok["user-3"])*100 || w3.Balance < 0 {
			t.Fatalf("ok=%v w2=%+v w3=%+v", ok, w2, w3)
		}
		if used := (3 - stock(t, rewardLimited)) + (1 - stock(t, rewardScarce)); used != ok["user-2"]+ok["user-3"] || stock(t, rewardScarce) < 0 || stock(t, rewardLimited) < 0 {
			t.Fatalf("stock mismatch: used=%d ok=%v", used, ok)
		}
	})
	t.Run("vouchers_are_private_and_used_once", func(t *testing.T) {
		if _, err := rewards.Voucher(ctx, "user-2", first.Voucher.ID); !errors.Is(err, service.ErrVoucherNotFound) {
			t.Fatalf("other user read voucher: %v", err)
		}
		if _, err := rewards.UseVoucher(ctx, "user-2", first.Voucher.ID); !errors.Is(err, service.ErrVoucherNotFound) {
			t.Fatalf("other user used voucher: %v", err)
		}
		used, err := rewards.UseVoucher(ctx, "plain-user", first.Voucher.ID)
		if err != nil || used.Status != "used" || used.UsedAt == nil {
			t.Fatalf("use: %+v %v", used, err)
		}
		again, err := rewards.UseVoucher(ctx, "plain-user", first.Voucher.ID)
		if err != nil || !again.UsedAt.Equal(*used.UsedAt) {
			t.Fatalf("repeat use: %+v %v", again, err)
		}
	})
	t.Run("expired_vouchers_cannot_be_used_and_filter", func(t *testing.T) {
		r, _, err := rewards.Redeem(ctx, "plain-user", rewardCheap, "redeem-0002")
		if err != nil {
			t.Fatal(err)
		}
		// D-20: at expires_at the voucher is already expired; no refund.
		if _, err = pool.Exec(ctx, `UPDATE public.redemptions SET created_at=clock_timestamp()-interval '7 days',expires_at=clock_timestamp() WHERE id=$1::uuid`, r.Voucher.ID); err != nil {
			t.Fatal(err)
		}
		before := wallet(t, "plain-user")
		if _, err := rewards.UseVoucher(ctx, "plain-user", r.Voucher.ID); !errors.Is(err, service.ErrVoucherExpired) {
			t.Fatalf("expired use: %v", err)
		}
		if wallet(t, "plain-user") != before {
			t.Fatal("expiry refunded points")
		}
		count := func(status string) int {
			page, err := rewards.Vouchers(ctx, "plain-user", service.HistoryQuery{Limit: 100, Status: status})
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range page.Items {
				if v.Status != status {
					t.Fatalf("filter %s returned %s", status, v.Status)
				}
			}
			return len(page.Items)
		}
		if count("used") != 1 || count("expired") != 1 || count("issued") != 0 {
			t.Fatal("status filter counts")
		}
		if page, err := rewards.Vouchers(ctx, "user-2", service.HistoryQuery{Limit: 100}); err != nil || len(page.Items) != 1 {
			t.Fatalf("user-2 vouchers: %d %v", len(page.Items), err)
		}
	})
	t.Run("wallets_equal_ledger_after_debits", func(t *testing.T) {
		var mismatched int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.wallets w WHERE w.balance <> (SELECT COALESCE(sum(amount),0) FROM public.point_ledger l WHERE l.user_uid=w.user_uid) OR w.total_spent <> -(SELECT COALESCE(sum(amount),0) FROM public.point_ledger l WHERE l.user_uid=w.user_uid AND amount<0)`).Scan(&mismatched); err != nil || mismatched != 0 {
			t.Fatalf("mismatched: %d %v", mismatched, err)
		}
	})
}

func TestRewardSeedCommandIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..", "..")
	cfg := pool.Config().ConnConfig
	dbURL := url.URL{Scheme: "postgresql", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, RawQuery: "sslmode=disable", User: url.UserPassword(cfg.User, cfg.Password)}
	run := func() {
		cmd := exec.CommandContext(ctx, "go", "run", "-p=1", "-mod=readonly", "./cmd/seed-rewards", "--demo")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dbURL.String())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("seed: %v %s", err, output)
		}
	}
	run()
	// Stock consumed by redemptions must survive a re-run.
	if _, err := pool.Exec(ctx, `UPDATE public.rewards SET stock=stock-1 WHERE id='80000000-0000-4000-8000-000000000001'`); err != nil {
		t.Fatal(err)
	}
	run()
	var count, stock int
	if err := pool.QueryRow(ctx, `SELECT count(*),(SELECT stock FROM public.rewards WHERE id='80000000-0000-4000-8000-000000000001') FROM public.rewards WHERE is_demo`).Scan(&count, &stock); err != nil || count != 3 || stock != 49 {
		t.Fatalf("seed rows=%d stock=%d %v", count, stock, err)
	}
}
