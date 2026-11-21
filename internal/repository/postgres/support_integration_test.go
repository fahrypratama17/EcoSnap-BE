package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

func TestSupportIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	accounts := postgres.NewAccountRepository(pool)
	for _, a := range []struct{ uid, role string }{{"sup-user", "user"}, {"sup-other", "user"}, {"sup-mitra", "mitra"}, {"sup-flood", "user"}, {"sup-race", "user"}, {"sup-suspended", "user"}} {
		in := service.AccountRegistration{Role: a.role, FullName: "Pengguna", Phone: "08123456789"}
		if a.role == "mitra" {
			in.BusinessName, in.BusinessCategory = "Bank Sampah", "Bank sampah"
		}
		if _, _, err := accounts.Register(ctx, a.uid, a.uid+"@example.test", in); err != nil {
			t.Fatal(err)
		}
	}
	support := service.NewSupportService(postgres.NewSupportRepository(pool))
	req := service.SupportTicketRequest{Kind: "problem", Subject: "QR gagal", Message: "Tidak terbaca di toko"}

	t.Run("faq_for_any_active_role_not_for_inactive", func(t *testing.T) {
		for _, uid := range []string{"sup-user", "sup-mitra"} {
			items, err := support.FAQs(ctx, uid)
			if err != nil || len(items) < 4 || !items[0].IsDemo {
				t.Fatalf("%s: %d %v", uid, len(items), err)
			}
		}
		if _, err := support.FAQs(ctx, "ghost"); !errors.Is(err, service.ErrAccountNotFound) {
			t.Fatalf("unknown: %v", err)
		}
	})
	var first string
	t.Run("create_replay_and_conflict", func(t *testing.T) {
		got, created, err := support.Create(ctx, "sup-user", "support-0001", req)
		if err != nil || !created || got.Status != "open" || got.StaffResponse != nil || got.RespondedAt != nil || !got.UpdatedAt.Equal(got.CreatedAt) {
			t.Fatalf("create: %+v %v %v", got, created, err)
		}
		first = got.ID
		same := req
		same.Message = " Tidak terbaca di toko \r\n"
		again, created, err := support.Create(ctx, "sup-user", "support-0001", same)
		if err != nil || created || again.ID != first {
			t.Fatalf("replay: %+v %v %v", again, created, err)
		}
		other := req
		other.Kind = "feedback"
		if _, _, err = support.Create(ctx, "sup-user", "support-0001", other); !errors.Is(err, service.ErrSupportTicketConflict) {
			t.Fatalf("conflict: %v", err)
		}
		// The same key belongs to another account independently.
		if o, created, err := support.Create(ctx, "sup-other", "support-0001", other); err != nil || !created || o.ID == first {
			t.Fatalf("other account same key: %+v %v %v", o, created, err)
		}
		if got, created, err = support.Create(ctx, "sup-mitra", "support-0001", req); err != nil || !created {
			t.Fatalf("mitra without verified location: %+v %v %v", got, created, err)
		}
	})
	t.Run("concurrent_retries_create_one_ticket", func(t *testing.T) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		createdCount := 0
		for range 12 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, created, err := support.Create(ctx, "sup-race", "race-key-0001", req)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					t.Error(err)
				}
				if created {
					createdCount++
				}
			}()
		}
		wg.Wait()
		var rows int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.support_tickets WHERE user_uid='sup-race'`).Scan(&rows); err != nil || rows != 1 || createdCount != 1 {
			t.Fatalf("rows=%d created=%d %v", rows, createdCount, err)
		}
	})
	t.Run("unresolved_cap_is_enforced_under_concurrency_and_retries_still_replay", func(t *testing.T) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		okCount, limited := 0, 0
		for i := range 30 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err := support.Create(ctx, "sup-flood", fmt.Sprintf("flood-key-%04d", i), req)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					okCount++
				case errors.Is(err, service.ErrSupportTicketLimit):
					limited++
				default:
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		if okCount != service.MaxUnresolvedSupportTickets || limited != 30-service.MaxUnresolvedSupportTickets {
			t.Fatalf("ok=%d limited=%d", okCount, limited)
		}
		var existing string
		if err := pool.QueryRow(ctx, `SELECT idempotency_key FROM public.support_tickets WHERE user_uid='sup-flood' LIMIT 1`).Scan(&existing); err != nil {
			t.Fatal(err)
		}
		if _, created, err := support.Create(ctx, "sup-flood", existing, req); err != nil || created {
			t.Fatalf("replay at cap: %v %v", created, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.support_tickets SET status='resolved' WHERE id=(SELECT id FROM public.support_tickets WHERE user_uid='sup-flood' LIMIT 1)`); err != nil {
			t.Fatal(err)
		}
		if _, created, err := support.Create(ctx, "sup-flood", "flood-after-resolve", req); err != nil || !created {
			t.Fatalf("resolving frees a slot: %v %v", created, err)
		}
	})
	t.Run("list_detail_and_isolation", func(t *testing.T) {
		for i := range 2 {
			if _, _, err := support.Create(ctx, "sup-user", fmt.Sprintf("support-more-%02d", i), req); err != nil {
				t.Fatal(err)
			}
		}
		p1, err := support.Tickets(ctx, "sup-user", service.HistoryQuery{Limit: 2})
		if err != nil || len(p1.Items) != 2 || p1.NextCursor == "" || !p1.Items[0].CreatedAt.After(p1.Items[1].CreatedAt) {
			t.Fatalf("page1: %+v %v", p1, err)
		}
		p2, err := support.Tickets(ctx, "sup-user", service.HistoryQuery{Limit: 2, Cursor: p1.NextCursor})
		if err != nil || len(p2.Items) != 1 || p2.Items[0].ID != first || p2.NextCursor != "" {
			t.Fatalf("page2: %+v %v", p2, err)
		}
		if _, err = support.Ticket(ctx, "sup-user", first); err != nil {
			t.Fatal(err)
		}
		if _, err = support.Ticket(ctx, "sup-other", first); !errors.Is(err, service.ErrSupportTicketNotFound) {
			t.Fatalf("other account must get not found: %v", err)
		}
		if page, err := support.Tickets(ctx, "sup-other", service.HistoryQuery{Limit: 10}); err != nil || len(page.Items) != 1 || page.Items[0].ID == first {
			t.Fatalf("other list: %+v %v", page.Items, err)
		}
	})
	t.Run("inactive_account_is_blocked_and_original_data_is_immutable", func(t *testing.T) {
		if _, _, err := support.Create(ctx, "sup-suspended", "suspended-key-1", req); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='sup-suspended'`); err != nil {
			t.Fatal(err)
		}
		if _, _, err := support.Create(ctx, "sup-suspended", "suspended-key-2", req); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("create: %v", err)
		}
		if _, err := support.Tickets(ctx, "sup-suspended", service.HistoryQuery{Limit: 5}); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("list: %v", err)
		}
		if _, err := support.FAQs(ctx, "sup-suspended"); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("faq: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.support_tickets SET message='diubah' WHERE id=$1::uuid`, first); err == nil {
			t.Fatal("original message must be immutable")
		}
		if _, err := pool.Exec(ctx, `DELETE FROM public.support_tickets WHERE id=$1::uuid`, first); err == nil {
			t.Fatal("tickets must not be deletable")
		}
		var rls bool
		var anonGrant bool
		if err := pool.QueryRow(ctx, `SELECT relrowsecurity FROM pg_class WHERE oid='public.support_tickets'::regclass`).Scan(&rls); err != nil || !rls {
			t.Fatalf("rls: %v %v", rls, err)
		}
		if err := pool.QueryRow(ctx, `SELECT has_table_privilege('anon','public.support_tickets','SELECT') OR has_table_privilege('authenticated','public.support_tickets','SELECT')`).Scan(&anonGrant); err != nil || anonGrant {
			t.Fatalf("grant: %v %v", anonGrant, err)
		}
	})
	t.Run("owner_cli_lifecycle_is_visible_to_the_user", func(t *testing.T) {
		_, source, _, _ := runtime.Caller(0)
		root := filepath.Join(filepath.Dir(source), "..", "..", "..")
		bin := filepath.Join(t.TempDir(), "support-tickets.exe")
		build := exec.CommandContext(ctx, "go", "build", "-p=1", "-mod=readonly", "-o", bin, "./cmd/support-tickets")
		build.Dir = root
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build: %v %s", err, out)
		}
		cfg := pool.Config().ConnConfig
		dbURL := url.URL{Scheme: "postgresql", Host: net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))), Path: "/" + cfg.Database, RawQuery: "sslmode=disable", User: url.UserPassword(cfg.User, cfg.Password)}
		run := func(env string, args ...string) (string, error) {
			cmd := exec.CommandContext(ctx, bin, args...)
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "DATABASE_URL="+env)
			out, err := cmd.CombinedOutput()
			return string(out), err
		}
		mustRun := func(args ...string) string {
			t.Helper()
			out, err := run(dbURL.String(), args...)
			if err != nil {
				t.Fatalf("%v: %v %s", args, err, out)
			}
			return out
		}
		if out := mustRun("list", "--id", first); !strings.Contains(out, first) || !strings.Contains(out, "sup-user") || !strings.Contains(out, `"status": "open"`) {
			t.Fatalf("list: %s", out)
		}
		if _, err := run(dbURL.String(), "reply", "--id", first, "--message", "tanpa konfirmasi"); err == nil {
			t.Fatal("reply without --confirm must fail")
		}
		if got, _ := support.Ticket(ctx, "sup-user", first); got.StaffResponse != nil || got.Status != "open" {
			t.Fatalf("a rejected command must change nothing: %+v", got)
		}
		mustRun("reply", "--id", first, "--message", "Terima kasih, kami cek.", "--status", "in_progress", "--confirm")
		got, err := support.Ticket(ctx, "sup-user", first)
		if err != nil || got.Status != "in_progress" || got.StaffResponse == nil || *got.StaffResponse != "Terima kasih, kami cek." || got.RespondedAt == nil || !got.UpdatedAt.After(got.CreatedAt) {
			t.Fatalf("user view after reply: %+v %v", got, err)
		}
		if out, err := run(dbURL.String(), "reply", "--id", first, "--message", "jawaban kedua", "--confirm"); err == nil || !strings.Contains(out, "--replace") {
			t.Fatalf("silent overwrite must be refused: %v %s", err, out)
		}
		mustRun("reply", "--id", first, "--message", "Jawaban final.", "--replace", "--confirm")
		if _, err := run(dbURL.String(), "status", "--id", first, "--status", "open", "--confirm"); err == nil {
			t.Fatal("in_progress -> open must be refused")
		}
		mustRun("status", "--id", first, "--status", "resolved", "--confirm")
		got, _ = support.Ticket(ctx, "sup-user", first)
		if got.Status != "resolved" || got.StaffResponse == nil || *got.StaffResponse != "Jawaban final." {
			t.Fatalf("after resolve: %+v", got)
		}
		if out := mustRun("list", "--status", "resolved"); !strings.Contains(out, first) {
			t.Fatalf("filter: %s", out)
		}
		secret := url.URL{Scheme: "postgresql", Host: "127.0.0.1:1", Path: "/postgres", RawQuery: "sslmode=disable&connect_timeout=2", User: url.UserPassword("owner", "s3cr3t-pw")}
		out, err := run(secret.String(), "list")
		if err == nil || strings.Contains(out, "s3cr3t-pw") || strings.Contains(out, "127.0.0.1") {
			t.Fatalf("connection failure must not leak: %v %s", err, out)
		}
	})
}
