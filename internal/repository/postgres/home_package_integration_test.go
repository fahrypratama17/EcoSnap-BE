package postgres_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/bootstrap"
	"github.com/fahrypratama17/EcoSnap-BE/internal/config"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This verifier supplies synthetic identities only. The HTTP, business logic,
// queries and isolated PostgreSQL database are real; Firebase is not exercised.
type homePackageVerifier struct{}

func (homePackageVerifier) VerifyIDToken(_ context.Context, token string) (*auth.Token, error) {
	return &auth.Token{UID: token}, nil
}

// Decode wire names independently from entity.UserHome: this is the shape the
// Android mapper consumes, not a serialization round-trip of the producer type.
type homePackageWire struct {
	UserName   string    `json:"user_name"`
	ServerTime time.Time `json:"server_time"`
	Month      struct {
		From     time.Time `json:"from"`
		To       time.Time `json:"to"`
		Timezone string    `json:"timezone"`
	} `json:"month"`
	MonthlyDeposits int64                      `json:"monthly_deposits"`
	DepositsClaimed int64                      `json:"deposits_claimed"`
	TotalWeightKG   string                     `json:"total_weight_kg"`
	Wallet          entity.Wallet              `json:"wallet"`
	RecentActivity  []entity.PointHistoryEntry `json:"recent_activity"`
	Missions        *entity.MissionList        `json:"missions"`
	Partner         *entity.PartnerDetail      `json:"partner"`
	Article         *entity.ArticleSummary     `json:"article"`
	UnreadCount     int64                      `json:"unread_count"`
}

func TestHomePackageIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	if _, err := pool.Exec(ctx, `UPDATE public.accounts SET full_name='Siti Home' WHERE firebase_uid='plain-user'`); err != nil {
		t.Fatal(err)
	}

	// A single connection must suffice even when every card and history are
	// populated. Calling nested repositories that acquire again would time out.
	cfg := pool.Config()
	cfg.MaxConns, cfg.MinConns = 1, 0
	homePool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer homePool.Close()
	server := httptest.NewServer(bootstrap.NewServer(config.Config{}, homePackageVerifier{}, homePool).Handler)
	defer server.Close()
	client := &http.Client{Timeout: 10 * time.Second}

	request := func(t *testing.T, method, path, uid, body string, want int, out any) []byte {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if uid != "" {
			req.Header.Set("Authorization", "Bearer "+uid)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		content, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != want {
			t.Fatalf("%s %s: status=%d want=%d err=%v body=%s", method, path, resp.StatusCode, want, err, content)
		}
		if out != nil {
			if err := json.Unmarshal(content, out); err != nil {
				t.Fatalf("invalid JSON: %v body=%s", err, content)
			}
		}
		return content
	}
	home := func(t *testing.T, uid string) homePackageWire {
		t.Helper()
		var response struct {
			Data homePackageWire `json:"data"`
		}
		request(t, "GET", "/api/v1/me/home", uid, "", http.StatusOK, &response)
		return response.Data
	}
	initial := home(t, "plain-user")
	localStart, localNow := initial.Month.From.In(service.WIB), initial.ServerTime.In(service.WIB)
	hour, minute, second := localStart.Clock()
	if initial.ServerTime.IsZero() || localStart.Day() != 1 || hour != 0 || minute != 0 || second != 0 || localStart.Nanosecond() != 0 || localStart.Year() != localNow.Year() || localStart.Month() != localNow.Month() {
		t.Fatalf("month must start at current WIB month midnight: %+v server=%s", initial.Month, initial.ServerTime)
	}

	// Historical zero-point rows use the existing immutable-claim fixture;
	// wallet funding and recent activity below use a genuine business claim.
	// The current-month boundary is 17:00 UTC on the previous UTC date, not
	// midnight UTC. Each fixture claim contains two items but counts once.
	monthStart := initial.Month.From
	insertGamificationClaim(ctx, t, pool, 2001, "plain-user", 1000, monthStart.Add(-time.Microsecond))
	insertGamificationClaim(ctx, t, pool, 2002, "plain-user", 1000, monthStart)
	insertGamificationClaim(ctx, t, pool, 2003, "user-2", 1000, monthStart)
	insertGamificationClaim(ctx, t, pool, 2004, "plain-user", 1000, monthStart.In(service.WIB).AddDate(0, 1, 0))
	deposits := service.NewDepositService(postgres.NewDepositRepository(pool))
	qr := service.NewDepositQRService(postgres.NewDepositRepository(pool))
	d, _, err := deposits.Create(ctx, "owner-a", "home-package-real-claim", entity.DepositCreateRequest{
		Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "1.500"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	code, err := qr.Issue(ctx, "owner-a", d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := qr.Claim(ctx, "plain-user", code.Token); err != nil {
		t.Fatal(err)
	}
	// Count by claim time, even when the deposit was created years earlier.
	if _, err := pool.Exec(ctx, `UPDATE public.deposits SET created_at='2020-01-01' WHERE id=$1::uuid`, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.articles(id,title,summary,category,reading_minutes,is_published,published_at) VALUES
('e0000000-0000-4000-8000-000000000001','Published one','First','GUIDE',2,true,now()-interval '1 day'),
('e0000000-0000-4000-8000-000000000002','Published two','Latest stable ID','GUIDE',3,true,now()-interval '1 day'),
('e0000000-0000-4000-8000-000000000003','Future article','Hidden','GUIDE',4,true,now()+interval '1 day'),
('e0000000-0000-4000-8000-000000000004','Draft article','Hidden','GUIDE',5,false,now())`); err != nil {
		t.Fatal(err)
	}

	t.Run("monthly_claims_wallet_and_clickable_cards", func(t *testing.T) {
		var response struct {
			Data homePackageWire `json:"data"`
		}
		body := request(t, "GET", "/api/v1/me/home", "plain-user", "", http.StatusOK, &response)
		h := response.Data
		if h.UserName != "Siti Home" || h.Wallet.Balance != 225 || h.MonthlyDeposits != 2 || h.DepositsClaimed != 4 || h.TotalWeightKG != "4.500" {
			t.Fatalf("wrong user/month/all-time data: %+v", h)
		}
		if h.Month.Timezone != "Asia/Jakarta" || h.Month.From.Format(time.RFC3339) != monthStart.Format(time.RFC3339) || !h.Month.To.Equal(h.ServerTime) {
			t.Fatalf("month/time contract: %+v server=%s", h.Month, h.ServerTime)
		}
		if h.Partner == nil || h.Partner.ID != "20000000-0000-4000-8000-000000000002" || h.Partner.Name != "Lain" {
			t.Fatalf("stable global partner suggestion: %+v", h.Partner)
		}
		if h.Article == nil || h.Article.ID != "e0000000-0000-4000-8000-000000000002" || h.Article.Title != "Published two" || h.Article.ReadingMinutes != 3 {
			t.Fatalf("latest visible article: %+v", h.Article)
		}
		if len(h.RecentActivity) != 1 || h.RecentActivity[0].Amount != 225 || h.RecentActivity[0].Deposit == nil || h.RecentActivity[0].Deposit.ID != d.ID || h.UnreadCount != 4 {
			t.Fatalf("receipt/unread mismatch: %+v unread=%d", h.RecentActivity, h.UnreadCount)
		}
		if bytes.Contains(body, []byte(code.Token)) || bytes.Contains(body, []byte("user-2@example.test")) {
			t.Fatal("home leaks private data")
		}
		t.Logf("Home HTTP smoke payload: %s", body)
	})

	t.Run("mission_card_matches_catalog_and_claim_refresh", func(t *testing.T) {
		h := home(t, "plain-user")
		var response struct {
			Data entity.MissionList `json:"data"`
		}
		request(t, "GET", "/api/v1/me/missions", "plain-user", "", http.StatusOK, &response)
		if h.Missions == nil || !reflect.DeepEqual(h.Missions.Missions, response.Data.Missions) || !h.Missions.PeriodStart.Equal(response.Data.PeriodStart) || !h.Missions.PeriodEnd.Equal(response.Data.PeriodEnd) || !h.Missions.ServerTime.Equal(h.ServerTime) {
			t.Fatalf("mission mismatch: home=%+v standalone=%+v", h.Missions, response.Data)
		}
		body, err := json.Marshal(map[string]time.Time{"period_start": h.Missions.PeriodStart})
		if err != nil {
			t.Fatal(err)
		}
		request(t, "POST", "/api/v1/me/missions/first_deposit/claim", "plain-user", string(body), http.StatusCreated, nil)
		request(t, "POST", "/api/v1/me/missions/first_deposit/claim", "plain-user", string(body), http.StatusOK, nil)
		after := home(t, "plain-user")
		if after.Missions == nil || after.Missions.WeeklyXP != 50 || after.Missions.TotalXP != 50 || after.Wallet.Balance != 225 || after.MonthlyDeposits != 2 || after.UnreadCount != 5 {
			t.Fatalf("refresh after claim/replay: %+v", after)
		}
		foundFirst := false
		for _, mission := range after.Missions.Missions {
			if mission.ID == "first_deposit" {
				foundFirst = true
				if mission.Status != entity.MissionClaimed || mission.ClaimedAt == nil || mission.RewardXP != 50 || mission.Unit != "deposit" {
					t.Fatalf("claim state did not refresh: %+v", mission)
				}
			}
		}
		if !foundFirst {
			t.Fatal("claimed first_deposit disappeared from Home")
		}
	})

	t.Run("caller_isolation_and_unread_read_refresh", func(t *testing.T) {
		h := home(t, "user-2")
		if h.UserName != "User" || h.Wallet.Balance != 0 || h.MonthlyDeposits != 1 || h.DepositsClaimed != 1 || h.UnreadCount != 1 || len(h.RecentActivity) != 0 {
			t.Fatalf("caller isolation: %+v", h)
		}
		var inbox struct {
			Data []entity.Notification `json:"data"`
		}
		request(t, "GET", "/api/v1/me/notifications", "user-2", "", http.StatusOK, &inbox)
		if len(inbox.Data) != 1 {
			t.Fatalf("user-2 inbox: %+v", inbox.Data)
		}
		path := "/api/v1/me/notifications/" + inbox.Data[0].ID + "/read"
		request(t, "PATCH", path, "plain-user", "", http.StatusNotFound, nil)
		request(t, "PATCH", path, "user-2", "", http.StatusOK, nil)
		request(t, "PATCH", path, "user-2", "", http.StatusOK, nil)
		if h := home(t, "user-2"); h.UnreadCount != 0 {
			t.Fatalf("read did not refresh home: %d", h.UnreadCount)
		}
		if h := home(t, "plain-user"); h.UnreadCount != 5 {
			t.Fatalf("foreign read changed owner home: %d", h.UnreadCount)
		}
		request(t, "PATCH", "/api/v1/me/settings", "plain-user", `{"notification_enabled":false}`, http.StatusOK, nil)
		if h := home(t, "plain-user"); h.UnreadCount != 5 {
			t.Fatalf("preference hid old unread: %d", h.UnreadCount)
		}
	})

	t.Run("partner_visibility_and_missing_cards_are_null", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='owner-b'`); err != nil {
			t.Fatal(err)
		}
		if h := home(t, "plain-user"); h.Partner == nil || h.Partner.ID != "20000000-0000-4000-8000-000000000001" || len(h.Partner.AcceptedMaterials) != 2 {
			t.Fatalf("inactive owner visible / active materials lost: %+v", h.Partner)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.partners SET status='suspended' WHERE mitra_uid='owner-a'; UPDATE public.articles SET is_published=false`); err != nil {
			t.Fatal(err)
		}
		var raw struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		request(t, "GET", "/api/v1/me/home", "user-3", "", http.StatusOK, &raw)
		if string(raw.Data["partner"]) != "null" || string(raw.Data["article"]) != "null" || string(raw.Data["recent_activity"]) != "[]" {
			t.Fatalf("empty cards/array: %s %s %s", raw.Data["partner"], raw.Data["article"], raw.Data["recent_activity"])
		}
		h := home(t, "user-3")
		if h.Wallet.Balance != 0 || h.MonthlyDeposits != 0 || h.DepositsClaimed != 0 || h.UnreadCount != 0 || h.Missions == nil {
			t.Fatalf("fresh account must have real zero state: %+v", h)
		}
	})

	t.Run("mitra_access_preserved_without_user_missions", func(t *testing.T) {
		h := home(t, "owner-a")
		if h.Missions != nil || h.UserName != "Owner" || h.MonthlyDeposits != 0 {
			t.Fatalf("mitra compatibility: %+v", h)
		}
		var response struct {
			Data entity.PartnerHome `json:"data"`
		}
		request(t, "GET", "/api/v1/partner/home", "owner-a", "", http.StatusOK, &response)
		if response.Data.PartnerStatus != "suspended" {
			t.Fatalf("partner read-only flow broken: %+v", response.Data)
		}
	})

	t.Run("auth_and_account_failures_do_not_return_cards", func(t *testing.T) {
		request(t, "GET", "/api/v1/me/home", "", "", http.StatusUnauthorized, nil)
		request(t, "GET", "/api/v1/me/home", "missing-account", "", http.StatusNotFound, nil)
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='user-3'`); err != nil {
			t.Fatal(err)
		}
		body := request(t, "GET", "/api/v1/me/home", "user-3", "", http.StatusForbidden, nil)
		if bytes.Contains(body, []byte(`"data"`)) {
			t.Fatalf("forbidden home returned data: %s", body)
		}
	})

	if acquired := homePool.Stat().AcquiredConns(); acquired != 0 {
		t.Fatalf("home connection retained: %d", acquired)
	}
}
