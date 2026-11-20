package handler

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type FavoriteHandler struct{ favorites *service.FavoriteService }

func NewFavoriteHandler(favorites *service.FavoriteService) *FavoriteHandler {
	return &FavoriteHandler{favorites: favorites}
}
func favoriteError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrAccountNotFound) || errors.Is(err, service.ErrAccountForbidden) {
		accountError(w, err)
		return
	}
	partnerError(w, err)
}
func (h *FavoriteHandler) Set(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		partnerError(w, service.ErrInvalidPartnerQuery)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.favorites.Set(ctx, uid, r.PathValue("partnerID"), r.Method == http.MethodPut); err != nil {
		favoriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h *FavoriteHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		partnerError(w, service.ErrInvalidPartnerQuery)
		return
	}
	limit, offset := 20, 0
	for key, values := range q {
		if len(values) != 1 || (key != "limit" && key != "offset") {
			partnerError(w, service.ErrInvalidPartnerQuery)
			return
		}
		value, err := strconv.Atoi(values[0])
		if err != nil {
			partnerError(w, service.ErrInvalidPartnerQuery)
			return
		}
		if key == "limit" {
			limit = value
		} else {
			offset = value
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.favorites.List(ctx, uid, limit, offset)
	if err != nil {
		favoriteError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": page.Items, "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "has_more": page.HasMore}})
}
