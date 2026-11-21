package httpdelivery_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type activityMemory struct {
	uid      string
	from, to time.Time
	err      error
	rec      service.ActivityReportRecord
	calls    int
}

func (m *activityMemory) ActivityReport(_ context.Context, uid string, from, to time.Time) (service.ActivityReportRecord, error) {
	m.calls++
	m.uid, m.from, m.to = uid, from, to
	return m.rec, m.err
}

func TestActivityReportHTTP(t *testing.T) {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "token-uid"}, nil })
	store := &activityMemory{rec: service.ActivityReportRecord{Role: "user", FullName: "Ünal Çelik", Days: []service.ActivityDayRecord{{Date: "2026-11-02", Claimed: 1, Grams: 1500, Points: 190}}}}
	now := time.Date(2026, 11, 21, 3, 0, 0, 0, time.UTC)
	reports := service.NewActivityReportService(store).WithClock(func() time.Time { return now })
	mux := httpdelivery.NewRouter(verifier)
	h := handler.NewActivityReportHandler(reports)
	protect := middleware.RequireAuth(verifier)
	// Same two routes the router registers for production.
	mux.Handle("GET /api/v1/me/activity-report", protect(http.HandlerFunc(h.Report)))
	mux.Handle("GET /api/v1/me/activity-report.pdf", protect(http.HandlerFunc(h.PDF)))

	const q = "?from=2026-11-01T00:00:00%2B07:00&to=2026-11-04T00:00:00%2B07:00"
	t.Run("json_report", func(t *testing.T) {
		w := accountRequest(mux, "GET", "/api/v1/me/activity-report"+q, "")
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%d %v", w.Code, w.Header())
		}
		var b struct {
			Data entity.ActivityReport `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
			t.Fatal(err)
		}
		if store.uid != "token-uid" || b.Data.Totals.ClaimedDeposits != 1 || b.Data.Totals.TotalWeightKG != "1.500" || len(b.Data.Daily) != 3 || *b.Data.Totals.PointsEarned != 190 || b.Data.Account.FullName != "Ünal Çelik" {
			t.Fatalf("uid=%s %+v", store.uid, b.Data)
		}
		if !store.from.Equal(time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)) || !store.to.Equal(time.Date(2026, 11, 3, 17, 0, 0, 0, time.UTC)) {
			t.Fatalf("range %v %v", store.from, store.to)
		}
		var raw map[string]map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &raw)
		if _, ok := raw["data"]["totals"].(map[string]any)["points_credited"]; ok {
			t.Fatal("user report must not expose points_credited")
		}
	})
	t.Run("pdf_download", func(t *testing.T) {
		w := accountRequest(mux, "GET", "/api/v1/me/activity-report.pdf"+q, "")
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		hd := w.Header()
		if hd.Get("Content-Type") != "application/pdf" || hd.Get("X-Content-Type-Options") != "nosniff" || hd.Get("Cache-Control") != "no-store" ||
			hd.Get("Content-Disposition") != `attachment; filename="ecosnap-activity-report-20261101-20261103.pdf"` || hd.Get("Content-Length") != strconv.Itoa(w.Body.Len()) {
			t.Fatalf("headers: %v", hd)
		}
		if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF-")) || !bytes.Contains(w.Body.Bytes(), []byte("%%EOF")) {
			t.Fatal("body is not a PDF")
		}
	})
	t.Run("default_range", func(t *testing.T) {
		if w := accountRequest(mux, "GET", "/api/v1/me/activity-report", ""); w.Code != 200 || !store.from.Equal(time.Date(2026, 10, 31, 17, 0, 0, 0, time.UTC)) || !store.to.Equal(now) {
			t.Fatalf("%d %v %v", w.Code, store.from, store.to)
		}
	})
	t.Run("invalid_queries_never_reach_the_store", func(t *testing.T) {
		store.calls = 0
		for _, path := range []string{
			"?from=2026-11-01T00:00:00%2B07:00",                       // only one bound
			"?to=2026-11-04T00:00:00Z",                                // only one bound
			"?from=2026-11-01&to=2026-11-04",                          // no time zone
			"?from=2026-11-01T00:00:00&to=2026-11-04T00:00:00",        // no zone offset
			"?from=2026-11-01T00:00:00+07:00&to=2026-11-04T00:00:00Z", // raw '+' decodes to a space
			"?from=2026-11-04T00:00:00Z&to=2026-11-01T00:00:00Z",      // reversed
			"?from=2026-11-01T00:00:00Z&to=2026-11-01T00:00:00Z",      // empty
			"?from=2026-01-01T00:00:00Z&to=2027-01-03T00:00:00Z",      // more than 366 days
			"?from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z&from=2026-11-01T00:00:00Z",
			"?from=2026-11-01T00:00:00Z&to=2026-11-02T00:00:00Z&limit=5",
			"?uid=someone-else",
			"?from=%zz",
		} {
			for _, p := range []string{"/api/v1/me/activity-report", "/api/v1/me/activity-report.pdf"} {
				w := accountRequest(mux, "GET", p+path, "")
				var b struct {
					Error struct{ Code string } `json:"error"`
				}
				_ = json.Unmarshal(w.Body.Bytes(), &b)
				if w.Code != 400 || b.Error.Code != "invalid_query" {
					t.Fatalf("%s%s: %d %s", p, path, w.Code, w.Body)
				}
			}
		}
		if store.calls != 0 {
			t.Fatal("store reached")
		}
	})
	t.Run("span_of_exactly_366_days_is_allowed", func(t *testing.T) {
		if w := accountRequest(mux, "GET", "/api/v1/me/activity-report?from=2026-01-01T00:00:00Z&to=2027-01-02T00:00:00Z", ""); w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
	})
	t.Run("requires_bearer_token", func(t *testing.T) {
		for _, p := range []string{"/api/v1/me/activity-report", "/api/v1/me/activity-report.pdf"} {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
			if w.Code != 401 {
				t.Fatalf("%s: %d", p, w.Code)
			}
		}
	})
	t.Run("account_errors", func(t *testing.T) {
		for err, want := range map[error][2]string{
			service.ErrAccountNotFound:  {"404", "account_not_found"},
			service.ErrAccountForbidden: {"403", "account_forbidden"},
			service.ErrDepositForbidden: {"403", "deposit_forbidden"},
			context.DeadlineExceeded:    {"504", "request_timeout"},
		} {
			store.err = err
			for _, p := range []string{"/api/v1/me/activity-report", "/api/v1/me/activity-report.pdf"} {
				w := accountRequest(mux, "GET", p+q, "")
				var b struct {
					Error struct{ Code string } `json:"error"`
				}
				_ = json.Unmarshal(w.Body.Bytes(), &b)
				if strconv.Itoa(w.Code) != want[0] || b.Error.Code != want[1] {
					t.Fatalf("%v %s: %d %s", err, p, w.Code, w.Body)
				}
				if w.Header().Get("Content-Type") == "application/pdf" {
					t.Fatal("error response must not be a PDF")
				}
			}
		}
		store.err = context.Canceled
		if w := accountRequest(mux, "GET", "/api/v1/me/activity-report"+q, ""); w.Code != 500 {
			t.Fatalf("internal: %d", w.Code)
		}
		store.err = nil
	})
}
