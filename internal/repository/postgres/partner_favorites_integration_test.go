package postgres_test

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"sync"
	"testing"
	"time"
)

func TestPartnerFavoritesIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	accounts := postgres.NewAccountRepository(pool)
	for _, uid := range []string{"alice", "bob", "owner"} {
		role := "user"
		in := service.AccountRegistration{Role: role, FullName: uid, Phone: "08123456789"}
		if uid == "owner" {
			in.Role = "mitra"
			in.BusinessName = "Tunas"
			in.BusinessCategory = "Plastik"
		}
		if _, _, err := accounts.Register(ctx, uid, uid+"@example.test", in); err != nil {
			t.Fatal(err)
		}
	}
	const id = "20000000-0000-4000-8000-000000000001"
	_, err := pool.Exec(ctx, `INSERT INTO public.partners(id,mitra_uid,business_name,area,address,latitude,longitude,status) VALUES($1,'owner','Tunas','Malang','Alamat simulasi',-7.9775,112.6304,'verified');`, id)
	if err != nil {
		t.Fatal(err)
	}
	repo := postgres.NewFavoriteRepository(pool)
	t.Run("concurrent_add_is_idempotent_and_owned_by_caller", func(t *testing.T) {
		results := make(chan error, 12)
		var wg sync.WaitGroup
		for i := 0; i < 12; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); results <- repo.Set(ctx, "alice", id, true) }()
		}
		wg.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
		items, err := repo.List(ctx, "alice", 21, 0)
		if err != nil || len(items) != 1 || items[0].PartnerID != id || items[0].CreatedAt.IsZero() {
			t.Fatalf("alice favorites: %+v %v", items, err)
		}
		if items, err = repo.List(ctx, "bob", 21, 0); err != nil || len(items) != 0 {
			t.Fatalf("bob saw alice data: %+v %v", items, err)
		}
		if err = repo.Set(ctx, "bob", id, false); err != nil {
			t.Fatal(err)
		}
		if items, err = repo.List(ctx, "alice", 21, 0); err != nil || len(items) != 1 {
			t.Fatal("bob changed alice favorite", err)
		}
	})
	t.Run("hidden_partner_cannot_be_added_and_can_be_removed", func(t *testing.T) {
		if _, err = pool.Exec(ctx, `UPDATE public.partners SET status='suspended' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if err = repo.Set(ctx, "bob", id, true); !errors.Is(err, service.ErrPartnerNotFound) {
			t.Fatalf("hidden target: %v", err)
		}
		if items, err := repo.List(ctx, "alice", 21, 0); err != nil || len(items) != 0 {
			t.Fatalf("hidden favorite visible: %+v %v", items, err)
		}
		for i := 0; i < 2; i++ {
			if err = repo.Set(ctx, "alice", id, false); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = pool.Exec(ctx, `UPDATE public.partners SET status='verified' WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
		if items, err := repo.List(ctx, "alice", 21, 0); err != nil || len(items) != 0 {
			t.Fatalf("removed favorite returned: %+v %v", items, err)
		}
	})
	t.Run("missing_and_suspended_accounts_are_rejected", func(t *testing.T) {
		if err = repo.Set(ctx, "missing", id, true); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='bob'`); err != nil {
			t.Fatal(err)
		}
		if err = repo.Set(ctx, "bob", id, true); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatal(err)
		}
		if _, err = repo.List(ctx, "bob", 21, 0); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatal(err)
		}
	})
	t.Run("partner_read_queries_filter_name_material_and_visibility", func(t *testing.T) {
		partners := postgres.NewPartnerRepository(pool)
		if _, err = pool.Exec(ctx, `INSERT INTO public.waste_categories(id,name,slug) VALUES('10000000-0000-4000-8000-000000000001','Plastik','plastic');
   INSERT INTO public.partner_accepted_waste(id,partner_id,category_id) VALUES('40000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001');`); err != nil {
			t.Fatal(err)
		}
		items, err := partners.List(ctx, service.PartnerFilter{Query: "malang", Material: "plastic", Limit: 21})
		if err != nil || len(items) != 1 {
			t.Fatalf("list: %+v %v", items, err)
		}
		detail, err := partners.Detail(ctx, id)
		if err != nil || len(detail.AcceptedMaterials) != 1 || detail.AcceptedMaterials[0].Code != "PLASTIC" || detail.Hours == nil {
			t.Fatalf("detail: %+v %v", detail, err)
		}
		if items, err = partners.List(ctx, service.PartnerFilter{Material: "glass", Limit: 21}); err != nil || len(items) != 0 {
			t.Fatalf("material filter: %+v %v", items, err)
		}
		categories, err := partners.Categories(ctx)
		if err != nil || len(categories) != 1 {
			t.Fatalf("categories: %+v %v", categories, err)
		}
		if _, err = pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='owner'`); err != nil {
			t.Fatal(err)
		}
		if _, err = partners.Detail(ctx, id); !errors.Is(err, service.ErrPartnerNotFound) {
			t.Fatalf("suspended owner visible: %v", err)
		}
	})
}
