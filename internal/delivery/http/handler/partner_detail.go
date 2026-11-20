package handler

import (
	"context"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"net/http"
	"time"
)

func (h *PartnerHandler) Detail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, err := h.partners.Detail(ctx, r.PathValue("partnerID"))
	if err != nil {
		partnerError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": p})
}
