package httpdelivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type settingsMemory struct {
	mu       sync.Mutex
	status   map[string]string
	settings map[string]entity.AccountSettings
	reads    []string
	updates  []string
}

func newSettingsMemory() *settingsMemory {
	return &settingsMemory{
		status: map[string]string{"caller": "active", "other": "active"},
		settings: map[string]entity.AccountSettings{
			"caller": {NotificationEnabled: true, ShowOnLeaderboard: false, OnboardingCompleted: false},
			"other":  {NotificationEnabled: true, ShowOnLeaderboard: true, OnboardingCompleted: true},
		},
	}
}

func (m *settingsMemory) Settings(_ context.Context, uid string) (service.SettingsSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads = append(m.reads, uid)
	s, ok := m.settings[uid]
	if !ok {
		return service.SettingsSnapshot{}, service.ErrAccountNotFound
	}
	return service.SettingsSnapshot{Status: m.status[uid], Settings: s}, nil
}

func (m *settingsMemory) UpdateSettings(_ context.Context, uid string, in service.AccountSettingsUpdate) (entity.AccountSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, uid)
	s, ok := m.settings[uid]
	if !ok {
		return entity.AccountSettings{}, service.ErrAccountNotFound
	}
	if m.status[uid] != "active" {
		return entity.AccountSettings{}, service.ErrAccountForbidden
	}
	if in.NotificationEnabled != nil {
		s.NotificationEnabled = *in.NotificationEnabled
	}
	if in.OnboardingCompleted != nil {
		s.OnboardingCompleted = *in.OnboardingCompleted
	}
	m.settings[uid] = s
	return s, nil
}

func settingsRouter(store *settingsMemory, uid string) http.Handler {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: uid}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterSettingsRoutes(mux, verifier, handler.NewSettingsHandler(service.NewSettingsService(store)))
	return mux
}

type settingsBody struct {
	Data map[string]any `json:"data"`
}

func decodeSettings(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out settingsBody
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	return out.Data
}

func TestSettingsGetReturnsAllThreeFieldsForOwnUIDOnly(t *testing.T) {
	store := newSettingsMemory()
	w := accountRequest(settingsRouter(store, "caller"), "GET", "/api/v1/me/settings", "")
	if w.Code != 200 {
		t.Fatalf("get: %d %s", w.Code, w.Body)
	}
	data := decodeSettings(t, w.Body.Bytes())
	if len(data) != 3 || data["notification_enabled"] != true || data["show_on_leaderboard"] != false || data["onboarding_completed"] != false {
		t.Fatalf("unexpected settings: %v", data)
	}
	if len(store.reads) != 1 || store.reads[0] != "caller" {
		t.Fatalf("must read token uid only: %v", store.reads)
	}
}

func TestSettingsPatchTreatsOmittedAndFalseDifferently(t *testing.T) {
	store := newSettingsMemory()
	router := settingsRouter(store, "caller")

	// Explicit false must be applied, omitted field must be preserved.
	w := accountRequest(router, "PATCH", "/api/v1/me/settings", `{"notification_enabled":false}`)
	if w.Code != 200 {
		t.Fatalf("patch: %d %s", w.Code, w.Body)
	}
	data := decodeSettings(t, w.Body.Bytes())
	if data["notification_enabled"] != false || data["onboarding_completed"] != false || data["show_on_leaderboard"] != false {
		t.Fatalf("false not applied / others changed: %v", data)
	}

	w = accountRequest(router, "PATCH", "/api/v1/me/settings", `{"onboarding_completed":true}`)
	data = decodeSettings(t, w.Body.Bytes())
	if w.Code != 200 || data["notification_enabled"] != false || data["onboarding_completed"] != true {
		t.Fatalf("omitted field not preserved: %d %v", w.Code, data)
	}

	// Setting both, including explicit false for one.
	w = accountRequest(router, "PATCH", "/api/v1/me/settings", `{"notification_enabled":true,"onboarding_completed":false}`)
	data = decodeSettings(t, w.Body.Bytes())
	if w.Code != 200 || data["notification_enabled"] != true || data["onboarding_completed"] != false {
		t.Fatalf("both fields: %d %v", w.Code, data)
	}
	if got := store.settings["other"]; !got.ShowOnLeaderboard || !got.OnboardingCompleted || !got.NotificationEnabled {
		t.Fatalf("other user changed: %+v", got)
	}
	if store.settings["caller"].ShowOnLeaderboard {
		t.Fatal("show_on_leaderboard changed")
	}
}

func TestSettingsPatchRejectsInvalidBodiesWithoutWriting(t *testing.T) {
	store := newSettingsMemory()
	router := settingsRouter(store, "caller")
	bad := []string{
		``, `{}`, `null`, `[]`, `"x"`,
		`{"notification_enabled":null}`,
		`{"onboarding_completed":null}`,
		`{"notification_enabled":null,"onboarding_completed":true}`,
		`{"notification_enabled":"true"}`, `{"notification_enabled":1}`, `{"onboarding_completed":0}`, `{"onboarding_completed":[]}`,
		`{"show_on_leaderboard":false}`, `{"show_on_leaderboard":true,"notification_enabled":true}`,
		`{"unknown":true}`, `{"notification_enabled":true,"role":"mitra"}`, `{"uid":"other","notification_enabled":true}`,
		`{"notification_enabled":true} {}`, `{"notification_enabled":true}garbage`, `{"notification_enabled":true}{"onboarding_completed":true}`,
		`{"notification_enabled":`, `{"notification_enabled":true,}`,
	}
	for _, body := range bad {
		if w := accountRequest(router, "PATCH", "/api/v1/me/settings", body); w.Code != 400 {
			t.Errorf("body %q: expected 400, got %d %s", body, w.Code, w.Body)
		}
	}
	for _, q := range []string{"?uid=other", "?", "?x=1", "?%zz"} {
		if w := accountRequest(router, "PATCH", "/api/v1/me/settings"+q, `{"notification_enabled":false}`); w.Code != 400 {
			t.Errorf("patch query %q: %d", q, w.Code)
		}
		if w := accountRequest(router, "GET", "/api/v1/me/settings"+q, ""); w.Code != 400 {
			t.Errorf("get query %q: %d", q, w.Code)
		}
	}
	if w := accountRequest(router, "PATCH", "/api/v1/me/settings", `{"notification_enabled":false}`+strings.Repeat(" ", 17000)); w.Code != 413 {
		t.Errorf("oversized body: %d", w.Code)
	}
	if len(store.updates) != 0 {
		t.Fatalf("invalid requests reached the store: %v", store.updates)
	}
	if s := store.settings["caller"]; !s.NotificationEnabled || s.OnboardingCompleted {
		t.Fatalf("settings mutated by invalid request: %+v", s)
	}
}

func TestSettingsRequiresAuthAndJSONContentType(t *testing.T) {
	store := newSettingsMemory()
	router := settingsRouter(store, "caller")
	for _, method := range []string{"GET", "PATCH"} {
		req := httptest.NewRequest(method, "/api/v1/me/settings", strings.NewReader(`{"notification_enabled":false}`))
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if out.Code != 401 {
			t.Fatalf("%s without auth: %d", method, out.Code)
		}
	}
	req := httptest.NewRequest("PATCH", "/api/v1/me/settings", strings.NewReader(`{"notification_enabled":false}`))
	req.Header.Set("Authorization", "Bearer test")
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	if out.Code != 415 {
		t.Fatalf("missing content type: %d", out.Code)
	}
	req = httptest.NewRequest("PATCH", "/api/v1/me/settings", strings.NewReader(`{"notification_enabled":false}`))
	req.Header.Set("Authorization", "Bearer test")
	req.Header.Set("Content-Type", "text/plain")
	out = httptest.NewRecorder()
	router.ServeHTTP(out, req)
	if out.Code != 415 {
		t.Fatalf("text/plain: %d", out.Code)
	}
	if len(store.updates) != 0 || len(store.reads) != 0 {
		t.Fatal("unauthenticated or malformed request reached the store")
	}
}

func TestSettingsForbiddenAndMissingAccounts(t *testing.T) {
	for _, status := range []string{"suspended", "deleted"} {
		store := newSettingsMemory()
		store.status["caller"] = status
		before := store.settings["caller"]
		router := settingsRouter(store, "caller")
		if w := accountRequest(router, "GET", "/api/v1/me/settings", ""); w.Code != 403 {
			t.Fatalf("%s GET: %d", status, w.Code)
		}
		if w := accountRequest(router, "PATCH", "/api/v1/me/settings", `{"notification_enabled":false}`); w.Code != 403 {
			t.Fatalf("%s PATCH: %d", status, w.Code)
		}
		if store.settings["caller"] != before {
			t.Fatalf("%s account settings changed", status)
		}
	}
	router := settingsRouter(newSettingsMemory(), "ghost")
	if w := accountRequest(router, "GET", "/api/v1/me/settings", ""); w.Code != 404 {
		t.Fatalf("missing GET: %d", w.Code)
	}
	if w := accountRequest(router, "PATCH", "/api/v1/me/settings", `{"onboarding_completed":true}`); w.Code != 404 {
		t.Fatalf("missing PATCH: %d", w.Code)
	}
}

func TestSettingsPatchUsesTokenUIDNotBody(t *testing.T) {
	store := newSettingsMemory()
	router := settingsRouter(store, "caller")
	if w := accountRequest(router, "PATCH", "/api/v1/me/settings", `{"notification_enabled":false}`); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if len(store.updates) != 1 || store.updates[0] != "caller" {
		t.Fatalf("updates: %v", store.updates)
	}
}
