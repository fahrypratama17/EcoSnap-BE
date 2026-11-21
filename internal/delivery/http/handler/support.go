package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/response"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

type SupportHandler struct{ support *service.SupportService }

func NewSupportHandler(support *service.SupportService) *SupportHandler {
	return &SupportHandler{support: support}
}

// supportMaxBody leaves room for 4000 four-byte characters plus JSON escapes.
const supportMaxBody = 64 * 1024

func supportCall(w http.ResponseWriter, r *http.Request, allowQuery bool) (string, context.Context, context.CancelFunc, bool) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return "", nil, nil, false
	}
	if !allowQuery && r.URL.RawQuery != "" {
		supportError(w, service.ErrInvalidHistoryQuery)
		return "", nil, nil, false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	return uid, ctx, cancel, true
}

func (h *SupportHandler) FAQs(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := supportCall(w, r, false)
	if !ok {
		return
	}
	defer cancel()
	items, err := h.support.FAQs(ctx, uid)
	if err != nil {
		supportError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": items})
}

type supportTicketBody struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Message string `json:"message"`
}

func (h *SupportHandler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := supportCall(w, r, false)
	if !ok {
		return
	}
	defer cancel()
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		response.Error(w, 415, "unsupported_media_type", "Gunakan application/json.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, supportMaxBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body supportTicketBody
	if err = decoder.Decode(&body); err == nil {
		var extra any
		if next := decoder.Decode(&extra); next == nil {
			err = errors.New("trailing JSON")
		} else if next != io.EOF {
			err = next
		}
	}
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			response.Error(w, 413, "body_too_large", "Body terlalu besar.")
			return
		}
		response.Error(w, 400, "invalid_request", "Body harus berisi tepat kind, subject, dan message.")
		return
	}
	t, created, err := h.support.Create(ctx, uid, r.Header.Get("Idempotency-Key"), service.SupportTicketRequest{Kind: body.Kind, Subject: body.Subject, Message: body.Message})
	if err != nil {
		supportError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.JSON(w, status, map[string]any{"data": t})
}

func (h *SupportHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	q, err := historyQuery(r, "status")
	if err != nil {
		supportError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.support.Tickets(ctx, uid, q)
	if err != nil {
		supportError(w, err)
		return
	}
	pageJSON(w, page)
}

func (h *SupportHandler) Detail(w http.ResponseWriter, r *http.Request) {
	uid, ctx, cancel, ok := supportCall(w, r, false)
	if !ok {
		return
	}
	defer cancel()
	t, err := h.support.Ticket(ctx, uid, r.PathValue("ticketID"))
	if err != nil {
		supportError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": t})
}

func supportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidSupportTicket):
		response.Error(w, 400, "invalid_ticket", err.Error())
	case errors.Is(err, service.ErrInvalidIdempotencyKey):
		response.Error(w, 400, "invalid_idempotency_key", err.Error())
	case errors.Is(err, service.ErrInvalidHistoryQuery):
		response.Error(w, 400, "invalid_query", err.Error())
	case errors.Is(err, service.ErrSupportTicketNotFound):
		response.Error(w, 404, "ticket_not_found", err.Error())
	case errors.Is(err, service.ErrSupportTicketConflict):
		response.Error(w, 409, "idempotency_conflict", err.Error())
	case errors.Is(err, service.ErrSupportTicketLimit):
		response.Error(w, 429, "ticket_limit_reached", err.Error())
	case errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		accountError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui; periksa daftar tiket atau ulangi dengan Idempotency-Key yang sama.")
	default:
		log.Printf("dukungan gagal: %v", err)
		response.Error(w, 500, "internal_error", "Operasi dukungan belum berhasil; ulangi dengan Idempotency-Key yang sama.")
	}
}
