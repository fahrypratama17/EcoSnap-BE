package httpdelivery

import (
	"net/http"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
)

func NewRouter(verifier middleware.TokenVerifier) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.Handle("GET /api/v1/auth/me", middleware.RequireAuth(verifier)(http.HandlerFunc(handler.AuthMe)))
	return mux
}

func RegisterAccountRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, accounts *handler.AccountHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("POST /api/v1/accounts", protect(http.HandlerFunc(accounts.Register)))
	mux.Handle("GET /api/v1/me", protect(http.HandlerFunc(accounts.Profile)))
	mux.Handle("PATCH /api/v1/me", protect(http.HandlerFunc(accounts.UpdateProfile)))
}

func RegisterPartnerRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, partners *handler.PartnerHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/partners", protect(http.HandlerFunc(partners.List)))
	mux.Handle("GET /api/v1/partners/{partnerID}", protect(http.HandlerFunc(partners.Detail)))
	mux.Handle("GET /api/v1/waste-categories", protect(http.HandlerFunc(partners.Categories)))
}

func RegisterFavoriteRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, favorites *handler.FavoriteHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/me/favorites", protect(http.HandlerFunc(favorites.List)))
	mux.Handle("PUT /api/v1/me/favorites/{partnerID}", protect(http.HandlerFunc(favorites.Set)))
	mux.Handle("DELETE /api/v1/me/favorites/{partnerID}", protect(http.HandlerFunc(favorites.Set)))
}
