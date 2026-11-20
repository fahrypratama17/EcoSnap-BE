package httpdelivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type accountMemory struct {
	profile *entity.AccountProfile
	calls   int
}

func (m *accountMemory) Register(_ context.Context, uid, email string, in service.AccountRegistration) (entity.AccountProfile, bool, error) {
	m.calls++
	if m.profile != nil {
		return *m.profile, false, nil
	}
	p := entity.AccountProfile{UID: uid, Email: email, Role: in.Role, FullName: in.FullName, Phone: in.Phone, Status: "active"}
	if in.Role == "mitra" {
		p.Business = &entity.BusinessRegistration{Name: in.BusinessName, Category: in.BusinessCategory, Status: "pending"}
	}
	m.profile = &p
	return p, true, nil
}
func (m *accountMemory) Profile(_ context.Context, uid string) (entity.AccountProfile, error) {
	if m.profile == nil || uid != m.profile.UID {
		return entity.AccountProfile{}, service.ErrAccountNotFound
	}
	return *m.profile, nil
}
func accountRouter(store *accountMemory) http.Handler {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) {
		return &auth.Token{UID: "caller", Claims: map[string]any{"email": "verified-source@example.com"}}, nil
	})
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterAccountRoutes(mux, verifier, handler.NewAccountHandler(service.NewAccountService(store)))
	return mux
}
func accountRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w
}
func TestAccountRegistrationAndProfileHTTP(t *testing.T) {
	store := &accountMemory{}
	router := accountRouter(store)
	if w := accountRequest(router, "GET", "/api/v1/me", ""); w.Code != 404 {
		t.Fatalf("missing profile: %d", w.Code)
	}
	body := `{"role":"mitra","full_name":"  Mitra Satu  ","phone":"+628123456789","business_name":"Tunas","business_category":"Plastik"}`
	for _, status := range []int{201, 200} {
		w := accountRequest(router, "POST", "/api/v1/accounts", body)
		if w.Code != status {
			t.Fatalf("register %d: %s", w.Code, w.Body)
		}
	}
	w := accountRequest(router, "GET", "/api/v1/me?uid=other", "")
	var result struct {
		Data entity.AccountProfile `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || result.Data.UID != "caller" || result.Data.Email != "verified-source@example.com" || result.Data.FullName != "Mitra Satu" || result.Data.Business.Status != "pending" || result.Data.Wallet.Balance != 0 {
		t.Fatalf("wrong profile: %s", w.Body)
	}
	store.profile.Status = "suspended"
	if w := accountRequest(router, "GET", "/api/v1/me", ""); w.Code != 403 {
		t.Fatalf("suspended: %d", w.Code)
	}
}
func TestAccountRegistrationRejectsInvalidInput(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `{"role":"admin"}`, `{"password":"secret"}`, `{"uid":"other"}`,
		`{"role":"user","full_name":"User","phone":"abc"}`,
		`{"role":"mitra","full_name":"User","phone":"08123456789"}`,
		`{"role":"user","full_name":"User","phone":"08123456789","business_name":"Unexpected"}`,
		`{}` + `{}`, strings.Repeat(" ", 17000),
	} {
		store := &accountMemory{}
		w := accountRequest(accountRouter(store), "POST", "/api/v1/accounts", body)
		if w.Code != 400 && w.Code != 413 {
			t.Fatalf("invalid input accepted: %d", w.Code)
		}
		if store.calls != 0 {
			t.Fatal("invalid input reached storage")
		}
	}
}
func TestAccountRegistrationRequiresJSONAndAuthentication(t *testing.T) {
	router := accountRouter(&accountMemory{})
	for _, authenticated := range []bool{false, true} {
		r := httptest.NewRequest("POST", "/api/v1/accounts", strings.NewReader(`{}`))
		if authenticated {
			r.Header.Set("Authorization", "Bearer test")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		expected := 401
		if authenticated {
			expected = 415
		}
		if w.Code != expected {
			t.Fatalf("status %d, expected %d", w.Code, expected)
		}
	}
}

func (m *accountMemory) UpdateProfile(_ context.Context, uid string, in service.AccountProfileUpdate) (entity.AccountProfile, error) {
	if m.profile == nil || uid != m.profile.UID {
		return entity.AccountProfile{}, service.ErrAccountNotFound
	}
	if m.profile.Status != "active" {
		return entity.AccountProfile{}, service.ErrAccountForbidden
	}
	if in.FullName != nil {
		m.profile.FullName = *in.FullName
		m.profile.DisplayName = *in.FullName
	}
	if in.Phone != nil {
		m.profile.Phone = *in.Phone
	}
	return *m.profile, nil
}
