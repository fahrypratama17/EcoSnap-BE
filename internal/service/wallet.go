package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

// FormatWeightKG renders stored grams in the wire format used by estimates.
func FormatWeightKG(grams int64) string { return formatPointDecimal(grams, 3) }

var ErrInvalidHistoryQuery = errors.New("parameter riwayat tidak valid")

// Cursor marks the last item of a page in (created_at DESC, id DESC) order.
// Keyset paging keeps pages stable while new entries arrive.
type Cursor struct {
	At time.Time
	ID string
}

func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "," + id))
}

func decodeCursor(raw string) (*Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, ErrInvalidHistoryQuery
	}
	at, id, ok := strings.Cut(string(b), ",")
	t, err := time.Parse(time.RFC3339Nano, at)
	if !ok || err != nil || !partnerID.MatchString(id) {
		return nil, ErrInvalidHistoryQuery
	}
	return &Cursor{At: t, ID: strings.ToLower(id)}, nil
}

type WalletStore interface {
	Wallet(ctx context.Context, uid string) (entity.Wallet, error)
	PointHistory(ctx context.Context, uid string, limit int, after *Cursor) ([]entity.PointHistoryEntry, error)
	// PartnerDeposits lists deposits of the caller's verified partner.
	PartnerDeposits(ctx context.Context, uid string, status *string, limit int, after *Cursor) ([]entity.PartnerDepositSummary, error)
}

type WalletService struct{ store WalletStore }

func NewWalletService(store WalletStore) *WalletService { return &WalletService{store: store} }

type HistoryQuery struct {
	Limit  int
	Cursor string
	Status string // partner deposits only
}

type Page[T any] struct {
	Items      []T
	Limit      int
	NextCursor string
}

func (s *WalletService) Wallet(ctx context.Context, uid string) (entity.Wallet, error) {
	return s.store.Wallet(ctx, uid)
}

func pageArgs(q HistoryQuery) (int, *Cursor, error) {
	if q.Limit < 1 || q.Limit > 100 {
		return 0, nil, ErrInvalidHistoryQuery
	}
	after, err := decodeCursor(q.Cursor)
	return q.Limit, after, err
}

// trim fetches limit+1 rows so the next cursor exists only when more remain.
func trim[T any](items []T, limit int, key func(T) (time.Time, string)) Page[T] {
	out := Page[T]{Items: items, Limit: limit}
	if len(items) > limit {
		out.Items = items[:limit]
		at, id := key(out.Items[limit-1])
		out.NextCursor = encodeCursor(at, id)
	}
	return out
}

func (s *WalletService) PointHistory(ctx context.Context, uid string, q HistoryQuery) (Page[entity.PointHistoryEntry], error) {
	limit, after, err := pageArgs(q)
	if err != nil || q.Status != "" {
		return Page[entity.PointHistoryEntry]{}, ErrInvalidHistoryQuery
	}
	items, err := s.store.PointHistory(ctx, uid, limit+1, after)
	if err != nil {
		return Page[entity.PointHistoryEntry]{}, err
	}
	return trim(items, limit, func(e entity.PointHistoryEntry) (time.Time, string) { return e.CreatedAt, e.ID }), nil
}

func (s *WalletService) PartnerDeposits(ctx context.Context, uid string, q HistoryQuery) (Page[entity.PartnerDepositSummary], error) {
	limit, after, err := pageArgs(q)
	if err != nil {
		return Page[entity.PartnerDepositSummary]{}, err
	}
	var status *string
	switch q.Status {
	case "":
	case "created", "qr_issued", "claimed":
		status = &q.Status
	default:
		return Page[entity.PartnerDepositSummary]{}, ErrInvalidHistoryQuery
	}
	items, err := s.store.PartnerDeposits(ctx, uid, status, limit+1, after)
	if err != nil {
		return Page[entity.PartnerDepositSummary]{}, err
	}
	return trim(items, limit, func(d entity.PartnerDepositSummary) (time.Time, string) { return d.CreatedAt, d.ID }), nil
}
