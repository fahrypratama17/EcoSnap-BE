package httpdelivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"firebase.google.com/go/v4/auth"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	httpdelivery "github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/handler"
	"github.com/fahrypratama17/EcoSnap-BE/internal/delivery/http/middleware"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

const notifID = "70000000-0000-4000-8000-000000000001"

type notificationMemory struct {
	limit  int
	unread bool
	after  *service.Cursor
}

func (m *notificationMemory) List(_ context.Context, _ string, unread bool, limit int, after *service.Cursor) ([]entity.Notification, int64, error) {
	m.limit, m.unread, m.after = limit, unread, after
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []entity.Notification{{ID: notifID, Type: "voucher_issued", Title: "t", Body: "b", Target: entity.NotificationTarget{Type: "voucher", ID: "x"}, CreatedAt: at}}, 7, nil
}
func (m *notificationMemory) MarkRead(_ context.Context, _ string, id string) (entity.Notification, error) {
	if id != notifID {
		return entity.Notification{}, service.ErrNotificationNotFound
	}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return entity.Notification{ID: id, CreatedAt: at, SeenAt: &at}, nil
}

func TestNotificationHTTP(t *testing.T) {
	verifier := verifierFunc(func(context.Context, string) (*auth.Token, error) { return &auth.Token{UID: "u"}, nil })
	mux := httpdelivery.NewRouter(verifier)
	store := &notificationMemory{}
	h := handler.NewNotificationHandler(service.NewNotificationService(store))
	protect := middleware.RequireAuth(verifier)
	mux.Handle("GET /api/v1/me/notifications", protect(http.HandlerFunc(h.List)))
	mux.Handle("PATCH /api/v1/me/notifications/{notificationID}/read", protect(http.HandlerFunc(h.MarkRead)))

	w := accountRequest(mux, "GET", "/api/v1/me/notifications", "")
	var body struct {
		Data        []entity.Notification `json:"data"`
		UnreadCount int64                 `json:"unread_count"`
		Pagination  struct {
			Limit      int     `json:"limit"`
			NextCursor *string `json:"next_cursor"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); w.Code != 200 || err != nil || store.limit != 21 || body.UnreadCount != 7 || len(body.Data) != 1 || body.Pagination.Limit != 20 || body.Data[0].SeenAt != nil {
		t.Fatalf("default: %d %s limit=%d", w.Code, w.Body, store.limit)
	}
	if w = accountRequest(mux, "GET", "/api/v1/me/notifications?unread=true&limit=5", ""); w.Code != 200 || !store.unread || store.limit != 6 {
		t.Fatalf("unread: %d %s", w.Code, w.Body)
	}
	for _, q := range []string{"limit=0", "limit=101", "limit=x", "limit=1&limit=2", "unread=false", "unread=1", "cursor=!!", "type=x", "cursor=a;b"} {
		if w = accountRequest(mux, "GET", "/api/v1/me/notifications?"+q, ""); w.Code != 400 {
			t.Fatalf("%s: %d", q, w.Code)
		}
	}
	if w = accountRequest(mux, "PATCH", "/api/v1/me/notifications/"+notifID+"/read", ""); w.Code != 200 {
		t.Fatalf("read: %d %s", w.Code, w.Body)
	}
	if w = accountRequest(mux, "PATCH", "/api/v1/me/notifications/"+notifID+"/read?x=1", ""); w.Code != 400 {
		t.Fatalf("query on read: %d", w.Code)
	}
	for _, id := range []string{"not-a-uuid", "70000000-0000-4000-8000-000000000009"} {
		if w = accountRequest(mux, "PATCH", "/api/v1/me/notifications/"+id+"/read", ""); w.Code != 404 {
			t.Fatalf("%s: %d", id, w.Code)
		}
	}
}
