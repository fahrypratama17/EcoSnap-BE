package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type DepositHandler struct{ deposits *service.DepositService }

func NewDepositHandler(deposits *service.DepositService) *DepositHandler {
	return &DepositHandler{deposits: deposits}
}

func (h *DepositHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		depositError(w, service.ErrInvalidEstimate)
		return
	}
	var in entity.DepositCreateRequest
	if !decodeStrictJSON(w, r, &in, "Kirim satu objek JSON setoran yang valid.") {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, created, err := h.deposits.Create(ctx, uid, r.Header.Get("Idempotency-Key"), in)
	if err != nil {
		depositError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.JSON(w, status, map[string]any{"data": result})
}

func (h *DepositHandler) Detail(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		depositError(w, service.ErrInvalidEstimate)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.deposits.Detail(ctx, uid, r.PathValue("depositID"))
	if err != nil {
		depositError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": result})
}

func depositError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidIdempotencyKey):
		response.Error(w, 400, "invalid_idempotency_key", err.Error())
	case errors.Is(err, service.ErrDepositForbidden):
		response.Error(w, 403, "deposit_forbidden", err.Error())
	case errors.Is(err, service.ErrDepositNotFound):
		response.Error(w, 404, "deposit_not_found", err.Error())
	case errors.Is(err, service.ErrDepositConflict):
		response.Error(w, 409, "idempotency_conflict", err.Error())
	case errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		accountError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui; ulangi dengan Idempotency-Key yang sama.")
	case errors.Is(err, service.ErrInvalidEstimate), errors.Is(err, service.ErrWasteNotAccepted),
		errors.Is(err, service.ErrPointRateUnavailable), errors.Is(err, service.ErrWasteCategoryNotFound):
		estimateError(w, err)
	default:
		log.Printf("setoran gagal: %v", err)
		response.Error(w, 500, "internal_error", "Setoran belum tersimpan; ulangi dengan Idempotency-Key yang sama.")
	}
}
