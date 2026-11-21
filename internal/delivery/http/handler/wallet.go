package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type WalletHandler struct{ wallets *service.WalletService }

func NewWalletHandler(wallets *service.WalletService) *WalletHandler {
	return &WalletHandler{wallets: wallets}
}

func (h *WalletHandler) Wallet(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		walletError(w, service.ErrInvalidHistoryQuery)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.wallets.Wallet(ctx, uid)
	if err != nil {
		walletError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": result})
}

// historyQuery accepts limit (default 20), cursor, and the listed extra keys
// (status, type, from, to); any other or repeated parameter is rejected.
// from/to are RFC 3339 timestamps with offset, e.g. 2026-11-01T00:00:00+07:00.
func historyQuery(r *http.Request, extra ...string) (service.HistoryQuery, error) {
	q := service.HistoryQuery{Limit: 20}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return q, service.ErrInvalidHistoryQuery
	}
	for key, v := range values {
		if len(v) != 1 || (key != "limit" && key != "cursor" && !slices.Contains(extra, key)) {
			return q, service.ErrInvalidHistoryQuery
		}
		switch key {
		case "limit":
			if q.Limit, err = strconv.Atoi(v[0]); err != nil {
				return q, service.ErrInvalidHistoryQuery
			}
		case "cursor":
			q.Cursor = v[0]
		case "status":
			q.Status = v[0]
		case "type":
			q.Type = v[0]
		case "from", "to":
			t, err := time.Parse(time.RFC3339, v[0])
			if err != nil {
				return q, service.ErrInvalidHistoryQuery
			}
			if key == "from" {
				q.From = &t
			} else {
				q.To = &t
			}
		}
	}
	return q, nil
}

func pageJSON[T any](w http.ResponseWriter, page service.Page[T]) {
	var next any
	if page.NextCursor != "" {
		next = page.NextCursor
	}
	response.JSON(w, 200, map[string]any{"data": page.Items, "pagination": map[string]any{"limit": page.Limit, "next_cursor": next}})
}

func (h *WalletHandler) PointHistory(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	q, err := historyQuery(r, "type", "from", "to")
	if err != nil {
		walletError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.wallets.PointHistory(ctx, uid, q)
	if err != nil {
		walletError(w, err)
		return
	}
	pageJSON(w, page)
}

func (h *WalletHandler) PartnerDeposits(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	q, err := historyQuery(r, "status", "from", "to")
	if err != nil {
		walletError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.wallets.PartnerDeposits(ctx, uid, q)
	if err != nil {
		walletError(w, err)
		return
	}
	pageJSON(w, page)
}

// ClaimedDeposit is the user-side deposit detail for a history entry.
func (h *WalletHandler) ClaimedDeposit(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		walletError(w, service.ErrInvalidHistoryQuery)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.wallets.ClaimedDeposit(ctx, uid, r.PathValue("depositID"))
	if err != nil {
		walletError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": result})
}

func (h *WalletHandler) UserHome(w http.ResponseWriter, r *http.Request) {
	h.home(w, r, func(ctx context.Context, uid string) (any, error) { return h.wallets.UserHome(ctx, uid) })
}

func (h *WalletHandler) PartnerHome(w http.ResponseWriter, r *http.Request) {
	h.home(w, r, func(ctx context.Context, uid string) (any, error) { return h.wallets.PartnerHome(ctx, uid) })
}

func (h *WalletHandler) home(w http.ResponseWriter, r *http.Request, load func(context.Context, string) (any, error)) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		walletError(w, service.ErrInvalidHistoryQuery)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := load(ctx, uid)
	if err != nil {
		walletError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": result})
}

func walletError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidHistoryQuery):
		response.Error(w, 400, "invalid_query", err.Error())
	case errors.Is(err, service.ErrDepositForbidden), errors.Is(err, service.ErrDepositNotFound), errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		depositError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui.")
	default:
		log.Printf("riwayat gagal: %v", err)
		response.Error(w, 500, "internal_error", "Data belum dapat dimuat.")
	}
}
