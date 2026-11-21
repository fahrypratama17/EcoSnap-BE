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
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

func TestVerifyPartnerCommandIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	accounts := postgres.NewAccountRepository(pool)
	for _, uid := range []string{"mitra-new", "mitra-bad"} {
		if _, _, err := accounts.Register(ctx, uid, uid+"@example.test", service.AccountRegistration{Role: "mitra", FullName: "Owner", Phone: "08123456789", BusinessName: "Baru", BusinessCategory: "Bank sampah"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := accounts.Register(ctx, "just-user", "u@example.test", service.AccountRegistration{Role: "user", FullName: "User", Phone: "08123456789"}); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO public.waste_categories(id,name,slug) VALUES
 ('10000000-0000-4000-8000-000000000001','Plastik','plastic'),('10000000-0000-4000-8000-000000000002','Kertas','paper');
 INSERT INTO public.point_rates(id,category_id,points_per_kg,active_from,is_demo) VALUES('50000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001',100,'2020-01-01',true)`)
	if err != nil {
		t.Fatal(err)
	}
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(source), "..", "..", "..")
	bin := filepath.Join(t.TempDir(), "verify-partner.exe")
	build := exec.CommandContext(ctx, "go", "build", "-p=1", "-mod=readonly", "-o", bin, "./cmd/verify-partner")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	cfg := pool.Config().ConnConfig
	dbURL := url.URL{Scheme: "postgresql", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, RawQuery: "sslmode=disable", User: url.UserPassword(cfg.User, cfg.Password)}
	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, bin, args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "DATABASE_URL="+dbURL.String())
		out, err := cmd.CombinedOutput()
		t.Logf("verify-partner %v: %s", args, out)
		return err
	}
	example := filepath.Join(root, "data", "seeders", "dev", "partner_location.example.json")
	deposits := service.NewDepositService(postgres.NewDepositRepository(pool))
	create := func(key string) error {
		_, _, err := deposits.Create(ctx, "mitra-new", key, entity.DepositCreateRequest{Items: []entity.PointEstimateInput{{CategoryID: "10000000-0000-4000-8000-000000000001", WeightKG: "1"}}})
		return err
	}

	if err := create("verify-key-0001"); !errors.Is(err, service.ErrDepositForbidden) {
		t.Fatalf("before approval: %v", err)
	}
	if err := run("approve", "--confirm", "--mitra-uid", "mitra-new", "--location", example); err != nil {
		t.Fatal(err)
	}
	var status, registration string
	var accepted int
	if err := pool.QueryRow(ctx, `SELECT p.status::text,r.status,(SELECT count(*) FROM public.partner_accepted_waste w WHERE w.partner_id=p.id) FROM public.partners p JOIN public.partner_registrations r ON r.user_uid=p.mitra_uid WHERE p.mitra_uid='mitra-new'`).Scan(&status, &registration, &accepted); err != nil || status != "verified" || registration != "approved" || accepted != 2 {
		t.Fatalf("approved: %s %s %d %v", status, registration, accepted, err)
	}
	if err := create("verify-key-0002"); err != nil {
		t.Fatalf("after approval: %v", err)
	}
	if err := run("approve", "--confirm", "--mitra-uid", "mitra-new", "--location", example); err == nil {
		t.Fatal("second approve overwrote location")
	}
	if err := run("set-status", "--confirm", "--mitra-uid", "mitra-new", "--status", "suspended"); err != nil {
		t.Fatal(err)
	}
	if err := create("verify-key-0003"); !errors.Is(err, service.ErrDepositForbidden) {
		t.Fatalf("suspended create: %v", err)
	}
	if err := run("set-status", "--confirm", "--mitra-uid", "mitra-new", "--status", "verified"); err != nil {
		t.Fatal(err)
	}
	if err := create("verify-key-0004"); err != nil {
		t.Fatalf("reverified: %v", err)
	}
	// Rejected inputs change nothing.
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"area":"A","address":"B","latitude":0,"longitude":0,"accepted_waste":["plastic","unknown"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"approve", "--confirm", "--mitra-uid", "mitra-bad", "--location", bad},
		{"approve", "--confirm", "--mitra-uid", "just-user", "--location", example},
		{"approve", "--mitra-uid", "mitra-bad", "--location", example},
		{"set-status", "--confirm", "--mitra-uid", "mitra-bad", "--status", "verified"},
		{"set-status", "--confirm", "--mitra-uid", "mitra-new", "--status", "pending"},
	} {
		if err := run(args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	var partners int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.partners WHERE mitra_uid<>'mitra-new'`).Scan(&partners); err != nil || partners != 0 {
		t.Fatalf("partial location created: %d %v", partners, err)
	}
	if err := run("set-status", "--confirm", "--mitra-uid", "mitra-bad", "--status", "rejected"); err != nil {
		t.Fatal(err)
	}
	if err := run("approve", "--confirm", "--mitra-uid", "mitra-bad", "--location", example); err == nil {
		t.Fatal("rejected registration approved")
	}
}
