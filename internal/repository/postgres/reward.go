package postgres

import (
	"context"
	"errors"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RewardRepository struct{ pool *pgxpool.Pool }

func NewRewardRepository(pool *pgxpool.Pool) *RewardRepository {
	return &RewardRepository{pool: pool}
}

const rewardColumns = `id::text,name,description,terms,cost_points,stock,stock>0,is_demo`

func scanReward(row pgx.Row) (entity.Reward, error) {
	var r entity.Reward
	err := row.Scan(&r.ID, &r.Name, &r.Description, &r.Terms, &r.CostPoints, &r.Stock, &r.Available, &r.IsDemo)
	return r, err
}

func (r *RewardRepository) Rewards(ctx context.Context) ([]entity.Reward, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+rewardColumns+` FROM public.rewards WHERE is_active ORDER BY cost_points,id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entity.Reward{}
	for rows.Next() {
		item, err := scanReward(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *RewardRepository) Reward(ctx context.Context, id string) (entity.Reward, error) {
	item, err := scanReward(r.pool.QueryRow(ctx, `SELECT `+rewardColumns+` FROM public.rewards WHERE id=$1::uuid AND is_active`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return item, service.ErrRewardNotFound
	}
	return item, err
}
// voucherStatus derives expired from server time; expired is never stored.
const voucherStatus = `CASE WHEN r.status='issued' AND r.expires_at<=clock_timestamp() THEN 'expired' ELSE r.status::text END`

const voucherColumns = `r.id::text,r.reward_id::text,r.reward_name,r.cost_points,r.voucher_code,` + voucherStatus + `,r.created_at,r.expires_at,r.used_at`

func scanVoucher(row pgx.Row) (entity.Voucher, error) {
	var v entity.Voucher
	err := row.Scan(&v.ID, &v.RewardID, &v.RewardName, &v.CostPoints, &v.Code, &v.Status, &v.CreatedAt, &v.ExpiresAt, &v.UsedAt)
	return v, err
}

func authorizeRedeemerTx(ctx context.Context, tx pgx.Tx, uid string) error {
	err := authorizeClaimerTx(ctx, tx, uid)
	if errors.Is(err, service.ErrClaimForbidden) {
		return service.ErrRedeemForbidden
	}
	return err
}

// Redeem debits points, decrements stock, writes the ledger debit, and issues
// the voucher in one transaction. Any failure rolls everything back, so a
// failed redemption never takes points (D-20: no refunds needed).
func (r *RewardRepository) Redeem(ctx context.Context, in service.NewRedemption) (entity.Redemption, bool, error) {
	var out entity.Redemption
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, false, err
	}
	defer rollbackLater(ctx, tx)
	if err = authorizeRedeemerTx(ctx, tx, in.UID); err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('redeem:'||$1||':'||$2,0))`, in.UID, in.IdempotencyKey); err != nil {
		return out, false, err
	}
	var existingID, existingReward string
	err = tx.QueryRow(ctx, `SELECT id::text,reward_id::text FROM public.redemptions WHERE user_uid=$1 AND idempotency_key=$2`, in.UID, in.IdempotencyKey).Scan(&existingID, &existingReward)
	if err == nil {
		if existingReward != in.RewardID {
			return out, false, service.ErrRedeemConflict
		}
		if out.Voucher, err = scanVoucher(tx.QueryRow(ctx, `SELECT `+voucherColumns+` FROM public.redemptions r WHERE r.id=$1::uuid`, existingID)); err != nil {
			return out, false, err
		}
		if err = tx.QueryRow(ctx, `SELECT balance FROM public.wallets WHERE user_uid=$1`, in.UID).Scan(&out.WalletBalance); err != nil {
			return out, false, err
		}
		return out, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, false, err
	}
	var name string
	var cost, stock int64
	err = tx.QueryRow(ctx, `SELECT name,cost_points,stock FROM public.rewards WHERE id=$1::uuid AND is_active FOR UPDATE`, in.RewardID).Scan(&name, &cost, &stock)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, service.ErrRewardNotFound
	}
	if err != nil {
		return out, false, err
	}
	if stock < 1 {
		return out, false, service.ErrRewardOutOfStock
	}
	var balance int64
	if err = tx.QueryRow(ctx, `SELECT balance FROM public.wallets WHERE user_uid=$1 FOR UPDATE`, in.UID).Scan(&balance); err != nil {
		return out, false, err
	}
	if balance < cost {
		return out, false, service.ErrInsufficientPoints
	}
	out.Voucher, err = scanVoucher(tx.QueryRow(ctx, `WITH clk AS (SELECT clock_timestamp() AS t)
 INSERT INTO public.redemptions AS r(id,user_uid,reward_id,idempotency_key,reward_name,cost_points,voucher_code,created_at,expires_at)
 SELECT gen_random_uuid(),$1,$2::uuid,$3,$4,$5,$6,t,t+$7::interval FROM clk RETURNING `+voucherColumns,
		in.UID, in.RewardID, in.IdempotencyKey, name, cost, in.VoucherCode, in.TTL))
	if err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.rewards SET stock=stock-1 WHERE id=$1::uuid`, in.RewardID); err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.point_ledger(id,user_uid,amount,source,redemption_id,idempotency_key,created_at) VALUES(gen_random_uuid(),$1,$2,'redemption',$3::uuid,'redemption:'||$3,$4)`, in.UID, -cost, out.Voucher.ID, out.Voucher.CreatedAt); err != nil {
		return out, false, err
	}
	if err = tx.QueryRow(ctx, `UPDATE public.wallets SET balance=balance-$2,total_spent=total_spent+$2,updated_at=$3 WHERE user_uid=$1 RETURNING balance`, in.UID, cost, out.Voucher.CreatedAt).Scan(&out.WalletBalance); err != nil {
		return out, false, err
	}
	return out, true, tx.Commit(ctx)
}

func (r *RewardRepository) Vouchers(ctx context.Context, uid string, status *string, limit int, after *service.Cursor) ([]entity.Voucher, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return nil, err
	}
	at, id := cursorArgs(after)
	rows, err := tx.Query(ctx, `SELECT `+voucherColumns+` FROM public.redemptions r
 WHERE r.user_uid=$1 AND ($2::text IS NULL OR `+voucherStatus+`=$2)
 AND ($3::timestamptz IS NULL OR (r.created_at,r.id)<($3,$4::uuid))
 ORDER BY r.created_at DESC,r.id DESC LIMIT $5`, uid, status, at, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entity.Voucher{}
	for rows.Next() {
		v, err := scanVoucher(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Voucher returns only the caller's voucher; other users get not-found so
// private codes and IDs do not leak.
func (r *RewardRepository) Voucher(ctx context.Context, uid, id string) (entity.Voucher, error) {
	v, err := scanVoucher(r.pool.QueryRow(ctx, `SELECT `+voucherColumns+` FROM public.redemptions r WHERE r.id=$1::uuid AND r.user_uid=$2`, id, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return v, service.ErrVoucherNotFound
	}
	return v, err
}

func (r *RewardRepository) UseVoucher(ctx context.Context, uid, id string) (entity.Voucher, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return entity.Voucher{}, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return entity.Voucher{}, err
	}
	v, err := scanVoucher(tx.QueryRow(ctx, `SELECT `+voucherColumns+` FROM public.redemptions r WHERE r.id=$1::uuid AND r.user_uid=$2 FOR UPDATE`, id, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return v, service.ErrVoucherNotFound
	}
	if err != nil {
		return v, err
	}
	switch v.Status {
	case "used":
		return v, tx.Commit(ctx)
	case "expired":
		return v, service.ErrVoucherExpired
	}
	// The expiry check repeats in the UPDATE so a use racing the deadline
	// cannot record used_at at or after expires_at.
	v, err = scanVoucher(tx.QueryRow(ctx, `UPDATE public.redemptions r SET status='used',used_at=clock_timestamp() WHERE r.id=$1::uuid AND r.expires_at>clock_timestamp() RETURNING `+voucherColumns, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return v, service.ErrVoucherExpired
	}
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
