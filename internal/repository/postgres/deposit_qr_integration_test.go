package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

func TestDepositQRClaimIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	repo := postgres.NewDepositRepository(pool)
	deposits := service.NewDepositService(repo)
	qr := service.NewDepositQRService(repo)
	request := entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "2.5"}, {CategoryID: paper, WeightKG: "1"}}}
	keyN := 0
	newDeposit := func(t *testing.T) entity.Deposit {
		t.Helper()
		keyN++
		d, _, err := deposits.Create(ctx, "owner-a", fmt.Sprintf("qr-key-%04d", keyN), request)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	type money struct{ balance, earned, ledger int64 }
	wallet := func(t *testing.T, uid string) money {
		t.Helper()
		var m money
		err := pool.QueryRow(ctx, `SELECT w.balance,w.total_earned,(SELECT COALESCE(sum(amount),0) FROM public.point_ledger l WHERE l.user_uid=w.user_uid) FROM public.wallets w WHERE w.user_uid=$1`, uid).Scan(&m.balance, &m.earned, &m.ledger)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	status := func(t *testing.T, id string) string {
		t.Helper()
		var s string
		if err := pool.QueryRow(ctx, `SELECT status::text FROM public.deposits WHERE id=$1::uuid`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	t.Run("issue_locks_deposit_and_stores_only_hash", func(t *testing.T) {
		d := newDeposit(t)
		got, err := qr.Issue(ctx, "owner-a", d.ID)
		if err != nil || len(got.Token) != 43 || got.ExpiresAt.Sub(got.IssuedAt) != service.QRTTL || got.Warning == "" || status(t, d.ID) != "qr_issued" {
			t.Fatalf("issue: %+v %v", got, err)
		}
		var plain int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.deposit_qr_tokens WHERE token_hash=$1`, got.Token).Scan(&plain); err != nil || plain != 0 {
			t.Fatalf("raw token stored: %d %v", plain, err)
		}
		for uid, want := range map[string]error{"owner-b": service.ErrDepositNotFound, "plain-user": service.ErrDepositForbidden, "owner-pending": service.ErrDepositForbidden} {
			if _, err := qr.Issue(ctx, uid, d.ID); !errors.Is(err, want) {
				t.Fatalf("%s issued: %v", uid, err)
			}
		}
	})
	t.Run("preview_does_not_mutate", func(t *testing.T) {
		d := newDeposit(t)
		code, _ := qr.Issue(ctx, "owner-a", d.ID)
		before := wallet(t, "plain-user")
		p, err := qr.Preview(ctx, "plain-user", code.Token)
		if err != nil || p.TotalPoints != 455 || p.PartnerName != "Tunas" || len(p.Items) != 2 || !p.ExpiresAt.Equal(code.ExpiresAt) || !p.ServerTime.After(code.IssuedAt) || !p.ServerTime.Before(p.ExpiresAt) {
			t.Fatalf("preview: %+v %v", p, err)
		}
		if wallet(t, "plain-user") != before || status(t, d.ID) != "qr_issued" {
			t.Fatal("preview mutated state")
		}
		if _, err := qr.Preview(ctx, "owner-b", code.Token); !errors.Is(err, service.ErrClaimForbidden) {
			t.Fatalf("mitra preview: %v", err)
		}
	})
	t.Run("reissue_revokes_old_token", func(t *testing.T) {
		d := newDeposit(t)
		old, _ := qr.Issue(ctx, "owner-a", d.ID)
		fresh, err := qr.Issue(ctx, "owner-a", d.ID)
		if err != nil || !fresh.IssuedAt.After(old.IssuedAt) {
			t.Fatalf("reissue: %v", err)
		}
		if _, _, err := qr.Claim(ctx, "plain-user", old.Token); !errors.Is(err, service.ErrQRRevoked) {
			t.Fatalf("old token: %v", err)
		}
		var history int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.deposit_qr_tokens WHERE deposit_id=$1::uuid`, d.ID).Scan(&history); err != nil || history != 2 {
			t.Fatalf("history: %d %v", history, err)
		}
		if _, _, err := qr.Claim(ctx, "plain-user", fresh.Token); err != nil {
			t.Fatalf("new token: %v", err)
		}
	})
	t.Run("expiry_boundary_and_unknown_tokens", func(t *testing.T) {
		d := newDeposit(t)
		code, _ := qr.Issue(ctx, "owner-a", d.ID)
		// D-05: a token whose expires_at has been reached is rejected.
		if _, err := pool.Exec(ctx, `UPDATE public.deposit_qr_tokens SET issued_at=clock_timestamp()-interval '10 minutes',expires_at=clock_timestamp() WHERE deposit_id=$1::uuid`, d.ID); err != nil {
			t.Fatal(err)
		}
		before := wallet(t, "plain-user")
		if _, _, err := qr.Claim(ctx, "plain-user", code.Token); !errors.Is(err, service.ErrQRExpired) {
			t.Fatalf("expired: %v", err)
		}
		if _, err := qr.Preview(ctx, "plain-user", code.Token); !errors.Is(err, service.ErrQRExpired) {
			t.Fatalf("expired preview: %v", err)
		}
		if wallet(t, "plain-user") != before {
			t.Fatal("expired claim credited")
		}
		for _, token := range []string{"", "short", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
			if _, _, err := qr.Claim(ctx, "plain-user", token); !errors.Is(err, service.ErrQRInvalid) {
				t.Fatalf("token %q: %v", token, err)
			}
		}
		// Reissue after expiry gives a fresh ten minutes (D-06).
		again, err := qr.Issue(ctx, "owner-a", d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := qr.Claim(ctx, "plain-user", again.Token); err != nil {
			t.Fatalf("after reissue: %v", err)
		}
	})
	t.Run("mitra_cannot_claim", func(t *testing.T) {
		d := newDeposit(t)
		code, _ := qr.Issue(ctx, "owner-a", d.ID)
		for _, uid := range []string{"owner-a", "owner-b"} {
			if _, _, err := qr.Claim(ctx, uid, code.Token); !errors.Is(err, service.ErrClaimForbidden) {
				t.Fatalf("%s: %v", uid, err)
			}
		}
	})
	t.Run("first_of_parallel_claimers_wins_once", func(t *testing.T) {
		d := newDeposit(t)
		code, _ := qr.Issue(ctx, "owner-a", d.ID)
		b2, b3 := wallet(t, "user-2"), wallet(t, "user-3")
		var wg sync.WaitGroup
		var mu sync.Mutex
		winners, alreadyClaimed := map[string]int{}, 0
		for i := range 8 {
			uid := []string{"user-2", "user-3"}[i%2]
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, created, err := qr.Claim(ctx, uid, code.Token)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil && created:
					winners[uid]++
				case err == nil || errors.Is(err, service.ErrDepositAlreadyClaimed):
					alreadyClaimed++ // replay by the winner or loser rejection
				default:
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if len(winners) != 1 || alreadyClaimed != 7 || status(t, d.ID) != "claimed" {
			t.Fatalf("winners=%v other=%d", winners, alreadyClaimed)
		}
		a2, a3 := wallet(t, "user-2"), wallet(t, "user-3")
		credited := (a2.balance - b2.balance) + (a3.balance - b3.balance)
		if credited != 455 || a2.balance != a2.ledger || a3.balance != a3.ledger || a2.earned != a2.ledger || a3.earned != a3.ledger {
			t.Fatalf("credit=%d wallets %+v %+v", credited, a2, a3)
		}
		var winner string
		for uid := range winners {
			winner = uid
		}
		replay, created, err := qr.Claim(ctx, winner, code.Token)
		if err != nil || created || replay.Points != 455 || replay.DepositID != d.ID {
			t.Fatalf("replay: %+v %v %v", replay, created, err)
		}
		if _, err := qr.Issue(ctx, "owner-a", d.ID); !errors.Is(err, service.ErrDepositAlreadyClaimed) {
			t.Fatalf("reissue after claim: %v", err)
		}
	})
	t.Run("reissue_racing_claim_credits_at_most_once", func(t *testing.T) {
		for range 5 {
			d := newDeposit(t)
			code, _ := qr.Issue(ctx, "owner-a", d.ID)
			before := wallet(t, "user-3")
			var wg sync.WaitGroup
			var claimErr, issueErr error
			wg.Add(2)
			go func() { defer wg.Done(); _, _, claimErr = qr.Claim(ctx, "user-3", code.Token) }()
			go func() { defer wg.Done(); _, issueErr = qr.Issue(ctx, "owner-a", d.ID) }()
			wg.Wait()
			after := wallet(t, "user-3")
			var live int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.deposit_qr_tokens WHERE deposit_id=$1::uuid AND revoked_at IS NULL AND used_at IS NULL`, d.ID).Scan(&live); err != nil {
				t.Fatal(err)
			}
			switch {
			case claimErr == nil: // claim first: reissue refused, no live token
				if !errors.Is(issueErr, service.ErrDepositAlreadyClaimed) || after.balance-before.balance != 455 || live != 0 {
					t.Fatalf("claim first: %v live=%d %+v", issueErr, live, after)
				}
			case errors.Is(claimErr, service.ErrQRRevoked): // reissue first
				if issueErr != nil || after != before || live != 1 {
					t.Fatalf("reissue first: %v live=%d", issueErr, live)
				}
			default:
				t.Fatalf("claim: %v", claimErr)
			}
		}
	})
	t.Run("ledger_is_the_wallet_source", func(t *testing.T) {
		var mismatched int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.wallets w WHERE w.balance <> (SELECT COALESCE(sum(amount),0) FROM public.point_ledger l WHERE l.user_uid=w.user_uid)`).Scan(&mismatched); err != nil || mismatched != 0 {
			t.Fatalf("mismatched wallets: %d %v", mismatched, err)
		}
	})
}
