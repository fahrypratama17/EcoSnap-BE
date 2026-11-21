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
)

func TestSettingsRepositoryIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	accountsRepo := postgres.NewAccountRepository(pool)
	store := postgres.NewSettingsRepository(pool)
	settings := service.NewSettingsService(store)

	for _, role := range []string{"user", "mitra"} {
		uid := "settings-" + role
		in := service.AccountRegistration{Role: role, FullName: "Nama Awal", Phone: "08123456789"}
		if role == "mitra" {
			in.BusinessName, in.BusinessCategory = "Bank Sampah Test", "Plastik"
		}
		if _, _, err := accountsRepo.Register(ctx, uid, uid+"@example.test", in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE public.wallets SET balance=250,total_earned=300,total_spent=50 WHERE user_uid='settings-user'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.user_settings SET show_on_leaderboard=false WHERE firebase_uid='settings-user'`); err != nil {
		t.Fatal(err)
	}

	t.Run("both_roles_read_defaults_and_patch_with_explicit_false", func(t *testing.T) {
		for _, uid := range []string{"settings-user", "settings-mitra"} {
			got, err := settings.Get(ctx, uid)
			if err != nil || !got.NotificationEnabled || got.OnboardingCompleted {
				t.Fatalf("%s defaults: %+v %v", uid, got, err)
			}
			showBefore := got.ShowOnLeaderboard
			got, err = settings.Update(ctx, uid, service.AccountSettingsUpdate{NotificationEnabled: new(false)})
			if err != nil || got.NotificationEnabled || got.OnboardingCompleted || got.ShowOnLeaderboard != showBefore {
				t.Fatalf("%s false patch: %+v %v", uid, got, err)
			}
			got, err = settings.Update(ctx, uid, service.AccountSettingsUpdate{OnboardingCompleted: new(true)})
			if err != nil || got.NotificationEnabled || !got.OnboardingCompleted || got.ShowOnLeaderboard != showBefore {
				t.Fatalf("%s omitted preserved: %+v %v", uid, got, err)
			}
			got, err = settings.Update(ctx, uid, service.AccountSettingsUpdate{NotificationEnabled: new(true), OnboardingCompleted: new(false)})
			if err != nil || !got.NotificationEnabled || got.OnboardingCompleted {
				t.Fatalf("%s both patch: %+v %v", uid, got, err)
			}
		}
	})

	t.Run("persistence_visible_through_profile_and_nothing_else_changes", func(t *testing.T) {
		before, err := accountsRepo.Profile(ctx, "settings-user")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = settings.Update(ctx, "settings-user", service.AccountSettingsUpdate{NotificationEnabled: new(false), OnboardingCompleted: new(true)}); err != nil {
			t.Fatal(err)
		}
		after, err := accountsRepo.Profile(ctx, "settings-user")
		if err != nil {
			t.Fatal(err)
		}
		want := entity.AccountSettings{NotificationEnabled: false, ShowOnLeaderboard: false, OnboardingCompleted: true}
		if after.Settings != want {
			t.Fatalf("profile settings: %+v", after.Settings)
		}
		if after.Wallet != before.Wallet || after.Wallet.Balance != 250 || after.Wallet.TotalEarned != 300 || after.Wallet.TotalSpent != 50 ||
			after.FullName != before.FullName || after.DisplayName != before.DisplayName || after.Phone != before.Phone ||
			after.Email != before.Email || after.Role != before.Role || after.Status != before.Status || !after.CreatedAt.Equal(before.CreatedAt) {
			t.Fatalf("profile/wallet altered: before=%+v after=%+v", before, after)
		}
		var wallets int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.wallets`).Scan(&wallets); err != nil || wallets != 2 {
			t.Fatalf("wallet rows: %d %v", wallets, err)
		}
	})

	t.Run("another_user_is_isolated", func(t *testing.T) {
		mitraBefore, err := settings.Get(ctx, "settings-mitra")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = settings.Update(ctx, "settings-user", service.AccountSettingsUpdate{NotificationEnabled: new(true), OnboardingCompleted: new(false)}); err != nil {
			t.Fatal(err)
		}
		mitraAfter, err := settings.Get(ctx, "settings-mitra")
		if err != nil || mitraAfter != mitraBefore {
			t.Fatalf("other account changed: %+v -> %+v %v", mitraBefore, mitraAfter, err)
		}
	})

	t.Run("leaderboard_visibility_is_never_written", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE public.user_settings SET show_on_leaderboard=true WHERE firebase_uid='settings-user'`); err != nil {
			t.Fatal(err)
		}
		got, err := settings.Update(ctx, "settings-user", service.AccountSettingsUpdate{NotificationEnabled: new(false), OnboardingCompleted: new(true)})
		if err != nil || !got.ShowOnLeaderboard {
			t.Fatalf("visibility changed: %+v %v", got, err)
		}
		if _, err = pool.Exec(ctx, `UPDATE public.user_settings SET show_on_leaderboard=false WHERE firebase_uid='settings-user'`); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("concurrent_disjoint_patches_both_survive", func(t *testing.T) {
		for round := range 25 {
			startNotify, startOnboard := round%2 == 0, round%2 == 1
			if _, err := pool.Exec(ctx, `UPDATE public.user_settings SET notification_enabled=$1,onboarding_completed=$2 WHERE firebase_uid='settings-user'`, startNotify, startOnboard); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			start := make(chan struct{})
			errs := make(chan error, 2)
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				_, err := store.UpdateSettings(ctx, "settings-user", service.AccountSettingsUpdate{NotificationEnabled: new(!startNotify)})
				errs <- err
			}()
			go func() {
				defer wg.Done()
				<-start
				_, err := store.UpdateSettings(ctx, "settings-user", service.AccountSettingsUpdate{OnboardingCompleted: new(!startOnboard)})
				errs <- err
			}()
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			var n, o, s bool
			if err := pool.QueryRow(ctx, `SELECT notification_enabled,onboarding_completed,show_on_leaderboard FROM public.user_settings WHERE firebase_uid='settings-user'`).Scan(&n, &o, &s); err != nil {
				t.Fatal(err)
			}
			if n != !startNotify || o != !startOnboard || s {
				t.Fatalf("round %d lost update: notification=%v onboarding=%v leaderboard=%v", round, n, o, s)
			}
		}
	})

	t.Run("suspended_and_deleted_are_forbidden_without_writes", func(t *testing.T) {
		for _, uid := range []string{"settings-user", "settings-mitra"} {
			if _, err := pool.Exec(ctx, `UPDATE public.user_settings SET notification_enabled=true,onboarding_completed=false WHERE firebase_uid=$1`, uid); err != nil {
				t.Fatal(err)
			}
		}
		for i, status := range []string{"suspended", "deleted"} {
			uid := []string{"settings-user", "settings-mitra"}[i]
			if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status=$2::public.account_status WHERE firebase_uid=$1`, uid, status); err != nil {
				t.Fatal(err)
			}
			if _, err := settings.Get(ctx, uid); !errors.Is(err, service.ErrAccountForbidden) {
				t.Fatalf("%s get: %v", status, err)
			}
			_, err := settings.Update(ctx, uid, service.AccountSettingsUpdate{NotificationEnabled: new(false), OnboardingCompleted: new(true)})
			if !errors.Is(err, service.ErrAccountForbidden) {
				t.Fatalf("%s update: %v", status, err)
			}
			var n, o bool
			if err = pool.QueryRow(ctx, `SELECT notification_enabled,onboarding_completed FROM public.user_settings WHERE firebase_uid=$1`, uid).Scan(&n, &o); err != nil || !n || o {
				t.Fatalf("%s rejected update wrote data: %v %v %v", status, n, o, err)
			}
		}
	})

	t.Run("missing_account_and_missing_settings_row_are_not_found_and_not_created", func(t *testing.T) {
		if _, err := settings.Get(ctx, "nobody"); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("missing get: %v", err)
		}
		if _, err := settings.Update(ctx, "nobody", service.AccountSettingsUpdate{NotificationEnabled: new(true)}); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("missing update: %v", err)
		}
		if _, _, err := accountsRepo.Register(ctx, "no-settings", "ns@example.test", service.AccountRegistration{Role: "user", FullName: "Tanpa Setelan", Phone: "08123456789"}); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `DELETE FROM public.user_settings WHERE firebase_uid='no-settings'`); err != nil {
			t.Fatal(err)
		}
		if _, err := settings.Get(ctx, "no-settings"); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("missing row get: %v", err)
		}
		if _, err := settings.Update(ctx, "no-settings", service.AccountSettingsUpdate{OnboardingCompleted: new(true)}); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("missing row update: %v", err)
		}
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.user_settings WHERE firebase_uid IN ('nobody','no-settings')`).Scan(&n); err != nil || n != 0 {
			t.Fatalf("settings rows created implicitly: %d %v", n, err)
		}
	})

	t.Run("empty_update_is_rejected_before_store", func(t *testing.T) {
		if _, err := settings.Update(ctx, "settings-mitra", service.AccountSettingsUpdate{}); !errors.Is(err, service.ErrInvalidSettingsUpdate) {
			t.Fatalf("empty update: %v", err)
		}
	})
}
