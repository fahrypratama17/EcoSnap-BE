package handler

import (
	"net/http"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
)

// AuthMe checks Firebase authentication without creating an application account.
func AuthMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.IdentityFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": identity})
}
