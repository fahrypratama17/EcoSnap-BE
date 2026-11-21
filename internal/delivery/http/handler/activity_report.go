package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type ActivityReportHandler struct {
	reports *service.ActivityReportService
}

func NewActivityReportHandler(reports *service.ActivityReportService) *ActivityReportHandler {
	return &ActivityReportHandler{reports: reports}
}

// activityReportQuery accepts only from and to (RFC 3339 with an explicit
// offset or Z), each at most once.
func activityReportQuery(r *http.Request) (service.ActivityReportQuery, error) {
	var q service.ActivityReportQuery
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return q, service.ErrInvalidActivityReportQuery
	}
	for key, v := range values {
		if len(v) != 1 || (key != "from" && key != "to") {
			return q, service.ErrInvalidActivityReportQuery
		}
		t, err := time.Parse(time.RFC3339, v[0])
		if err != nil {
			return q, service.ErrInvalidActivityReportQuery
		}
		if key == "from" {
			q.From = &t
		} else {
			q.To = &t
		}
	}
	return q, nil
}

func (h *ActivityReportHandler) Report(w http.ResponseWriter, r *http.Request) {
	uid, q, ok := h.prepare(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	report, err := h.reports.Report(ctx, uid, q)
	if err != nil {
		activityReportError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response.JSON(w, 200, map[string]any{"data": report})
}

func (h *ActivityReportHandler) PDF(w http.ResponseWriter, r *http.Request) {
	uid, q, ok := h.prepare(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	doc, err := h.reports.PDF(ctx, uid, q)
	if err != nil {
		activityReportError(w, err)
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", "application/pdf")
	hd.Set("Content-Disposition", `attachment; filename="`+doc.Filename+`"`)
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Cache-Control", "no-store")
	hd.Set("Content-Length", strconv.Itoa(len(doc.Content)))
	w.WriteHeader(200)
	if _, err := w.Write(doc.Content); err != nil {
		log.Printf("menulis PDF laporan: %v", err)
	}
}

func (h *ActivityReportHandler) prepare(w http.ResponseWriter, r *http.Request) (string, service.ActivityReportQuery, bool) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return "", service.ActivityReportQuery{}, false
	}
	q, err := activityReportQuery(r)
	if err != nil {
		activityReportError(w, err)
		return "", q, false
	}
	return uid, q, true
}

func activityReportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidActivityReportQuery):
		response.Error(w, 400, "invalid_query", err.Error())
	case errors.Is(err, service.ErrDepositForbidden), errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		depositError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui.")
	default:
		log.Printf("laporan aktivitas gagal: %v", err)
		response.Error(w, 500, "internal_error", "Laporan belum dapat dibuat.")
	}
}
