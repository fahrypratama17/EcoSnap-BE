package postgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWeekPeriodWIBBoundaries(t *testing.T) {
	cases := []struct {
		name  string
		now   time.Time
		start time.Time
	}{
		{"sunday_last_second", time.Date(2026, 11, 22, 23, 59, 59, 0, service.WIB), time.Date(2026, 11, 16, 0, 0, 0, 0, service.WIB)},
		{"monday_exact_start", time.Date(2026, 11, 23, 0, 0, 0, 0, service.WIB), time.Date(2026, 11, 23, 0, 0, 0, 0, service.WIB)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := service.WeekPeriod(tc.now)
			if !start.Equal(tc.start) || !end.Equal(tc.start.AddDate(0, 0, 7)) {
				t.Fatalf("period [%s,%s), want [%s,%s)", start, end, tc.start, tc.start.AddDate(0, 0, 7))
			}
		})
	}
}

// insertGamificationClaim inserts a structurally real claimed deposit. Explicit
// timestamps let these tests cover WIB week boundaries without changing clocks.
func insertGamificationClaim(ctx context.Context, t *testing.T, pool *pgxpool.Pool, n int, uid string, grams int64, claimedAt time.Time) {
	t.Helper()
	depositID := fmt.Sprintf("a0000000-0000-4000-8000-%012d", n)
	tokenID := fmt.Sprintf("b0000000-0000-4000-8000-%012d", n)
	if _, err := pool.Exec(ctx, `INSERT INTO public.deposits(id,partner_id,created_by_uid,idempotency_key,request_hash,total_weight_grams,total_points,uses_demo_rates,rates_as_of,status)
VALUES($1::uuid,'20000000-0000-4000-8000-000000000001','owner-a',$2,repeat('a',64),$3,0,true,$4,'claimed')`, depositID, fmt.Sprintf("gamify-%08d", n), grams, claimedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.deposit_qr_tokens(id,deposit_id,token_hash,issued_by_uid,issued_at,expires_at,used_at)
VALUES($1::uuid,$2::uuid,md5($1::text), 'owner-a',$3::timestamptz,$3::timestamptz+interval '1 day',$3::timestamptz)`, tokenID, depositID, claimedAt); err != nil {
		t.Fatal(err)
	}
	if grams > 1 {
		half := grams / 2
		if _, err := pool.Exec(ctx, `INSERT INTO public.deposit_items(deposit_id,line_no,category_id,category_name,weight_grams,rate_id,rate_hundredths,exact_points_e5,is_demo) VALUES
($1::uuid,1,'10000000-0000-4000-8000-000000000001','Plastik',$2::bigint,'50000000-0000-4000-8000-000000000003',15000,$2::bigint*15000,true),
($1::uuid,2,'10000000-0000-4000-8000-000000000002','Kertas',$3::bigint,'50000000-0000-4000-8000-000000000002',8000,$3::bigint*8000,true)`, depositID, half, grams-half); err != nil {
			t.Fatal(err)
		}
	} else if _, err := pool.Exec(ctx, `INSERT INTO public.deposit_items(deposit_id,line_no,category_id,category_name,weight_grams,rate_id,rate_hundredths,exact_points_e5,is_demo) VALUES($1::uuid,1,'10000000-0000-4000-8000-000000000001','Plastik',1,'50000000-0000-4000-8000-000000000003',15000,15000,true)`, depositID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.deposit_claims(deposit_id,qr_token_id,claimer_uid,points,claimed_at) VALUES($1::uuid,$2::uuid,$3,0,$4)`, depositID, tokenID, uid, claimedAt); err != nil {
		t.Fatal(err)
	}
}

func TestGamificationIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	gamification := service.NewGamificationService(postgres.NewGamificationRepository(pool))
	current, err := gamification.Missions(ctx, "plain-user")
	if err != nil {
		t.Fatal(err)
	}
	period := current.PeriodStart
	claimAt := current.ServerTime.Add(-time.Second)
	if claimAt.Before(period) {
		claimAt = period
	}

	t.Run("incomplete_claim_replay_and_wallet_separation", func(t *testing.T) {
		if _, _, err := gamification.ClaimMission(ctx, "plain-user", service.MissionFirstDeposit, period); err != service.ErrMissionIncomplete {
			t.Fatalf("incomplete: %v", err)
		}
		insertGamificationClaim(ctx, t, pool, 1, "plain-user", 1500, claimAt)
		before, err := service.NewWalletService(postgres.NewWalletRepository(pool)).Wallet(ctx, "plain-user")
		if err != nil {
			t.Fatal(err)
		}
		result, created, err := gamification.ClaimMission(ctx, "plain-user", service.MissionFirstDeposit, period)
		if err != nil || !created || result.XPAwarded != 50 || result.Mission.Status != "claimed" {
			t.Fatalf("claim: %+v %v %v", result, created, err)
		}
		replay, created, err := gamification.ClaimMission(ctx, "plain-user", service.MissionFirstDeposit, period)
		if err != nil || created || replay.WeeklyXP != 50 || replay.TotalXP != 50 {
			t.Fatalf("replay: %+v %v %v", replay, created, err)
		}
		after, err := service.NewWalletService(postgres.NewWalletRepository(pool)).Wallet(ctx, "plain-user")
		if err != nil || before != after {
			t.Fatalf("mission XP changed wallet: before=%+v after=%+v err=%v", before, after, err)
		}
	})

	t.Run("concurrent_claim_awards_once", func(t *testing.T) {
		insertGamificationClaim(ctx, t, pool, 2, "user-2", 1000, claimAt)
		const workers = 8
		var wg sync.WaitGroup
		var mu sync.Mutex
		createdCount, failures := 0, 0
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, created, err := gamification.ClaimMission(ctx, "user-2", service.MissionFirstDeposit, period)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failures++
				}
				if created {
					createdCount++
				}
			}()
		}
		wg.Wait()
		var rows int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.mission_claims WHERE user_uid='user-2' AND mission_id='first_deposit' AND period_start=$1`, period).Scan(&rows); err != nil {
			t.Fatal(err)
		}
		if failures != 0 || createdCount != 1 || rows != 1 {
			t.Fatalf("failures=%d created=%d rows=%d", failures, createdCount, rows)
		}
	})

	t.Run("weekly_window_and_weight_threshold", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `INSERT INTO public.deposits(id,partner_id,created_by_uid,idempotency_key,request_hash,total_weight_grams,total_points,uses_demo_rates,rates_as_of,status) VALUES('a0000000-0000-4000-8000-000000000006','20000000-0000-4000-8000-000000000001','owner-a','gamify-unclaimed-6',repeat('c',64),50000,0,true,$1,'created')`, claimAt); err != nil {
			t.Fatal(err)
		}
		insertGamificationClaim(ctx, t, pool, 3, "user-3", 2999, period.Add(-time.Second))
		insertGamificationClaim(ctx, t, pool, 4, "user-3", 2999, claimAt)
		view, err := gamification.Missions(ctx, "user-3")
		if err != nil {
			t.Fatal(err)
		}
		if view.Missions[0].Progress != 1 || view.Missions[1].Progress != 2999 || view.Missions[1].Status != "in_progress" {
			t.Fatalf("week progress: %+v", view.Missions)
		}
		if _, _, err = gamification.ClaimMission(ctx, "user-3", service.MissionThreeKilograms, period); err != service.ErrMissionIncomplete {
			t.Fatalf("2999g: %v", err)
		}
		insertGamificationClaim(ctx, t, pool, 5, "user-3", 1, claimAt)
		result, created, err := gamification.ClaimMission(ctx, "user-3", service.MissionThreeKilograms, period)
		if err != nil || !created || result.Mission.Progress != 3000 || result.XPAwarded != 100 {
			t.Fatalf("3000g: %+v created=%v err=%v", result, created, err)
		}
		if _, _, err = gamification.ClaimMission(ctx, "user-3", service.MissionFirstDeposit, period.Add(-7*24*time.Hour)); err != service.ErrMissionExpired {
			t.Fatalf("stale week accepted: %v", err)
		}
	})

	t.Run("alltime_badges_and_leaderboard_privacy", func(t *testing.T) {
		for i := 0; i < 9; i++ {
			insertGamificationClaim(ctx, t, pool, 20+i, "plain-user", 1000, claimAt.Add(-time.Duration(i)*time.Second))
		}
		insertGamificationClaim(ctx, t, pool, 29, "user-2", 1000, claimAt)
		for _, row := range []struct {
			uid, mission string
			xp           int
		}{{"plain-user", "three_kilograms", 100}, {"user-2", "three_kilograms", 100}, {"user-3", "first_deposit", 50}, {"owner-pending", "first_deposit", 50}} {
			if _, err := pool.Exec(ctx, `INSERT INTO public.mission_claims(user_uid,mission_id,period_start,period_end,progress,target,xp,claimed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, row.uid, row.mission, period, current.PeriodEnd, int64(3000), int64(1), row.xp, claimAt); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET display_name='Private Name',photo_path='private-photo' WHERE firebase_uid='plain-user'; UPDATE public.accounts SET status='suspended' WHERE firebase_uid='user-3'`); err != nil {
			t.Fatal(err)
		}
		badges, err := gamification.Badges(ctx, "plain-user")
		if err != nil {
			t.Fatal(err)
		}
		if badges.ClaimedDeposits != 10 || badges.ClaimedWeightGrams != 10500 || !badges.Badges[0].Earned || !badges.Badges[1].Earned || !badges.Badges[2].Earned {
			t.Fatalf("badges: %+v", badges)
		}
		board, err := gamification.Leaderboard(ctx, "plain-user", 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(board.Entries) != 2 || board.Entries[0].DisplayName != "Private Name" || board.Entries[0].PhotoPath == nil || *board.Entries[0].PhotoPath != "private-photo" || board.Entries[0].Rank != 1 || board.Me == nil || board.Me.Rank != 1 {
			t.Fatalf("opted-in board: %+v", board)
		}
		if _, err := gamification.SetLeaderboardVisibility(ctx, "user-2", false); err != nil {
			t.Fatal(err)
		}
		board, err = gamification.Leaderboard(ctx, "plain-user", 20)
		if err != nil || len(board.Entries) != 1 {
			t.Fatalf("opt-out: %+v %v", board, err)
		}
		if _, err = gamification.SetLeaderboardVisibility(ctx, "user-2", true); err != nil {
			t.Fatal(err)
		}
		board, err = gamification.Leaderboard(ctx, "plain-user", 20)
		if err != nil || len(board.Entries) != 2 || board.Entries[0].Rank != 1 || board.Entries[1].Rank != 1 {
			t.Fatalf("tie/include: %+v %v", board, err)
		}
		for _, e := range board.Entries {
			if e.DisplayName == "user-3" || e.DisplayName == "owner-pending" {
				t.Fatalf("ineligible entry: %+v", e)
			}
		}
	})
	t.Run("earned_history_cannot_be_rewritten", func(t *testing.T) {
		for _, statement := range []string{
			`UPDATE public.mission_claims SET xp=999 WHERE user_uid='plain-user'`,
			`DELETE FROM public.mission_claims WHERE user_uid='plain-user'`,
			`UPDATE public.deposit_claims SET claimer_uid='user-2' WHERE claimer_uid='plain-user'`,
			`DELETE FROM public.deposit_claims WHERE claimer_uid='plain-user'`,
		} {
			if _, err := pool.Exec(ctx, statement); err == nil {
				t.Fatalf("earned history was mutable: %s", statement)
			}
		}
		view, err := gamification.Missions(ctx, "plain-user")
		if err != nil || view.TotalXP != 150 {
			t.Fatalf("XP changed after rejected history mutation: %+v %v", view, err)
		}
		badges, err := gamification.Badges(ctx, "plain-user")
		if err != nil || badges.ClaimedDeposits != 10 || !badges.Badges[2].Earned {
			t.Fatalf("badge lost after rejected history mutation: %+v %v", badges, err)
		}
	})
}
