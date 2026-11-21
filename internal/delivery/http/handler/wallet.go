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

// historyQuery accepts limit (default 20), cursor, and status when allowed;
// any other or repeated parameter is rejected.
func historyQuery(r *http.Request, allowStatus bool) (service.HistoryQuery, error) {
	q := service.HistoryQuery{Limit: 20}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return q, service.ErrInvalidHistoryQuery
	}
	for key, v := range values {
		if len(v) != 1 {
			return q, service.ErrInvalidHistoryQuery
		}
		switch {
		case key == "limit":
			if q.Limit, err = strconv.Atoi(v[0]); err != nil {
				return q, service.ErrInvalidHistoryQuery
			}
		case key == "cursor":
			q.Cursor = v[0]
		case key == "status" && allowStatus:
			q.Status = v[0]
		default:
			return q, service.ErrInvalidHistoryQuery
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
	q, err := historyQuery(r, false)
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
	q, err := historyQuery(r, true)
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

func walletError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidHistoryQuery):
		response.Error(w, 400, "invalid_query", err.Error())
	case errors.Is(err, service.ErrDepositForbidden), errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		depositError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui.")
	default:
		log.Printf("riwayat gagal: %v", err)
		response.Error(w, 500, "internal_error", "Data belum dapat dimuat.")
	}
}
