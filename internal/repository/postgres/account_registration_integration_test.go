package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Each run creates a fresh database on an explicitly configured local test
// cluster. It never falls back to DATABASE_URL or modifies an existing database.
func accountTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("ECOSNAP_TEST_ADMIN_URL")
	if url == "" {
		t.Skip("set ECOSNAP_TEST_ADMIN_URL to a local disposable PostgreSQL cluster")
	}
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test admin connection configuration")
	}
	if cfg.Database != "postgres" || (cfg.Host != "127.0.0.1" && cfg.Host != "localhost" && cfg.Host != "::1") || len(cfg.Fallbacks) != 0 {
		t.Fatal("test admin must target postgres on loopback without fallback hosts (use sslmode=disable)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot connect to local test PostgreSQL:", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = admin.Close(ctx)
	})
	// Supabase defines these roles; a vanilla disposable cluster needs them for
	// the existing migration's REVOKE statements. They grant no login rights.
	_, err = admin.Exec(ctx, `DO $$ BEGIN
 IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='anon') THEN CREATE ROLE anon NOLOGIN; END IF;
 IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname='authenticated') THEN CREATE ROLE authenticated NOLOGIN; END IF;
 END $$;`)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [12]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "ecosnap_account_test_" + hex.EncodeToString(suffix[:])
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Errorf("cleanup database %s: %v", name, err)
		}
	})
	// Copy the parsed config rather than rebuilding a URL with credentials.
	cfg = cfg.Copy()
	cfg.Database = name
	poolCfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid pool configuration")
	}
	poolCfg.ConnConfig = cfg
	poolCfg.MaxConns = 8
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	applyAccountMigrations(t, pool, true)
	return pool
}

func applyAccountMigrations(t *testing.T, pool *pgxpool.Pool, up bool) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate migration directory")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "database", "migrations")
	names := []string{"00001_create_accounts.sql", "00002_create_partner_registrations.sql", "00003_create_partners_and_categories.sql", "00004_create_partner_favorites.sql"}
	if !up {
		for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
			names[i], names[j] = names[j], names[i]
		}
	}
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sections := strings.SplitN(string(raw), "-- +goose Down", 2)
		if len(sections) != 2 {
			t.Fatal("missing migration sections:", name)
		}
		sql := sections[1]
		if up {
			sql = strings.TrimPrefix(sections[0], "-- +goose Up")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, err = pool.Exec(ctx, sql)
		cancel()
		if err != nil {
			t.Fatalf("migration %s up=%v: %v", name, up, err)
		}
	}
}

func TestAccountRepositoryIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	repo := postgres.NewAccountRepository(pool)
	input := service.AccountRegistration{Role: "user", FullName: "Pengguna Test", Phone: "08123456789"}
	t.Run("registration_is_retrievable_with_initial_settings_and_wallet", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, created, err := repo.Register(ctx, "new-user", "user@example.test", input)
		if err != nil || !created {
			t.Fatalf("register: created=%v err=%v", created, err)
		}
		p, err := repo.Profile(ctx, "new-user")
		if err != nil {
			t.Fatal(err)
		}
		if p.UID != "new-user" || p.FullName != "Pengguna Test" || p.DisplayName != "Pengguna Test" || p.Email != "user@example.test" || p.Role != "user" || p.Status != "active" || p.CreatedAt.IsZero() {
			t.Fatalf("wrong profile: %+v", p)
		}
		if p.Wallet.Balance != 0 || p.Wallet.TotalEarned != 0 || p.Wallet.TotalSpent != 0 || !p.Settings.NotificationEnabled || !p.Settings.ShowOnLeaderboard || p.Settings.OnboardingCompleted || p.Business != nil {
			t.Fatalf("wrong initial data: %+v", p)
		}
	})
	t.Run("retry_does_not_replace_profile_or_create_another_wallet", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		p, created, err := repo.Register(ctx, "new-user", "changed@example.test", input)
		if err != nil || created || p.Email != "user@example.test" || p.Wallet.Balance != 0 {
			t.Fatalf("retry: created=%v profile=%+v err=%v", created, p, err)
		}
	})
	t.Run("different_registration_conflicts_without_mutating_profile", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		changed := input
		changed.FullName = "Nama Berbeda"
		_, _, err := repo.Register(ctx, "new-user", "user@example.test", changed)
		if !errors.Is(err, service.ErrAccountConflict) {
			t.Fatalf("want conflict, got %v", err)
		}
		p, err := repo.Profile(ctx, "new-user")
		if err != nil || p.FullName != "Pengguna Test" {
			t.Fatalf("profile changed: %+v %v", p, err)
		}
	})
	t.Run("mitra_registration_remains_pending", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		in := input
		in.Role = "mitra"
		in.BusinessName = "Bank Sampah Test"
		in.BusinessCategory = "Plastik"
		_, _, err := repo.Register(ctx, "mitra-user", "mitra@example.test", in)
		if err != nil {
			t.Fatal(err)
		}
		p, err := repo.Profile(ctx, "mitra-user")
		if err != nil {
			t.Fatal(err)
		}
		if p.Business == nil || p.Business.Name != "Bank Sampah Test" || p.Business.Category != "Plastik" || p.Business.Status != "pending" {
			t.Fatalf("wrong business: %+v", p.Business)
		}
	})
	t.Run("concurrent_retries_create_exactly_one_account", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		const count = 16
		type result struct {
			created bool
			err     error
		}
		results := make(chan result, count)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				p, created, err := repo.Register(ctx, "concurrent-user", "parallel@example.test", input)
				if err == nil && p.UID != "concurrent-user" {
					err = errors.New("wrong UID")
				}
				results <- result{created, err}
			}()
		}
		close(start)
		wg.Wait()
		close(results)
		created := 0
		for result := range results {
			if result.err != nil {
				t.Fatal(result.err)
			}
			if result.created {
				created++
			}
		}
		if created != 1 {
			t.Fatalf("created=%d, expected one", created)
		}
		if _, err := repo.Profile(ctx, "concurrent-user"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("failed_wallet_write_rolls_back_and_can_be_retried", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		// Controlled failure in this disposable database only. Assertions remain
		// through Register/Profile, including retry to detect partial account rows.
		_, err := pool.Exec(ctx, `CREATE FUNCTION public.reject_test_wallet() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test wallet failure'; END $$;
 CREATE TRIGGER reject_test_wallet BEFORE INSERT ON public.wallets FOR EACH ROW EXECUTE FUNCTION public.reject_test_wallet();`)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = pool.Exec(ctx, `DROP TRIGGER IF EXISTS reject_test_wallet ON public.wallets; DROP FUNCTION IF EXISTS public.reject_test_wallet();`)
		})
		if _, _, err = repo.Register(ctx, "rollback-user", "rollback@example.test", input); err == nil {
			t.Fatal("fault injection must fail registration")
		}
		if _, err = repo.Profile(ctx, "rollback-user"); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("failed registration became visible: %v", err)
		}
		_, err = pool.Exec(ctx, `DROP TRIGGER reject_test_wallet ON public.wallets; DROP FUNCTION public.reject_test_wallet();`)
		if err != nil {
			t.Fatal(err)
		}
		if _, created, err := repo.Register(ctx, "rollback-user", "rollback@example.test", input); err != nil || !created {
			t.Fatalf("rollback left partial data: created=%v err=%v", created, err)
		}
	})
	t.Run("failed_business_write_rolls_back_account_settings_and_wallet", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		in := input
		in.Role = "mitra"
		in.BusinessName = "Test Usaha"
		in.BusinessCategory = ""
		if _, _, err := repo.Register(ctx, "business-rollback", "business@example.test", in); err == nil {
			t.Fatal("business constraint must reject registration")
		}
		if _, err := repo.Profile(ctx, "business-rollback"); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("failed business profile visible: %v", err)
		}
		in.BusinessCategory = "Plastik"
		if _, created, err := repo.Register(ctx, "business-rollback", "business@example.test", in); err != nil || !created {
			t.Fatalf("partial rows after business failure: created=%v err=%v", created, err)
		}
	})
	t.Run("missing_profile_returns_not_found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := repo.Profile(ctx, "missing-user"); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("want not found, got %v", err)
		}
	})
	t.Run("migrations_can_be_reversed_and_applied_again", func(t *testing.T) {
		applyAccountMigrations(t, pool, false)
		// Destructive Down/Up recreates enum OIDs: reconnect, as a restarted app would.
		applyAccountMigrations(t, pool, true)
		pool.Reset()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, created, err := repo.Register(ctx, "after-remigration", "new@example.test", input); err != nil || !created {
			t.Fatalf("reapplied migration: created=%v err=%v", created, err)
		}
	})
}
