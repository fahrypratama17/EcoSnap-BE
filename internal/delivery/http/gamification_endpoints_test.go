package httpdelivery_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type gamificationMemory struct {
	claimed         bool
	claimCalls      int
	forbidden       bool
	visible         bool
	start, end, now time.Time
}

func (m *gamificationMemory) Missions(context.Context, string) (service.MissionSnapshot, error) {
	if m.forbidden {
		return service.MissionSnapshot{}, service.ErrGamificationForbidden
	}
	return service.MissionSnapshot{PeriodStart: m.start, PeriodEnd: m.end, Now: m.now, Deposits: 1, Claimed: map[string]time.Time{}}, nil
}
func (m *gamificationMemory) ClaimMission(_ context.Context, _ string, def service.MissionDefinition, start time.Time) (service.MissionSnapshot, bool, error) {
	m.claimCalls++
	if m.forbidden {
		return service.MissionSnapshot{}, false, service.ErrGamificationForbidden
	}
	if !start.Equal(m.start) {
		return service.MissionSnapshot{}, false, service.ErrMissionExpired
	}
	if m.claimed {
		return service.MissionSnapshot{PeriodStart: m.start, PeriodEnd: m.end, Now: m.now, Claimed: map[string]time.Time{def.ID: m.now}}, false, nil
	}
	m.claimed = true
	return service.MissionSnapshot{PeriodStart: m.start, PeriodEnd: m.end, Now: m.now, Deposits: 1, Claimed: map[string]time.Time{def.ID: m.now}, WeeklyXP: def.RewardXP, TotalXP: def.RewardXP}, true, nil
}
func (m *gamificationMemory) Badges(context.Context, string) (service.BadgeSnapshot, error) {
	return service.BadgeSnapshot{Now: m.now}, nil
}
func (m *gamificationMemory) Leaderboard(context.Context, string, int) (service.LeaderboardSnapshot, error) {
	return service.LeaderboardSnapshot{PeriodStart: m.start, PeriodEnd: m.end, Now: m.now}, nil
}
func (m *gamificationMemory) SetLeaderboardVisibility(_ context.Context, _ string, visible bool) error {
	if m.forbidden {
		return service.ErrGamificationForbidden
	}
	m.visible = visible
	return nil
}

func TestGamificationHTTP(t *testing.T) {
	start := time.Date(2026, 11, 16, 0, 0, 0, 0, service.WIB)
	store := &gamificationMemory{start: start, end: start.AddDate(0, 0, 7), now: start.Add(time.Hour)}
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "caller"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterGamificationRoutes(mux, verifier, handler.NewGamificationHandler(service.NewGamificationService(store)))
	request := func(method, path, body string, authenticated bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if authenticated {
			r.Header.Set("Authorization", "Bearer test")
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	if w := request("GET", "/api/v1/me/missions", "", false); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth: %d", w.Code)
	}
	body := `{"period_start":"` + start.Format(time.RFC3339Nano) + `"}`
	if w := request("POST", "/api/v1/me/missions/not-a-mission/claim", body, true); w.Code != http.StatusNotFound {
		t.Fatalf("unknown mission: %d %s", w.Code, w.Body)
	}
	if w := request("GET", "/api/v1/leaderboard?limit=0", "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit: %d %s", w.Code, w.Body)
	}
	if w := request("GET", "/api/v1/leaderboard?limit=x", "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit: %d %s", w.Code, w.Body)
	}
	if w := request("GET", "/api/v1/leaderboard?foo=bar", "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown query: %d %s", w.Code, w.Body)
	}
	if w := request("GET", "/api/v1/leaderboard?limit=1&limit=2", "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate limit: %d %s", w.Code, w.Body)
	}
	if w := request("GET", "/api/v1/leaderboard?limit=", "", true); w.Code != http.StatusBadRequest {
		t.Fatalf("empty limit: %d %s", w.Code, w.Body)
	}
	badQuery := httptest.NewRequest("GET", "/api/v1/leaderboard", nil)
	badQuery.URL.RawQuery = "limit=%zz"
	badQuery.Header.Set("Authorization", "Bearer test")
	badQueryResponse := httptest.NewRecorder()
	mux.ServeHTTP(badQueryResponse, badQuery)
	if badQueryResponse.Code != http.StatusBadRequest {
		t.Fatalf("malformed query: %d %s", badQueryResponse.Code, badQueryResponse.Body)
	}
	if w := request("POST", "/api/v1/me/missions/first_deposit/claim", `{}`, true); w.Code != http.StatusBadRequest {
		t.Fatalf("missing period: %d %s", w.Code, w.Body)
	}
	if w := request("PATCH", "/api/v1/me/leaderboard-visibility", `{"visible":false,"unexpected":true}`, true); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid visibility body: %d %s", w.Code, w.Body)
	}
	path := "/api/v1/me/missions/first_deposit/claim"
	if w := request("POST", path, body, true); w.Code != http.StatusCreated {
		t.Fatalf("claim: %d %s", w.Code, w.Body)
	}
	if w := request("POST", path, body, true); w.Code != http.StatusOK {
		t.Fatalf("replay: %d %s", w.Code, w.Body)
	}
	if store.claimCalls != 2 {
		t.Fatalf("claim calls %d", store.claimCalls)
	}
	if w := request("POST", path, `{"period_start":"2026-11-09T00:00:00+07:00"}`, true); w.Code != http.StatusGone {
		t.Fatalf("stale week: %d %s", w.Code, w.Body)
	}
	if w := request("PATCH", "/api/v1/me/leaderboard-visibility", `{"visible":false}`, true); w.Code != http.StatusOK || store.visible {
		t.Fatalf("visibility false: %d %s visible=%v", w.Code, w.Body, store.visible)
	}
	store.forbidden = true
	if w := request("GET", "/api/v1/me/missions", "", true); w.Code != http.StatusForbidden {
		t.Fatalf("role denial: %d %s", w.Code, w.Body)
	}
}
