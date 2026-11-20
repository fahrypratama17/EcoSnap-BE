package httpdelivery_test

import (
	"context"
	"encoding/json"
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

// depositMemory mimics the store contract: one deposit per key, replay on an
// identical hash, conflict otherwise, and owner-only reads.
type depositMemory struct {
	byKey map[string]service.DepositRecord
	hash  map[string]string
}

func (m *depositMemory) CreateDeposit(_ context.Context, in service.NewDeposit, price service.DepositPricer) (service.DepositRecord, bool, error) {
	if in.CreatorUID != "mitra" {
		return service.DepositRecord{}, false, service.ErrDepositForbidden
	}
	if old, ok := m.byKey[in.IdempotencyKey]; ok {
		if m.hash[in.IdempotencyKey] != in.RequestHash {
			return old, false, service.ErrDepositConflict
		}
		return old, false, nil
	}
	sel := service.PointRateSelection{AsOf: time.Now(), Rates: map[string]service.PointTariff{}}
	for _, l := range in.Lines {
		sel.Rates[l.CategoryID] = service.PointTariff{CategoryID: l.CategoryID, Name: "Plastik", RateID: "50000000-0000-4000-8000-000000000001", RateHundredths: 10000, Accepted: true}
	}
	est, err := price(sel)
	if err != nil {
		return service.DepositRecord{}, false, err
	}
	rec := service.DepositRecord{ID: "60000000-0000-4000-8000-000000000001", Status: "created", TotalGrams: in.TotalGrams, TotalPoint: est.EstimatedPoints}
	m.byKey[in.IdempotencyKey], m.hash[in.IdempotencyKey] = rec, in.RequestHash
	return rec, true, nil
}
func (m *depositMemory) DepositForMitra(_ context.Context, uid, id string) (service.DepositRecord, error) {
	for _, rec := range m.byKey {
		if rec.ID == id && uid == "mitra" {
			return rec, nil
		}
	}
	return service.DepositRecord{}, service.ErrDepositNotFound
}

func depositRequest(router http.Handler, method, path, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}

func TestDepositHTTP(t *testing.T) {
	uid := "mitra"
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: uid}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterDepositRoutes(mux, verifier, handler.NewDepositHandler(service.NewDepositService(&depositMemory{byKey: map[string]service.DepositRecord{}, hash: map[string]string{}})))
	const id = "60000000-0000-4000-8000-000000000001"

	w := depositRequest(mux, "POST", "/api/v1/deposits", "key-00000001", estimateBody)
	var body struct {
		Data struct {
			ID          string `json:"id"`
			TotalPoints int64  `json:"total_points"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 201 || body.Data.TotalPoints != 1 || body.Data.ID != id {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	for _, tt := range []struct {
		name, method, path, key, body string
		want                          int
	}{
		{"replay", "POST", "/api/v1/deposits", "key-00000001", estimateBody, 200},
		{"changed payload", "POST", "/api/v1/deposits", "key-00000001", strings.Replace(estimateBody, "0.006", "0.007", 1), 409},
		{"missing key", "POST", "/api/v1/deposits", "", estimateBody, 400},
		{"short key", "POST", "/api/v1/deposits", "short", estimateBody, 400},
		{"client total", "POST", "/api/v1/deposits", "key-00000002", strings.Replace(estimateBody, `"items":`, `"total_points":999,"items":`, 1), 400},
		{"client partner", "POST", "/api/v1/deposits", "key-00000002", strings.Replace(estimateBody, `"items":`, `"partner_id":"20000000-0000-4000-8000-000000000002","items":`, 1), 400},
		{"invalid weight", "POST", "/api/v1/deposits", "key-00000002", strings.Replace(estimateBody, `"0.006"`, `"-1"`, 1), 400},
		{"detail", "GET", "/api/v1/deposits/" + id, "", "", 200},
		{"unknown detail", "GET", "/api/v1/deposits/60000000-0000-4000-8000-000000000009", "", "", 404},
		{"malformed detail", "GET", "/api/v1/deposits/not-a-uuid", "", "", 404},
	} {
		if w := depositRequest(mux, tt.method, tt.path, tt.key, tt.body); w.Code != tt.want {
			t.Fatalf("%s: %d %s", tt.name, w.Code, w.Body)
		}
	}
	uid = "user"
	if w := depositRequest(mux, "POST", "/api/v1/deposits", "key-00000003", estimateBody); w.Code != 403 {
		t.Fatalf("non-mitra: %d %s", w.Code, w.Body)
	}
	if w := depositRequest(mux, "GET", "/api/v1/deposits/"+id, "", ""); w.Code != 404 {
		t.Fatalf("other caller detail: %d %s", w.Code, w.Body)
	}
}
