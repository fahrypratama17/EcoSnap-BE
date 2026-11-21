package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type GamificationHandler struct{ gamification *service.GamificationService }

func NewGamificationHandler(g *service.GamificationService) *GamificationHandler {
	return &GamificationHandler{gamification: g}
}

func gamificationCall(w http.ResponseWriter, r *http.Request, allowQuery bool) (string, context.Context, context.CancelFunc, bool) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, "unauthorized", "Identitas belum terverifikasi.")
		return "", nil, nil, false
	}
	if !allowQuery && r.URL.RawQuery != "" {
		response.Error(w, http.StatusBadRequest, "invalid_query", "Parameter query tidak didukung.")
		return "", nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	return uid, ctx, cancel, true
}

func (h *GamificationHandler) Missions(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := gamificationCall(w, r, false)
	if !ok {
		return
	}
	defer cancel()
	data, err := h.gamification.Missions(ctx, uid)
	if err != nil {
		gamificationError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *GamificationHandler) ClaimMission(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := gamificationCall(w, r, false)
	if !ok {
		return
	}
	defer cancel()
	if media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || media != "application/json" {
		response.Error(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Gunakan Content-Type application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var body struct {
		PeriodStart string `json:"period_start"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&body)
	if err == nil {
		var extra any
		if next := dec.Decode(&extra); next != io.EOF {
			if next != nil {
				err = next
			} else {
				err = service.ErrInvalidMissionClaim
			}
		}
	}
	if err != nil || body.PeriodStart == "" {
		response.Error(w, http.StatusBadRequest, "invalid_request", service.ErrInvalidMissionClaim.Error())
		return
	}
	periodStart, err := time.Parse(time.RFC3339Nano, body.PeriodStart)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_request", service.ErrInvalidMissionClaim.Error())
		return
	}
	result, created, err := h.gamification.ClaimMission(ctx, uid, r.PathValue("missionID"), periodStart)
	if err != nil {
		gamificationError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.JSON(w, status, map[string]any{"data": result})
}

func (h *GamificationHandler) Badges(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := gamificationCall(w, r, false)
	if !ok {
		return
	}
	defer cancel()
	data, err := h.gamification.Badges(ctx, uid)
	if err != nil {
		gamificationError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *GamificationHandler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := gamificationCall(w, r, true)
	if !ok {
		return
	}
	defer cancel()
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(q) > 1 || len(q["limit"]) > 1 {
		response.Error(w, http.StatusBadRequest, "invalid_query", service.ErrInvalidGamificationQuery.Error())
		return
	}
	if _, present := q["limit"]; len(q) > 0 && (!present || q.Get("limit") == "") {
		response.Error(w, http.StatusBadRequest, "invalid_query", service.ErrInvalidGamificationQuery.Error())
		return
	}
	limit := service.LeaderboardDefaultLimit
	if raw, present := q["limit"]; present {
		parsed, err := strconv.Atoi(raw[0])
		if err != nil {
			gamificationError(w, service.ErrInvalidGamificationQuery)
			return
		}
		limit = parsed
	}
	data, err := h.gamification.Leaderboard(ctx, uid, limit)
	if err != nil {
		gamificationError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h *GamificationHandler) SetLeaderboardVisibility(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := gamificationCall(w, r, false)
	if !ok {
		return
	}
	defer cancel()
	if media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || media != "application/json" {
		response.Error(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Gunakan Content-Type application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var body struct {
		Visible *bool `json:"visible"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	err := dec.Decode(&body)
	if err == nil {
		var extra any
		if next := dec.Decode(&extra); next != io.EOF {
			if next != nil {
				err = next
			} else {
				err = errors.New("extra JSON value")
			}
		}
	}
	if err != nil || body.Visible == nil {
		response.Error(w, http.StatusBadRequest, "invalid_request", "Body wajib berisi visible boolean.")
		return
	}
	data, err := h.gamification.SetLeaderboardVisibility(ctx, uid, *body.Visible)
	if err != nil {
		gamificationError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}

func gamificationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrMissionNotFound):
		response.Error(w, http.StatusNotFound, "mission_not_found", err.Error())
	case errors.Is(err, service.ErrMissionIncomplete):
		response.Error(w, http.StatusConflict, "mission_incomplete", err.Error())
	case errors.Is(err, service.ErrMissionExpired):
		response.Error(w, http.StatusGone, "mission_expired", err.Error())
	case errors.Is(err, service.ErrInvalidGamificationQuery):
		response.Error(w, http.StatusBadRequest, "invalid_query", err.Error())
	case errors.Is(err, service.ErrGamificationForbidden):
		response.Error(w, http.StatusForbidden, "gamification_forbidden", err.Error())
	case errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		accountError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, http.StatusGatewayTimeout, "request_timeout", "Batas waktu terlampaui; ulangi permintaan.")
	default:
		log.Printf("gamification gagal: %v", err)
		response.Error(w, http.StatusInternalServerError, "internal_error", "Operasi gamification belum berhasil.")
	}
}
