package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	plastic = "10000000-0000-4000-8000-000000000001"
	paper   = "10000000-0000-4000-8000-000000000002"
	glass   = "10000000-0000-4000-8000-000000000004"
)

// depositFixture: verified owner-a (plastic local 150, paper global 80),
// verified owner-b, pending owner-pending, and users plain-user/user-2/user-3.
func depositFixture(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	accounts := postgres.NewAccountRepository(pool)
	mitra := service.AccountRegistration{Role: "mitra", FullName: "Owner", Phone: "08123456789", BusinessName: "Tunas", BusinessCategory: "Bank sampah"}
	for _, uid := range []string{"owner-a", "owner-b", "owner-pending"} {
		if _, _, err := accounts.Register(ctx, uid, uid+"@example.test", mitra); err != nil {
			t.Fatal(err)
		}
	}
	for _, uid := range []string{"plain-user", "user-2", "user-3"} {
		if _, _, err := accounts.Register(ctx, uid, uid+"@example.test", service.AccountRegistration{Role: "user", FullName: "User", Phone: "08123456789"}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.waste_categories(id,name,slug) VALUES
 ('10000000-0000-4000-8000-000000000001','Plastik','plastic'),
 ('10000000-0000-4000-8000-000000000002','Kertas','paper'),
 ('10000000-0000-4000-8000-000000000004','Kaca','glass');
 INSERT INTO public.partners(id,mitra_uid,business_name,area,address,latitude,longitude,status) VALUES
 ('20000000-0000-4000-8000-000000000001','owner-a','Tunas','Malang','Demo',-7.9775,112.6304,'verified'),
 ('20000000-0000-4000-8000-000000000002','owner-b','Lain','Malang','Demo',-7.9775,112.6304,'verified'),
 ('20000000-0000-4000-8000-000000000003','owner-pending','Pending','Malang','Demo',-7.9775,112.6304,'pending');
 INSERT INTO public.partner_accepted_waste(id,partner_id,category_id) VALUES
 ('40000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001'),
 ('40000000-0000-4000-8000-000000000002','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000002'),
 ('40000000-0000-4000-8000-000000000003','20000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000001');
 INSERT INTO public.point_rates(id,category_id,partner_id,points_per_kg,active_from,active_until,is_demo) VALUES
 ('50000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001',NULL,100,'2020-01-01',NULL,true),
 ('50000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000002',NULL,80,'2020-01-01',NULL,true),
 ('50000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001',150,'2020-01-01',NULL,true);`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDepositIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	deposits := service.NewDepositService(postgres.NewDepositRepository(pool))
	estimates := service.NewPointEstimationService(postgres.NewPointRateRepository(pool))
	request := entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "2.5"}, {CategoryID: paper, WeightKG: "1"}}}
	count := func(t *testing.T) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.deposits`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	var first entity.Deposit
	t.Run("server_total_matches_estimator_and_snapshot_is_retrievable", func(t *testing.T) {
		partner := "20000000-0000-4000-8000-000000000001"
		estimate, err := estimates.Estimate(ctx, entity.PointEstimateRequest{PartnerID: &partner, Items: request.Items})
		if err != nil {
			t.Fatal(err)
		}
		var created bool
		first, created, err = deposits.Create(ctx, "owner-a", "key-00000001", request)
		// Partner plastic 150*2.5 + global paper 80*1 = 455.
		if err != nil || !created || first.TotalPoints != 455 || first.TotalPoints != estimate.EstimatedPoints || first.PartnerID != partner || first.Status != "created" || first.TotalWeightKG != "3.500" || len(first.Items) != 2 || first.Items[0].RateID != "50000000-0000-4000-8000-000000000003" {
			t.Fatalf("create: %+v created=%v err=%v", first, created, err)
		}
		got, err := deposits.Detail(ctx, "owner-a", first.ID)
		if err != nil || got.TotalPoints != 455 || len(got.Items) != 2 || got.Items[0].ExactPoints != first.Items[0].ExactPoints || got.Items[1].CategoryName != "Kertas" {
			t.Fatalf("detail: %+v %v", got, err)
		}
	})
	t.Run("tariff_changes_do_not_rewrite_history", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `INSERT INTO public.point_rates(id,category_id,partner_id,points_per_kg,active_from,is_demo) VALUES('50000000-0000-4000-8000-000000000009',$1,'20000000-0000-4000-8000-000000000001',999,now()-interval '1 second',true)`, plastic); err != nil {
			t.Fatal(err)
		}
		got, err := deposits.Detail(ctx, "owner-a", first.ID)
		if err != nil || got.TotalPoints != 455 || got.Items[0].PointsPerKG != "150.00" {
			t.Fatalf("history changed: %+v %v", got, err)
		}
	})
	t.Run("retry_replays_and_changed_payload_conflicts", func(t *testing.T) {
		before := count(t)
		again, created, err := deposits.Create(ctx, "owner-a", "key-00000001", request)
		if err != nil || created || again.ID != first.ID || again.TotalPoints != 455 {
			t.Fatalf("replay: %+v created=%v err=%v", again, created, err)
		}
		changed := entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "3"}}}
		if _, _, err := deposits.Create(ctx, "owner-a", "key-00000001", changed); !errors.Is(err, service.ErrDepositConflict) {
			t.Fatalf("want conflict, got %v", err)
		}
		if count(t) != before {
			t.Fatal("retry inserted a row")
		}
	})
	t.Run("parallel_retries_create_one_deposit", func(t *testing.T) {
		before := count(t)
		var wg sync.WaitGroup
		var mu sync.Mutex
		ids, createdCount := map[string]bool{}, 0
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got, created, err := deposits.Create(ctx, "owner-a", "key-parallel-1", request)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					t.Error(err)
					return
				}
				ids[got.ID] = true
				if created {
					createdCount++
				}
			}()
		}
		wg.Wait()
		if len(ids) != 1 || createdCount != 1 || count(t) != before+1 {
			t.Fatalf("ids=%v created=%d rows=%d", ids, createdCount, count(t)-before)
		}
	})
	t.Run("authorization_and_rejections_leave_no_rows", func(t *testing.T) {
		before := count(t)
		if _, err := deposits.Detail(ctx, "owner-b", first.ID); !errors.Is(err, service.ErrDepositNotFound) {
			t.Fatalf("other mitra read deposit: %v", err)
		}
		for _, tt := range []struct {
			uid  string
			req  entity.DepositCreateRequest
			want error
		}{
			{"plain-user", request, service.ErrDepositForbidden},
			{"owner-pending", entity.DepositCreateRequest{Items: request.Items[:1]}, service.ErrDepositForbidden},
			{"missing-uid", request, service.ErrAccountNotFound},
			{"owner-a", entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "1"}, {CategoryID: glass, WeightKG: "1"}}}, service.ErrWasteNotAccepted},
			{"owner-a", entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "0"}}}, service.ErrInvalidEstimate},
			// The tariff subtest raised plastic to 999/kg: 0.001 kg → 0.999 → 0 points.
			{"owner-a", entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "0.001"}}}, service.ErrZeroPoints},
		} {
			if _, _, err := deposits.Create(ctx, tt.uid, "key-reject-1", tt.req); !errors.Is(err, tt.want) {
				t.Fatalf("%s: want %v, got %v", tt.uid, tt.want, err)
			}
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='owner-b'`); err != nil {
			t.Fatal(err)
		}
		if _, _, err := deposits.Create(ctx, "owner-b", "key-reject-2", request); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("suspended: %v", err)
		}
		if count(t) != before {
			t.Fatal("rejected request inserted a deposit")
		}
	})
	t.Run("creation_does_not_credit_wallets", func(t *testing.T) {
		var total int64
		if err := pool.QueryRow(ctx, `SELECT COALESCE(sum(balance+total_earned),0) FROM public.wallets`).Scan(&total); err != nil || total != 0 {
			t.Fatalf("wallets changed: %d %v", total, err)
		}
	})
}
