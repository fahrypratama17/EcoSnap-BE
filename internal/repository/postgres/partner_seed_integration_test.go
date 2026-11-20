package postgres_test

import (
	"context"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestPartnerSeedCommandIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	accounts := postgres.NewAccountRepository(pool)
	in := service.AccountRegistration{Role: "mitra", FullName: "Pemilik Demo", Phone: "08123456789", BusinessName: "Tunas", BusinessCategory: "Bank sampah"}
	if _, _, err := accounts.Register(ctx, "seed-owner", "demo@example.test", in); err != nil {
		t.Fatal(err)
	}
	_, source, _, _ := runtime.Caller(0)
	runSeed := func(uid string) error {
		cmd := exec.CommandContext(ctx, "go", "run", "-p=1", "-mod=readonly", "./cmd/seed-partners", "--demo", "--mitra-uid", uid)
		cmd.Dir = filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", ".."))
		cfg := pool.Config().ConnConfig
		databaseURL := url.URL{Scheme: "postgresql", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, RawQuery: "sslmode=disable"}
		databaseURL.User = url.UserPassword(cfg.User, cfg.Password)
		cmd.Env = append(os.Environ(), "DATABASE_URL="+databaseURL.String())
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Logf("seed output: %s", output)
			return err
		}
		return nil
	}
	for i := 0; i < 2; i++ {
		if err := runSeed("seed-owner"); err != nil {
			t.Fatal(err)
		}
	}
	partners := postgres.NewPartnerRepository(pool)
	items, err := partners.List(ctx, service.PartnerFilter{Limit: 20})
	if err != nil || len(items) != 1 {
		t.Fatalf("seed duplicated or missing location: %+v %v", items, err)
	}
	if len(items[0].Hours) != 7 || len(items[0].AcceptedMaterials) != 3 || items[0].Name != "Bank Sampah Tunas" {
		t.Fatalf("wrong seed: %+v", items[0])
	}
	categories, err := partners.Categories(ctx)
	if err != nil || len(categories) != 4 {
		t.Fatalf("wrong category seed: %+v %v", categories, err)
	}
	profile, err := accounts.Profile(ctx, "seed-owner")
	if err != nil || profile.Business == nil || profile.Business.Status != "approved" {
		t.Fatalf("wrong approval: %+v %v", profile, err)
	}
	in.Role = "user"
	in.BusinessName = ""
	in.BusinessCategory = ""
	if _, _, err = accounts.Register(ctx, "seed-user", "user@example.test", in); err != nil {
		t.Fatal(err)
	}
	if err = runSeed("seed-user"); err == nil {
		t.Fatal("seed must not upgrade a regular user")
	}
	profile, err = accounts.Profile(ctx, "seed-user")
	if err != nil || profile.Role != "user" {
		t.Fatal("seed changed user role", err)
	}
}
