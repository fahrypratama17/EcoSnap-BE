package httpdelivery

import (
	"net/http"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
)

func RegisterSupportRoutes(mux *http.ServeMux, verifier middleware.TokenVerifier, support *handler.SupportHandler) {
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/help/faqs", protect(http.HandlerFunc(support.FAQs)))
	mux.Handle("POST /api/v1/me/support-tickets", protect(http.HandlerFunc(support.Create)))
	mux.Handle("GET /api/v1/me/support-tickets", protect(http.HandlerFunc(support.List)))
	mux.Handle("GET /api/v1/me/support-tickets/{ticketID}", protect(http.HandlerFunc(support.Detail)))
}
