package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"io"
	"mime"
	"net/http"
	"time"
)

func (h *AccountHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		profileUpdateRequestError(w, service.ErrInvalidProfileUpdate)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		response.Error(w, 415, "unsupported_media_type", "Gunakan Content-Type application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var body struct {
		FullName json.RawMessage `json:"full_name"`
		Phone    json.RawMessage `json:"phone"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&body); err == nil {
		var extra any
		if next := decoder.Decode(&extra); next != io.EOF {
			if next != nil {
				err = next
			} else {
				err = service.ErrInvalidProfileUpdate
			}
		}
	}
	if err != nil {
		profileUpdateRequestError(w, err)
		return
	}
	parse := func(raw json.RawMessage) (*string, error) {
		if len(raw) == 0 {
			return nil, nil
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, service.ErrInvalidProfileUpdate
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return &value, nil
	}
	name, err := parse(body.FullName)
	if err != nil {
		profileUpdateRequestError(w, err)
		return
	}
	phone, err := parse(body.Phone)
	if err != nil {
		profileUpdateRequestError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	p, err := h.accounts.UpdateProfile(ctx, uid, service.AccountProfileUpdate{FullName: name, Phone: phone})
	if err != nil {
		accountError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": p})
}
func profileUpdateRequestError(w http.ResponseWriter, err error) {
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		response.Error(w, 413, "body_too_large", "Body terlalu besar.")
		return
	}
	response.Error(w, 400, "invalid_request", "Kirim satu objek JSON berisi full_name atau phone tanpa null.")
}
