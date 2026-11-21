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

type NotificationHandler struct {
	notifications *service.NotificationService
}

func NewNotificationHandler(notifications *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notifications: notifications}
}

// notificationQuery accepts limit (default 20), cursor and unread=true; any
// other, repeated or malformed parameter is rejected.
func notificationQuery(r *http.Request) (service.NotificationQuery, error) {
	q := service.NotificationQuery{Limit: 20}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return q, service.ErrInvalidHistoryQuery
	}
	for key, v := range values {
		if len(v) != 1 {
			return q, service.ErrInvalidHistoryQuery
		}
		switch key {
		case "limit":
			if q.Limit, err = strconv.Atoi(v[0]); err != nil {
				return q, service.ErrInvalidHistoryQuery
			}
		case "cursor":
			q.Cursor = v[0]
		case "unread":
			if v[0] != "true" {
				return q, service.ErrInvalidHistoryQuery
			}
			q.UnreadOnly = true
		default:
			return q, service.ErrInvalidHistoryQuery
		}
	}
	return q, nil
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	q, err := notificationQuery(r)
	if err != nil {
		notificationError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	page, err := h.notifications.List(ctx, uid, q)
	if err != nil {
		notificationError(w, err)
		return
	}
	var next any
	if page.NextCursor != "" {
		next = page.NextCursor
	}
	response.JSON(w, 200, map[string]any{
		"data":         page.Items,
		"unread_count": page.UnreadCount,
		"pagination":   map[string]any{"limit": page.Limit, "next_cursor": next},
	})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UIDFromContext(r.Context())
	if !ok {
		response.Error(w, 401, "unauthorized", "Identitas belum terverifikasi.")
		return
	}
	if r.URL.RawQuery != "" {
		notificationError(w, service.ErrInvalidHistoryQuery)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	n, err := h.notifications.MarkRead(ctx, uid, r.PathValue("notificationID"))
	if err != nil {
		notificationError(w, err)
		return
	}
	response.JSON(w, 200, map[string]any{"data": n})
}

func notificationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotificationNotFound):
		response.Error(w, 404, "notification_not_found", err.Error())
	case errors.Is(err, service.ErrInvalidHistoryQuery):
		response.Error(w, 400, "invalid_query", "Parameter notifikasi tidak valid.")
	case errors.Is(err, service.ErrAccountNotFound), errors.Is(err, service.ErrAccountForbidden):
		depositError(w, err)
	case errors.Is(err, context.DeadlineExceeded):
		response.Error(w, 504, "request_timeout", "Batas waktu terlampaui.")
	default:
		log.Printf("notifikasi gagal: %v", err)
		response.Error(w, 500, "internal_error", "Data belum dapat dimuat.")
	}
}
