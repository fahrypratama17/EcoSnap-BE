package postgres_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

func TestPointEstimationIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	accounts := postgres.NewAccountRepository(pool)
	if _, _, err := accounts.Register(ctx, "rate-owner", "owner@example.test", service.AccountRegistration{Role: "mitra", FullName: "Owner", Phone: "08123456789", BusinessName: "Tunas", BusinessCategory: "Bank sampah"}); err != nil {
		t.Fatal(err)
	}
	const partner = "20000000-0000-4000-8000-000000000001"
	const plastic = "10000000-0000-4000-8000-000000000001"
	const paper = "10000000-0000-4000-8000-000000000002"
	const glass = "10000000-0000-4000-8000-000000000004"
	const missing = "10000000-0000-4000-8000-000000000099"
	_, err := pool.Exec(ctx, `INSERT INTO public.waste_categories(id,name,slug) VALUES
 ('10000000-0000-4000-8000-000000000001','Plastik','plastic'),
 ('10000000-0000-4000-8000-000000000002','Kertas','paper'),
 ('10000000-0000-4000-8000-000000000004','Kaca','glass');
 INSERT INTO public.partners(id,mitra_uid,business_name,area,address,latitude,longitude,status) VALUES('20000000-0000-4000-8000-000000000001','rate-owner','Tunas','Malang','Demo',-7.9775,112.6304,'verified');
 INSERT INTO public.partner_accepted_waste(id,partner_id,category_id) VALUES
 ('40000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001'),
 ('40000000-0000-4000-8000-000000000002','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000002');
 INSERT INTO public.point_rates(id,category_id,partner_id,points_per_kg,active_from,active_until,is_demo) VALUES
 ('50000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001',NULL,100,'2020-01-01',NULL,true),
 ('50000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000002',NULL,80,'2020-01-01',NULL,true),
 ('50000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001',150,'2020-01-01',NULL,true),
 ('50000000-0000-4000-8000-000000000004','10000000-0000-4000-8000-000000000001',NULL,999,'2021-01-01','2021-02-01',true),
 ('50000000-0000-4000-8000-000000000005','10000000-0000-4000-8000-000000000001',NULL,777,now()+interval '1 day',NULL,true),
 ('50000000-0000-4000-8000-000000000006','10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001',999,now()+interval '1 day',NULL,true);`)
	if err != nil {
		t.Fatal(err)
	}
	estimates := service.NewPointEstimationService(postgres.NewPointRateRepository(pool))
	request := entity.PointEstimateRequest{Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "2.5"}, {CategoryID: paper, WeightKG: "1"}}}
	t.Run("global_and_partner_rates_use_active_versions_and_fallback", func(t *testing.T) {
		result, err := estimates.Estimate(ctx, request)
		if err != nil || result.EstimatedPoints != 330 || result.TotalWeightKG != "3.500" || result.Items[0].RateID != "50000000-0000-4000-8000-000000000001" || result.AsOf.IsZero() || !result.UsesDemoRates {
			t.Fatalf("global: %+v %v", result, err)
		}
		local := request
		id := partner
		local.PartnerID = &id
		result, err = estimates.Estimate(ctx, local)
		if err != nil || result.EstimatedPoints != 455 || result.Items[0].PointsPerKG != "150.00" || result.Items[1].PointsPerKG != "80.00" {
			t.Fatalf("partner/fallback: %+v %v", result, err)
		}
		// A newer active global rate wins globally, while the partner override wins locally.
		_, err = pool.Exec(ctx, `INSERT INTO public.point_rates(id,category_id,points_per_kg,active_from) VALUES('50000000-0000-4000-8000-000000000007',$1,120,now()-interval '1 minute')`, plastic)
		if err != nil {
			t.Fatal(err)
		}
		result, err = estimates.Estimate(ctx, request)
		if err != nil || result.EstimatedPoints != 380 {
			t.Fatalf("newest global: %+v %v", result, err)
		}
		result, err = estimates.Estimate(ctx, local)
		if err != nil || result.EstimatedPoints != 455 {
			t.Fatalf("partner precedence: %+v %v", result, err)
		}
	})
	t.Run("missing_rate_category_partner_and_unaccepted_material", func(t *testing.T) {
		id := partner
		unknown := missing
		for _, tt := range []struct {
			partner  *string
			category string
			want     error
		}{
			{nil, glass, service.ErrPointRateUnavailable}, {nil, missing, service.ErrWasteCategoryNotFound}, {&id, glass, service.ErrWasteNotAccepted}, {&unknown, plastic, service.ErrPartnerNotFound},
		} {
			_, err := estimates.Estimate(ctx, entity.PointEstimateRequest{PartnerID: tt.partner, Items: []entity.PointEstimateInput{{CategoryID: tt.category, WeightKG: "1"}}})
			if !errors.Is(err, tt.want) {
				t.Fatalf("want %v, got %v", tt.want, err)
			}
		}
	})
	t.Run("hidden_location_and_inactive_owner_are_rejected", func(t *testing.T) {
		id := partner
		local := request
		local.PartnerID = &id
		if _, err := pool.Exec(ctx, `UPDATE public.partners SET status='pending' WHERE id=$1`, partner); err != nil {
			t.Fatal(err)
		}
		if _, err := estimates.Estimate(ctx, local); !errors.Is(err, service.ErrPartnerNotFound) {
			t.Fatalf("pending visible: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.partners SET status='verified' WHERE id=$1`, partner); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='rate-owner'`); err != nil {
			t.Fatal(err)
		}
		if _, err := estimates.Estimate(ctx, local); !errors.Is(err, service.ErrPartnerNotFound) {
			t.Fatalf("suspended owner visible: %v", err)
		}
	})
	t.Run("repeated_estimates_leave_wallet_unchanged", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			if _, err := estimates.Estimate(ctx, request); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='active' WHERE firebase_uid='rate-owner'`); err != nil {
			t.Fatal(err)
		}
		profile, err := accounts.Profile(ctx, "rate-owner")
		if err != nil || profile.Wallet.Balance != 0 || profile.Wallet.TotalEarned != 0 || profile.Wallet.TotalSpent != 0 {
			t.Fatalf("wallet mutated: %+v %v", profile, err)
		}
	})
}

func TestPointRateSeedCommandIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `INSERT INTO public.waste_categories(id,name,slug) VALUES
 ('10000000-0000-4000-8000-000000000001','Plastik','plastic'),
 ('10000000-0000-4000-8000-000000000002','Kertas','paper'),
 ('10000000-0000-4000-8000-000000000003','Logam','metal'),
 ('10000000-0000-4000-8000-000000000004','Kaca','glass');`); err != nil {
		t.Fatal(err)
	}
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..", "..")
	cfg := pool.Config().ConnConfig
	dbURL := url.URL{Scheme: "postgresql", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, RawQuery: "sslmode=disable", User: url.UserPassword(cfg.User, cfg.Password)}
	run := func(file string) error {
		cmd := exec.CommandContext(ctx, "go", "run", "-p=1", "-mod=readonly", "./cmd/seed-point-rates", "--demo", "--file", file)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dbURL.String())
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Logf("seed: %s", output)
		}
		return err
	}
	file := filepath.Join(root, "data", "seeders", "dev", "point_rates.json")
	for i := 0; i < 2; i++ {
		if err := run(file); err != nil {
			t.Fatal(err)
		}
	}
	estimates := service.NewPointEstimationService(postgres.NewPointRateRepository(pool))
	request := entity.PointEstimateRequest{Items: []entity.PointEstimateInput{{CategoryID: "10000000-0000-4000-8000-000000000001", WeightKG: "1"}}}
	result, err := estimates.Estimate(ctx, request)
	if err != nil || result.EstimatedPoints != 100 || !result.UsesDemoRates {
		t.Fatalf("seed tariff: %+v %v", result, err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	changed := filepath.Join(t.TempDir(), "changed.json")
	if err = os.WriteFile(changed, []byte(strings.Replace(string(raw), `"100.00"`, `"999.00"`, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err = run(changed); err == nil {
		t.Fatal("seed silently overwrote a tariff")
	}
	result, err = estimates.Estimate(ctx, request)
	if err != nil || result.EstimatedPoints != 100 {
		t.Fatalf("tariff changed after failed seed: %+v %v", result, err)
	}
}
