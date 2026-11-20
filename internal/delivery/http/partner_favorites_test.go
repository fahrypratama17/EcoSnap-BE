package httpdelivery_test

import (
	"context"
	"encoding/json"
	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"net/http/httptest"
	"testing"
)

type favoriteMemory struct{ owners map[string]map[string]bool }

func (m *favoriteMemory) Set(_ context.Context, uid, id string, on bool) error {
	if id != "20000000-0000-4000-8000-000000000001" && on {
		return service.ErrPartnerNotFound
	}
	if m.owners[uid] == nil {
		m.owners[uid] = map[string]bool{}
	}
	m.owners[uid][id] = on
	return nil
}
func (m *favoriteMemory) List(_ context.Context, uid string, limit, offset int) ([]entity.FavoritePartner, error) {
	result := []entity.FavoritePartner{}
	for id, on := range m.owners[uid] {
		if on {
			result = append(result, entity.FavoritePartner{PartnerID: id})
		}
	}
	return result, nil
}
func TestFavoriteHTTPKeepsUsersSeparateAndSupportsRetry(t *testing.T) {
	memory := &favoriteMemory{owners: map[string]map[string]bool{}}
	verifier := verifierFunc(func(_ context.Context, token string) (*auth.Token, error) { return &auth.Token{UID: token}, nil })
	mux := httpdelivery.NewRouter(verifier)
	httpdelivery.RegisterFavoriteRoutes(mux, verifier, handler.NewFavoriteHandler(service.NewFavoriteService(memory)))
	request := func(method, path, uid string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if uid != "" {
			r.Header.Set("Authorization", "Bearer "+uid)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	const path = "/api/v1/me/favorites/20000000-0000-4000-8000-000000000001"
	if w := request("PUT", path, ""); w.Code != 401 {
		t.Fatalf("auth: %d", w.Code)
	}
	for i := 0; i < 2; i++ {
		if w := request("PUT", path, "alice"); w.Code != 204 {
			t.Fatalf("add: %d %s", w.Code, w.Body)
		}
	}
	readFavorites := func(uid string) []entity.FavoritePartner {
		w := request("GET", "/api/v1/me/favorites", uid)
		if w.Code != 200 {
			t.Fatalf("read favorites: %d %s", w.Code, w.Body)
		}
		var body struct {
			Data []entity.FavoritePartner `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Data
	}
	if got := readFavorites("bob"); len(got) != 0 {
		t.Fatalf("bob saw alice favorites: %+v", got)
	}
	if w := request("DELETE", path, "bob"); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if got := readFavorites("alice"); len(got) != 1 || got[0].PartnerID != "20000000-0000-4000-8000-000000000001" {
		t.Fatal("another user removed alice favorite")
	}
	for i := 0; i < 2; i++ {
		if w := request("DELETE", path, "alice"); w.Code != 204 {
			t.Fatal(w.Code)
		}
	}
	if w := request("PUT", "/api/v1/me/favorites/invalid", "alice"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := request("PUT", "/api/v1/me/favorites/20000000-0000-4000-8000-000000000002", "alice"); w.Code != 404 {
		t.Fatal(w.Code)
	}
	if w := request("GET", "/api/v1/me/favorites?user_uid=bob", "alice"); w.Code != 400 {
		t.Fatal(w.Code)
	}
}
