package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

var ErrNotificationNotFound = errors.New("notifikasi tidak ditemukan")

type NotificationStore interface {
	// List returns up to limit notifications of uid, newest first, and the
	// caller's total unread count read from the same snapshot.
	List(ctx context.Context, uid string, unreadOnly bool, limit int, after *Cursor) ([]entity.Notification, int64, error)
	// MarkRead sets seen_at once (first read wins) and returns the row; a
	// missing or foreign notification is ErrNotificationNotFound.
	MarkRead(ctx context.Context, uid, id string) (entity.Notification, error)
}

type NotificationService struct{ store NotificationStore }

func NewNotificationService(store NotificationStore) *NotificationService {
	return &NotificationService{store: store}
}

type NotificationQuery struct {
	Limit      int
	Cursor     string
	UnreadOnly bool
}

type NotificationPage struct {
	Page[entity.Notification]
	UnreadCount int64
}

func (s *NotificationService) List(ctx context.Context, uid string, q NotificationQuery) (NotificationPage, error) {
	limit, after, err := pageArgs(HistoryQuery{Limit: q.Limit, Cursor: q.Cursor})
	if err != nil {
		return NotificationPage{}, err
	}
	items, unread, err := s.store.List(ctx, uid, q.UnreadOnly, limit+1, after)
	if err != nil {
		return NotificationPage{}, err
	}
	page := trim(items, limit, func(n entity.Notification) (time.Time, string) { return n.CreatedAt, n.ID })
	return NotificationPage{Page: page, UnreadCount: unread}, nil
}

func (s *NotificationService) MarkRead(ctx context.Context, uid, id string) (entity.Notification, error) {
	if !partnerID.MatchString(id) {
		return entity.Notification{}, ErrNotificationNotFound
	}
	return s.store.MarkRead(ctx, uid, strings.ToLower(id))
}
