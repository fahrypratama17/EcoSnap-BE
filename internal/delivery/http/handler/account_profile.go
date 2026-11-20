package handler

import (
	"context"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"net/http"
	"time"
)

func (h *AccountHandler) Profile(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, err := h.accounts.Profile(ctx, uid)
	if err != nil {
		accountError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": p})
}
