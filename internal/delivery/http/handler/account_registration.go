package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"io"
	"log"
	"mime"
	"net/http"
	"time"
)

type AccountHandler struct{ accounts *service.AccountService }

func NewAccountHandler(accounts *service.AccountService) *AccountHandler {
	return &AccountHandler{accounts: accounts}
}
func (h *AccountHandler) Register(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.IdentityFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		response.Error(w, 415, "unsupported_media_type", "Gunakan Content-Type application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var in service.AccountRegistration
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&in)
	if err == nil {
		var extra any
		if extraErr := decoder.Decode(&extra); extraErr != io.EOF {
			if extraErr != nil {
				err = extraErr
			} else {
				err = service.ErrInvalidAccount
			}
		}
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			response.Error(w, 413, "body_too_large", "Body terlalu besar.")
			return
		}
		response.Error(w, 400, "invalid_request", "Body harus satu objek JSON dengan field pendaftaran yang valid.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, created, err := h.accounts.Register(ctx, identity.UID, identity.Email, in)
	if err != nil {
		accountError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.JSON(w, status, map[string]any{"data": p})
}
func accountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidProfileUpdate):
		response.Error(w, 400, "invalid_profile", err.Error())
	case errors.Is(err, service.ErrInvalidAccount):
		response.Error(w, 400, "invalid_account", err.Error())
	case errors.Is(err, service.ErrAccountNotFound):
		response.Error(w, 404, "account_not_found", err.Error())
	case errors.Is(err, service.ErrAccountForbidden):
		response.Error(w, 403, "account_forbidden", err.Error())
	case errors.Is(err, service.ErrAccountConflict):
		response.Error(w, 409, "account_conflict", err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui; periksa profil sebelum mengulang.")
	default:
		log.Printf("operasi akun gagal: %v", err)
		response.Error(w, 500, "internal_error", "Operasi akun belum berhasil; periksa profil sebelum mengulang.")
	}
}
