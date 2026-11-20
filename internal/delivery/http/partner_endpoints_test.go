package httpdelivery_test

import (
	"context"
	"encoding/json"
	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"net/http"
	"net/http/httptest"
	"testing"
)

type partnerFixture struct{}

func (partnerFixture) List(context.Context, service.PartnerFilter) ([]entity.PartnerDetail, error) {
	return []entity.PartnerDetail{{ID: "first", Name: "Tunas"}, {ID: "second", Name: "Lestari"}, {ID: "third", Name: "Hijau"}}, nil
}
func (partnerFixture) Detail(_ context.Context, id string) (entity.PartnerDetail, error) {
	if id == "20000000-0000-4000-8000-000000000001" {
		return entity.PartnerDetail{ID: id, Name: "Tunas", Location: entity.PartnerLocation{Latitude: -7.9775, Longitude: 112.6304}, AcceptedMaterials: []entity.WasteCategory{}, Hours: []entity.PartnerHour{}}, nil
	}
	return entity.PartnerDetail{}, service.ErrPartnerNotFound
}
func (partnerFixture) Categories(context.Context) ([]entity.WasteCategory, error) {
	return []entity.WasteCategory{{Name: "Plastik", Slug: "plastic", Code: "PLASTIC"}}, nil
}
func partnerRouter() http.Handler {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "caller"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterPartnerRoutes(mux, verifier, handler.NewPartnerHandler(service.NewPartnerService(partnerFixture{})))
	return mux
}
func TestPartnerListPaginationAndDetail(t *testing.T) {
	router := partnerRouter()
	w := accountRequest(router, "GET", "/api/v1/partners?limit=2", "")
	var body struct {
		Data       []entity.PartnerDetail `json:"data"`
		Pagination struct {
			Limit   int  `json:"limit"`
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(body.Data) != 2 || !body.Pagination.HasMore || body.Pagination.Limit != 2 {
		t.Fatalf("wrong page: %d %s", w.Code, w.Body)
	}
	w = accountRequest(router, "GET", "/api/v1/partners/20000000-0000-4000-8000-000000000001", "")
	var detail struct {
		Data entity.PartnerDetail `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || detail.Data.Name != "Tunas" || detail.Data.Location.Latitude != -7.9775 {
		t.Fatalf("wrong detail: %s", w.Body)
	}
}
func TestPartnerRejectsInvalidFiltersAndIDs(t *testing.T) {
	for _, path := range []string{"/api/v1/partners?limit=0", "/api/v1/partners?limit=101", "/api/v1/partners?limit=x", "/api/v1/partners?offset=-1", "/api/v1/partners?offset=10001", "/api/v1/partners?offset=x", "/api/v1/partners?limit=2&limit=3", "/api/v1/partners?unexpected=true", "/api/v1/partners?material=plastic%27", "/api/v1/partners/not-a-uuid"} {
		w := accountRequest(partnerRouter(), "GET", path, "")
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}
func TestPartnerMissingAndCategories(t *testing.T) {
	w := accountRequest(partnerRouter(), "GET", "/api/v1/partners/20000000-0000-4000-8000-000000000002", "")
	if w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}
	w = accountRequest(partnerRouter(), "GET", "/api/v1/waste-categories", "")
	var body struct {
		Data []entity.WasteCategory `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(body.Data) != 1 || body.Data[0].Code != "PLASTIC" {
		t.Fatalf("categories: %s", w.Body)
	}
}
func TestPartnerRoutesRequireAuthentication(t *testing.T) {
	for _, path := range []string{"/api/v1/partners", "/api/v1/partners/20000000-0000-4000-8000-000000000001", "/api/v1/waste-categories"} {
		w := httptest.NewRecorder()
		partnerRouter().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 401 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
}
