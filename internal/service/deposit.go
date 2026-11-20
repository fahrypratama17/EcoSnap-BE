package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

var (
	ErrInvalidIdempotencyKey = errors.New("Idempotency-Key wajib 8-128 karakter huruf, angka, '-' atau '_'")
	ErrDepositForbidden      = errors.New("akun belum berwenang membuat setoran")
	ErrDepositNotFound       = errors.New("setoran tidak ditemukan")
	ErrDepositConflict       = errors.New("Idempotency-Key sudah dipakai untuk isi setoran berbeda")
)

var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

type DepositLine struct {
	CategoryID string
	Grams      int64
}

// DepositPricer computes the authoritative total from the tariffs the store
// selected inside its transaction. It is the estimator rule (D-17).
type DepositPricer func(PointRateSelection) (entity.PointEstimate, error)

type DepositRecordItem struct {
	CategoryID, CategoryName, RateID string
	Grams, RateHundredths            int64
	IsDemo                           bool
}
type DepositRecord struct {
	ID, PartnerID, Status  string
	TotalGrams, TotalPoint int64
	UsesDemoRates          bool
	RatesAsOf, CreatedAt   time.Time
	Items                  []DepositRecordItem
}
type NewDeposit struct {
	CreatorUID, IdempotencyKey, RequestHash string
	Lines                                   []DepositLine
	TotalGrams                              int64
}
type DepositStore interface {
	// CreateDeposit returns created=false when the same key and request were
	// already stored for the caller's partner.
	CreateDeposit(context.Context, NewDeposit, DepositPricer) (DepositRecord, bool, error)
	DepositForMitra(ctx context.Context, uid, depositID string) (DepositRecord, error)
}

type DepositService struct{ store DepositStore }

func NewDepositService(store DepositStore) *DepositService { return &DepositService{store: store} }

func (s *DepositService) Create(ctx context.Context, uid, key string, in entity.DepositCreateRequest) (entity.Deposit, bool, error) {
	if !idempotencyKey.MatchString(key) {
		return entity.Deposit{}, false, ErrInvalidIdempotencyKey
	}
	lines, err := parseEstimateItems(in.Items)
	if err != nil {
		return entity.Deposit{}, false, err
	}
	// The hash covers the normalized request so a retry with the same key but
	// different materials or weights is rejected instead of replayed.
	h := sha256.New()
	next := NewDeposit{CreatorUID: uid, IdempotencyKey: key, TotalGrams: lines.totalGrams, Lines: make([]DepositLine, len(lines.ids))}
	for i, id := range lines.ids {
		next.Lines[i] = DepositLine{CategoryID: id, Grams: lines.grams[i]}
		h.Write([]byte(id + ":" + strconv.FormatInt(lines.grams[i], 10) + "\n"))
	}
	next.RequestHash = hex.EncodeToString(h.Sum(nil))
	price := func(selection PointRateSelection) (entity.PointEstimate, error) {
		return priceEstimate(nil, lines, selection)
	}
	record, created, err := s.store.CreateDeposit(ctx, next, price)
	if err != nil {
		return entity.Deposit{}, false, err
	}
	return depositView(record), created, nil
}

func (s *DepositService) Detail(ctx context.Context, uid, depositID string) (entity.Deposit, error) {
	if !partnerID.MatchString(depositID) {
		return entity.Deposit{}, ErrDepositNotFound
	}
	record, err := s.store.DepositForMitra(ctx, uid, strings.ToLower(depositID))
	if err != nil {
		return entity.Deposit{}, err
	}
	return depositView(record), nil
}

func depositView(r DepositRecord) entity.Deposit {
	out := entity.Deposit{ID: r.ID, PartnerID: r.PartnerID, Status: r.Status, TotalWeightKG: formatPointDecimal(r.TotalGrams, 3), TotalPoints: r.TotalPoint, UsesDemoRates: r.UsesDemoRates, RatesAsOf: r.RatesAsOf, CreatedAt: r.CreatedAt, Items: make([]entity.PointEstimateItem, len(r.Items))}
	for i, item := range r.Items {
		out.Items[i] = entity.PointEstimateItem{CategoryID: item.CategoryID, CategoryName: item.CategoryName, WeightKG: formatPointDecimal(item.Grams, 3), RateID: item.RateID, PointsPerKG: formatPointDecimal(item.RateHundredths, 2), ExactPoints: formatPointDecimal(item.Grams*item.RateHundredths, 5), IsDemo: item.IsDemo}
	}
	return out
}
