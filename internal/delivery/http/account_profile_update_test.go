package httpdelivery_test

import (
	"encoding/json"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileUpdateHTTP(t *testing.T) {
	store := &accountMemory{profile: &entity.AccountProfile{UID: "caller", FullName: "Nama Lama", Phone: "08123456789", Email: "original@example.test", Role: "user", Status: "active", Wallet: entity.WalletSummary{Balance: 250}}}
	router := accountRouter(store)
	w := accountRequest(router, "PATCH", "/api/v1/me", `{"full_name":"  Nama Baru  "}`)
	if w.Code != 200 {
		t.Fatalf("expected PATCH success, got %d %s", w.Code, w.Body)
	}
	var body struct {
		Data entity.AccountProfile `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || body.Data.FullName != "Nama Baru" || body.Data.Phone != "08123456789" || body.Data.Email != "original@example.test" || body.Data.Wallet.Balance != 250 {
		t.Fatalf("profile update: %d %s", w.Code, w.Body)
	}
	for _, bad := range []string{`{}`, `null`, `{"full_name":null}`, `{"full_name":null,"phone":"08123456789"}`, `{"phone":"abc"}`, `{"role":"mitra"}`, `{"email":"other@example.test"}`, `{"balance":10000}`, `{"full_name":"` + strings.Repeat("a", 151) + `"}`, `{"full_name":""}`, `{"phone":123}`, `{} {}`} {
		w = accountRequest(router, "PATCH", "/api/v1/me", bad)
		if w.Code != 400 {
			t.Fatalf("invalid body %s: %d", bad, w.Code)
		}
	}
	w = accountRequest(router, "PATCH", "/api/v1/me?uid=other", `{"phone":"08123456789"}`)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = accountRequest(router, "PATCH", "/api/v1/me", strings.Repeat(" ", 17000))
	if w.Code != 413 {
		t.Fatalf("large body: %d", w.Code)
	}
	req := httptest.NewRequest("PATCH", "/api/v1/me", strings.NewReader(`{"phone":"08123456789"}`))
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	if out.Code != 401 {
		t.Fatal(out.Code)
	}
	req.Header.Set("Authorization", "Bearer test")
	out = httptest.NewRecorder()
	router.ServeHTTP(out, req)
	if out.Code != 415 {
		t.Fatal(out.Code)
	}

	store.profile.Status = "suspended"
	w = accountRequest(router, "PATCH", "/api/v1/me", `{"full_name":"Dilarang"}`)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
