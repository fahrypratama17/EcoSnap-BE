package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type SettingsHandler struct{ settings *service.SettingsService }

func NewSettingsHandler(settings *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		settingsRequestError(w, service.ErrInvalidSettingsUpdate)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	data, err := h.settings.Get(ctx, uid)
	if err != nil {
		accountError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": data})
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		settingsRequestError(w, service.ErrInvalidSettingsUpdate)
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		response.Error(w, 415, "unsupported_media_type", "Gunakan Content-Type application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var body struct {
		NotificationEnabled json.RawMessage `json:"notification_enabled"`
		OnboardingCompleted json.RawMessage `json:"onboarding_completed"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&body); err == nil {
		var extra any
		if next := decoder.Decode(&extra); next != io.EOF {
			if next != nil {
				err = next
			} else {
				err = service.ErrInvalidSettingsUpdate
			}
		}
	}
	if err != nil {
		settingsRequestError(w, err)
		return
	}
	parse := func(raw json.RawMessage) (*bool, error) {
		if len(raw) == 0 {
			return nil, nil
		}
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return nil, service.ErrInvalidSettingsUpdate
		}
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return &value, nil
	}
	notification, err := parse(body.NotificationEnabled)
	if err != nil {
		settingsRequestError(w, err)
		return
	}
	onboarding, err := parse(body.OnboardingCompleted)
	if err != nil {
		settingsRequestError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	data, err := h.settings.Update(ctx, uid, service.AccountSettingsUpdate{NotificationEnabled: notification, OnboardingCompleted: onboarding})
	if err != nil {
		settingsError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": data})
}

func settingsError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrInvalidSettingsUpdate) {
		settingsRequestError(w, err)
		return
	}
	accountError(w, err)
}

func settingsRequestError(w http.ResponseWriter, err error) {
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		response.Error(w, 413, "body_too_large", "Body terlalu besar.")
		return
	}
	response.Error(w, 400, "invalid_request", "Kirim satu objek JSON berisi notification_enabled atau onboarding_completed boolean tanpa null, tanpa query.")
}
