package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type RewardHandler struct{ rewards *service.RewardService }

func NewRewardHandler(rewards *service.RewardService) *RewardHandler {
	return &RewardHandler{rewards: rewards}
}

// rewardCall resolves the caller, rejects query strings, and bounds the call.
func rewardCall(w http.ResponseWriter, r *http.Request) (string, context.Context, context.CancelFunc, bool) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return "", nil, nil, false
	}
	if r.URL.RawQuery != "" {
		rewardError(w, service.ErrInvalidHistoryQuery)
		return "", nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	return uid, ctx, cancel, true
}

func (h *RewardHandler) List(w http.ResponseWriter, r *http.Request) {
	_, ctx, cancel, ok := rewardCall(w, r)
	if !ok {
		return
	}
	defer cancel()
	items, err := h.rewards.Rewards(ctx)
	if err != nil {
		rewardError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": items})
}

func (h *RewardHandler) Detail(w http.ResponseWriter, r *http.Request) {
	_, ctx, cancel, ok := rewardCall(w, r)
	if !ok {
		return
	}
	defer cancel()
	item, err := h.rewards.Reward(ctx, r.PathValue("rewardID"))
	if err != nil {
		rewardError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": item})
}

func (h *RewardHandler) Redeem(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := rewardCall(w, r)
	if !ok {
		return
	}
	defer cancel()
	result, created, err := h.rewards.Redeem(ctx, uid, r.PathValue("rewardID"), r.Header.Get("Idempotency-Key"))
	if err != nil {
		rewardError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.JSON(w, status, map[string]any{"data": result})
}

func (h *RewardHandler) Vouchers(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	q, err := historyQuery(r, true)
	if err != nil {
		rewardError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.rewards.Vouchers(ctx, uid, q)
	if err != nil {
		rewardError(w, err)
		return
	}
	pageJSON(w, page)
}

func (h *RewardHandler) Voucher(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := rewardCall(w, r)
	if !ok {
		return
	}
	defer cancel()
	v, err := h.rewards.Voucher(ctx, uid, r.PathValue("voucherID"))
	if err != nil {
		rewardError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": v})
}

func (h *RewardHandler) UseVoucher(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := rewardCall(w, r)
	if !ok {
		return
	}
	defer cancel()
	v, err := h.rewards.UseVoucher(ctx, uid, r.PathValue("voucherID"))
	if err != nil {
		rewardError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": v})
}

func rewardError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrRewardNotFound):
		response.Error(w, 404, "reward_not_found", err.Error())
	case errors.Is(err, service.ErrVoucherNotFound):
		response.Error(w, 404, "voucher_not_found", err.Error())
	case errors.Is(err, service.ErrRewardOutOfStock):
		response.Error(w, 409, "out_of_stock", err.Error())
	case errors.Is(err, service.ErrInsufficientPoints):
		response.Error(w, 409, "insufficient_points", err.Error())
	case errors.Is(err, service.ErrRedeemConflict):
		response.Error(w, 409, "idempotency_conflict", err.Error())
	case errors.Is(err, service.ErrVoucherExpired):
		response.Error(w, 410, "voucher_expired", err.Error())
	case errors.Is(err, service.ErrRedeemForbidden):
		response.Error(w, 403, "redeem_forbidden", err.Error())
	case errors.Is(err, service.ErrInvalidIdempotencyKey):
		response.Error(w, 400, "invalid_idempotency_key", err.Error())
	case errors.Is(err, service.ErrInvalidHistoryQuery):
		response.Error(w, 400, "invalid_query", err.Error())
	case errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		accountError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui; ulangi dengan Idempotency-Key yang sama.")
	default:
		log.Printf("reward gagal: %v", err)
		response.Error(w, 500, "internal_error", "Operasi reward belum berhasil; poin tidak dipotong.")
	}
}
