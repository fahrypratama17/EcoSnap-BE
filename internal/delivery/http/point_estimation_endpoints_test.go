package httpdelivery_test

import (
	"context"
	"encoding/json"
	"errors"
	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type estimateRates struct{ err error }

func (s estimateRates) SelectRates(_ context.Context, _ *string, ids []string) (service.PointRateSelection, error) {
	out := service.PointRateSelection{AsOf: time.Now(), Rates: map[string]service.PointTariff{}}
	for _, id := range ids {
		out.Rates[id] = service.PointTariff{CategoryID: id, Name: "Plastik", RateID: "50000000-0000-4000-8000-000000000001", RateHundredths: 10000, Accepted: true, IsDemo: true}
	}
	return out, s.err
}
func estimateRouter(err error) http.Handler {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "caller"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterPointEstimateRoutes(mux, verifier, handler.NewPointEstimationHandler(service.NewPointEstimationService(estimateRates{err})))
	return mux
}

const estimateBody = `{"items":[{"category_id":"10000000-0000-4000-8000-000000000001","weight_kg":"0.006"},{"category_id":"10000000-0000-4000-8000-000000000002","weight_kg":"0.006"}]}`

func TestPointEstimateHTTP(t *testing.T) {
	w := accountRequest(estimateRouter(nil), "POST", "/api/v1/point-estimates", estimateBody)
	var body struct {
		Data entity.PointEstimate `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != 200 || body.Data.EstimatedPoints != 1 || !body.Data.UsesDemoRates || len(body.Data.Items) != 2 {
		t.Fatalf("response: %d %s %v", w.Code, w.Body, err)
	}
}
func TestPointEstimateHTTPRejectsInvalidRequests(t *testing.T) {
	for _, tt := range []struct {
		name, body, query string
		status            int
	}{
		{"numeric weight", strings.Replace(estimateBody, `"0.006"`, `0.006`, 1), "", 400},
		{"client rate", strings.Replace(estimateBody, `"items":`, `"points_per_kg":"999","items":`, 1), "", 400},
		{"trailing JSON", estimateBody + ` {}`, "", 400},
		{"null body", `null`, "", 400},
		{"duplicate", strings.ReplaceAll(estimateBody, "000000000002", "000000000001"), "", 400},
		{"query override", estimateBody, "?uid=other", 400},
		{"oversized", `{"items":[{"weight_kg":"` + strings.Repeat("x", 17000) + `"}]}`, "", 413},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := accountRequest(estimateRouter(nil), "POST", "/api/v1/point-estimates"+tt.query, tt.body)
			if w.Code != tt.status {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
	for _, tt := range []struct {
		auth, contentType string
		want              int
	}{{"", "application/json", 401}, {"Bearer test", "text/plain", 415}} {
		r := httptest.NewRequest("POST", "/api/v1/point-estimates", strings.NewReader(estimateBody))
		r.Header.Set("Authorization", tt.auth)
		r.Header.Set("Content-Type", tt.contentType)
		w := httptest.NewRecorder()
		estimateRouter(nil).ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Fatalf("headers: %d %s", w.Code, w.Body)
		}
	}
}
func TestPointEstimateHTTPMapsErrors(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want int
		code string
	}{
		{service.ErrPointRateUnavailable, 409, "point_rate_unavailable"},
		{service.ErrWasteCategoryNotFound, 404, "waste_category_not_found"},
		{service.ErrPartnerNotFound, 404, "partner_not_found"},
		{service.ErrWasteNotAccepted, 400, "waste_not_accepted"},
		{context.DeadlineExceeded, 504, "request_timeout"},
		{errors.New("private database details"), 500, "internal_error"},
	} {
		w := accountRequest(estimateRouter(tt.err), "POST", "/api/v1/point-estimates", estimateBody)
		if w.Code != tt.want || !strings.Contains(w.Body.String(), tt.code) || strings.Contains(w.Body.String(), "private database") {
			t.Fatalf("error response: %d %s", w.Code, w.Body)
		}
	}
}
