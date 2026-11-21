package httpdelivery_test

import (
	"context"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type articleMemory struct{ filter service.ArticleFilter }

func (m *articleMemory) Articles(_ context.Context, f service.ArticleFilter, _ int, _ *service.Cursor) ([]entity.ArticleSummary, error) {
	m.filter = f
	return []entity.ArticleSummary{}, nil
}
func (m *articleMemory) Article(context.Context, string) (entity.Article, error) {
	return entity.Article{}, service.ErrArticleNotFound
}

func TestArticleHTTP(t *testing.T) {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "u"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	store := &articleMemory{}
	httpdelivery.RegisterArticleRoutes(mux, verifier, handler.NewArticleHandler(service.NewArticleService(store)))
	for path, want := range map[string]int{
		"/api/v1/articles":                                      200,
		"/api/v1/articles?q=%20&limit=5":                        200,
		"/api/v1/articles?category=blog":                        400,
		"/api/v1/articles?featured=maybe":                       400,
		"/api/v1/articles?q=a&q=b":                              400,
		"/api/v1/articles?status=draft":                         400,
		"/api/v1/articles/not-a-uuid":                           404,
		"/api/v1/articles/90000000-0000-4000-8000-000000000001": 404,
	} {
		if w := accountRequest(mux, "GET", path, ""); w.Code != want {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	if store.filter.Search != nil {
		t.Fatal("blank search should not filter")
	}
	if w := accountRequest(mux, "GET", "/api/v1/articles?q=+Kompos+&category=GUIDE&featured=true", ""); w.Code != 200 || store.filter.Search == nil || *store.filter.Search != "Kompos" || *store.filter.Category != "GUIDE" || !store.filter.Featured {
		t.Fatalf("filter: %d %+v", w.Code, store.filter)
	}
}
