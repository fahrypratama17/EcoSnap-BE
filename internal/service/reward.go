package service

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

// VoucherTTL is D-20: one week from redemption.
const VoucherTTL = 7 * 24 * time.Hour

var (
	ErrRewardNotFound     = errors.New("reward tidak ditemukan")
	ErrRewardOutOfStock   = errors.New("stok reward habis")
	ErrInsufficientPoints = errors.New("poin tidak cukup")
	ErrRedeemForbidden    = errors.New("hanya akun pengguna yang dapat menukar reward")
	ErrRedeemConflict     = errors.New("Idempotency-Key sudah dipakai untuk reward lain")
	ErrVoucherNotFound    = errors.New("voucher tidak ditemukan")
	ErrVoucherExpired     = errors.New("voucher sudah kedaluwarsa")
)

// Crockford base32 without I, L, O, U to avoid misreading when typed.
const voucherAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// newVoucherCode returns ECO-XXXX-XXXX (40 random bits).
func newVoucherCode() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("ECO-")
	for i, v := range raw {
		if i == 4 {
			b.WriteByte('-')
		}
		b.WriteByte(voucherAlphabet[v&31])
	}
	return b.String(), nil
}

type NewRedemption struct {
	UID, RewardID, IdempotencyKey, VoucherCode string
	TTL                                        time.Duration
}

type RewardStore interface {
	Rewards(ctx context.Context) ([]entity.Reward, error)
	Reward(ctx context.Context, id string) (entity.Reward, error)
	// Redeem returns created=false when the same key and reward were already
	// redeemed by uid.
	Redeem(ctx context.Context, in NewRedemption) (entity.Redemption, bool, error)
	Vouchers(ctx context.Context, uid string, status *string, limit int, after *Cursor) ([]entity.Voucher, error)
	Voucher(ctx context.Context, uid, id string) (entity.Voucher, error)
	UseVoucher(ctx context.Context, uid, id string) (entity.Voucher, error)
}

type RewardService struct{ store RewardStore }

func NewRewardService(store RewardStore) *RewardService { return &RewardService{store: store} }

func (s *RewardService) Rewards(ctx context.Context) ([]entity.Reward, error) {
	return s.store.Rewards(ctx)
}

func (s *RewardService) Reward(ctx context.Context, id string) (entity.Reward, error) {
	if !partnerID.MatchString(id) {
		return entity.Reward{}, ErrRewardNotFound
	}
	return s.store.Reward(ctx, strings.ToLower(id))
}

func (s *RewardService) Redeem(ctx context.Context, uid, rewardID, key string) (entity.Redemption, bool, error) {
	if !idempotencyKey.MatchString(key) {
		return entity.Redemption{}, false, ErrInvalidIdempotencyKey
	}
	if !partnerID.MatchString(rewardID) {
		return entity.Redemption{}, false, ErrRewardNotFound
	}
	code, err := newVoucherCode()
	if err != nil {
		return entity.Redemption{}, false, err
	}
	return s.store.Redeem(ctx, NewRedemption{UID: uid, RewardID: strings.ToLower(rewardID), IdempotencyKey: key, VoucherCode: code, TTL: VoucherTTL})
}

func (s *RewardService) Vouchers(ctx context.Context, uid string, q HistoryQuery) (Page[entity.Voucher], error) {
	limit, after, err := pageArgs(q)
	if err != nil {
		return Page[entity.Voucher]{}, err
	}
	var status *string
	switch q.Status {
	case "":
	case "issued", "used", "expired":
		status = &q.Status
	default:
		return Page[entity.Voucher]{}, ErrInvalidHistoryQuery
	}
	items, err := s.store.Vouchers(ctx, uid, status, limit+1, after)
	if err != nil {
		return Page[entity.Voucher]{}, err
	}
	return trim(items, limit, func(v entity.Voucher) (time.Time, string) { return v.CreatedAt, v.ID }), nil
}

func (s *RewardService) Voucher(ctx context.Context, uid, id string) (entity.Voucher, error) {
	if !partnerID.MatchString(id) {
		return entity.Voucher{}, ErrVoucherNotFound
	}
	return s.store.Voucher(ctx, uid, strings.ToLower(id))
}

// UseVoucher marks the voucher used. Repeating it on a used voucher returns
// the same result.
func (s *RewardService) UseVoucher(ctx context.Context, uid, id string) (entity.Voucher, error) {
	if !partnerID.MatchString(id) {
		return entity.Voucher{}, ErrVoucherNotFound
	}
	return s.store.UseVoucher(ctx, uid, strings.ToLower(id))
}
