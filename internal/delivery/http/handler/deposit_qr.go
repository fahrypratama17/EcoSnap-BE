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

type DepositQRHandler struct{ qr *service.DepositQRService }

func NewDepositQRHandler(qr *service.DepositQRService) *DepositQRHandler {
	return &DepositQRHandler{qr: qr}
}

func (h *DepositQRHandler) Issue(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		qrError(w, service.ErrDepositNotFound)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.qr.Issue(ctx, uid, r.PathValue("depositID"))
	if err != nil {
		qrError(w, err)
		return
	}
	response.JSON(w, 201, map[string]any{"data": result})
}

// claimToken reads the token from the body so it never appears in URLs or
// access logs.
func claimToken(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return "", "", false
	}
	if r.URL.RawQuery != "" {
		qrError(w, service.ErrQRInvalid)
		return "", "", false
	}
	var in entity.ClaimTokenRequest
	if !decodeStrictJSON(w, r, &in, "Kirim objek JSON berisi token QR.") {
		return "", "", false
	}
	return uid, in.Token, true
}

func (h *DepositQRHandler) Preview(w http.ResponseWriter, r *http.Request) {
	uid, token, ok := claimToken(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.qr.Preview(ctx, uid, token)
	if err != nil {
		qrError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": result})
}

func (h *DepositQRHandler) Claim(w http.ResponseWriter, r *http.Request) {
	uid, token, ok := claimToken(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, created, err := h.qr.Claim(ctx, uid, token)
	if err != nil {
		qrError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.JSON(w, status, map[string]any{"data": result})
}

func qrError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrQRInvalid):
		response.Error(w, 404, "qr_invalid", err.Error())
	case errors.Is(err, service.ErrQRRevoked):
		response.Error(w, 410, "qr_revoked", err.Error())
	case errors.Is(err, service.ErrQRExpired):
		response.Error(w, 410, "qr_expired", err.Error())
	case errors.Is(err, service.ErrDepositAlreadyClaimed):
		response.Error(w, 409, "deposit_already_claimed", err.Error())
	case errors.Is(err, service.ErrClaimForbidden):
		response.Error(w, 403, "claim_forbidden", err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui; ulangi permintaan yang sama.")
	case errors.Is(err, service.ErrDepositNotFound), errors.Is(err, service.ErrDepositForbidden),
		errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		depositError(w, err)
	default:
		// Never log the request token; err comes from storage only.
		log.Printf("operasi QR gagal: %v", err)
		response.Error(w, 500, "internal_error", "Operasi QR belum berhasil.")
	}
}
