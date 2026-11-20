package handler

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type PartnerHandler struct{ partners *service.PartnerService }

func NewPartnerHandler(partners *service.PartnerService) *PartnerHandler {
	return &PartnerHandler{partners: partners}
}
func (h *PartnerHandler) List(w http.ResponseWriter, r *http.Request) {
	query, parseErr := url.ParseQuery(r.URL.RawQuery)
	if parseErr != nil {
		partnerError(w, service.ErrInvalidPartnerQuery)
		return
	}
	f := service.PartnerFilter{Query: query.Get("q"), Material: query.Get("material"), Limit: 20}
	for key, values := range query {
		if (key != "q" && key != "material" && key != "limit" && key != "offset") || len(values) != 1 {
			partnerError(w, service.ErrInvalidPartnerQuery)
			return
		}
	}
	var err error
	if raw, ok := query["limit"]; ok {
		f.Limit, err = strconv.Atoi(raw[0])
		if err != nil || f.Limit == 0 {
			partnerError(w, service.ErrInvalidPartnerQuery)
			return
		}
	}
	if raw, ok := query["offset"]; ok {
		f.Offset, err = strconv.Atoi(raw[0])
		if err != nil {
			partnerError(w, service.ErrInvalidPartnerQuery)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.partners.List(ctx, f)
	if err != nil {
		partnerError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": page.Items, "pagination": map[string]any{"limit": page.Limit, "offset": page.Offset, "has_more": page.HasMore}})
}
func (h *PartnerHandler) Categories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	items, err := h.partners.Categories(ctx)
	if err != nil {
		partnerError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": items})
}
func partnerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidPartnerQuery):
		response.Error(w, 400, "invalid_partner_query", err.Error())
	case errors.Is(err, service.ErrPartnerNotFound):
		response.Error(w, 404, "partner_not_found", err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui.")
	default:
		log.Printf("operasi mitra gagal: %v", err)
		response.Error(w, 500, "internal_error", "Data mitra belum dapat dimuat.")
	}
}
