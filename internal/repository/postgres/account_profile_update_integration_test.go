package postgres_test

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"strings"
	"testing"
	"time"
)

func TestAccountProfileUpdateIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repo := postgres.NewAccountRepository(pool)
	accounts := service.NewAccountService(repo)
	for _, uid := range []string{"alice-update", "bob-update"} {
		in := service.AccountRegistration{Role: "user", FullName: "Nama Awal", Phone: "08123456789"}
		if _, _, err := accounts.Register(ctx, uid, uid+"@example.test", in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE public.wallets SET balance=250,total_earned=250 WHERE user_uid='alice-update'`); err != nil {
		t.Fatal(err)
	}
	name := "  Nama Baru  "
	p, err := accounts.UpdateProfile(ctx, "alice-update", service.AccountProfileUpdate{FullName: &name})
	if err != nil || p.FullName != "Nama Baru" || p.DisplayName != "Nama Baru" || p.Phone != "08123456789" || p.Email != "alice-update@example.test" || p.Role != "user" || p.Wallet.Balance != 250 || !p.Settings.NotificationEnabled {
		t.Fatalf("partial update lost data: %+v %v", p, err)
	}
	longName := strings.Repeat("名", 100)
	phone := "+628123456700"
	results := make(chan error, 2)
	go func() {
		_, err := accounts.UpdateProfile(ctx, "alice-update", service.AccountProfileUpdate{FullName: &longName})
		results <- err
	}()
	go func() {
		_, err := accounts.UpdateProfile(ctx, "alice-update", service.AccountProfileUpdate{Phone: &phone})
		results <- err
	}()
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	p, err = accounts.Profile(ctx, "alice-update")
	if err != nil || p.FullName != longName || p.DisplayName != strings.Repeat("名", 80) || p.Phone != phone || p.Wallet.Balance != 250 {
		t.Fatalf("parallel partial updates: %+v %v", p, err)
	}
	if _, err = accounts.UpdateProfile(ctx, "alice-update", service.AccountProfileUpdate{Phone: &phone}); err != nil {
		t.Fatal(err)
	}
	bob, err := accounts.Profile(ctx, "bob-update")
	if err != nil || bob.FullName != "Nama Awal" || bob.Phone != "08123456789" {
		t.Fatalf("another user changed: %+v %v", bob, err)
	}
	if _, err = accounts.UpdateProfile(ctx, "missing-update", service.AccountProfileUpdate{Phone: &phone}); !errors.Is(err, service.ErrAccountNotFound) {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='alice-update'`); err != nil {
		t.Fatal(err)
	}
	forbidden := "Dilarang"
	if _, err = accounts.UpdateProfile(ctx, "alice-update", service.AccountProfileUpdate{FullName: &forbidden}); !errors.Is(err, service.ErrAccountForbidden) {
		t.Fatalf("suspended update: %v", err)
	}
	stored, err := repo.Profile(ctx, "alice-update")
	if err != nil || stored.FullName != longName {
		t.Fatal("rejected update changed data", err)
	}
}
