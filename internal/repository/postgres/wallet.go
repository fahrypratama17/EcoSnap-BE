package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletRepository struct{ pool *pgxpool.Pool }

func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

// cursorArgs maps a nil cursor to NULLs so one query serves the first page.
func cursorArgs(after *service.Cursor) (*time.Time, *string) {
	if after == nil {
		return nil, nil
	}
	return &after.At, &after.ID
}

func (r *WalletRepository) Wallet(ctx context.Context, uid string) (entity.Wallet, error) {
	var w entity.Wallet
	var status string
	err := r.pool.QueryRow(ctx, `SELECT a.status::text,w.balance,w.total_earned,w.total_spent,w.updated_at FROM public.accounts a JOIN public.wallets w ON w.user_uid=a.firebase_uid WHERE a.firebase_uid=$1`, uid).
		Scan(&status, &w.Balance, &w.TotalEarned, &w.TotalSpent, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, service.ErrAccountNotFound
	}
	if err == nil && status != "active" {
		return entity.Wallet{}, service.ErrAccountForbidden
	}
	return w, err
}

func (r *WalletRepository) PointHistory(ctx context.Context, uid string, f service.HistoryFilter, limit int, after *service.Cursor) ([]entity.PointHistoryEntry, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return nil, err
	}
	return pointHistoryTx(ctx, tx, uid, f, limit, after)
}

// pointHistoryTx lists ledger entries newest first; shared by the history
// endpoint and the home card so both read the same rows the same way.
func pointHistoryTx(ctx context.Context, q rowQuerier, uid string, f service.HistoryFilter, limit int, after *service.Cursor) ([]entity.PointHistoryEntry, error) {
	sql, args := pointHistoryQuery(uid, f, limit, after)
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return scanPointHistory(rows)
}

// pointHistoryQuery builds the ledger page query; also queued by the home batch.
func pointHistoryQuery(uid string, f service.HistoryFilter, limit int, after *service.Cursor) (string, []any) {
	at, id := cursorArgs(after)
	return `SELECT l.id::text,l.amount,l.source::text,l.created_at,d.id::text,d.partner_id::text,p.business_name,d.total_weight_grams,rd.id::text,rd.reward_name
 FROM public.point_ledger l LEFT JOIN public.deposits d ON d.id=l.deposit_id LEFT JOIN public.partners p ON p.id=d.partner_id
 LEFT JOIN public.redemptions rd ON rd.id=l.redemption_id
 WHERE l.user_uid=$1 AND ($2::timestamptz IS NULL OR (l.created_at,l.id)<($2,$3::uuid))
 AND ($5::text IS NULL OR l.source::text=$5) AND ($6::timestamptz IS NULL OR l.created_at>=$6) AND ($7::timestamptz IS NULL OR l.created_at<$7)
 ORDER BY l.created_at DESC,l.id DESC LIMIT $4`, []any{uid, at, id, limit, f.Kind, f.From, f.To}
}

// scanPointHistory drains and closes rows.
func scanPointHistory(rows pgx.Rows) ([]entity.PointHistoryEntry, error) {
	defer rows.Close()
	out := []entity.PointHistoryEntry{}
	for rows.Next() {
		var e entity.PointHistoryEntry
		var depositID, partnerID, partnerName, redemptionID, rewardName *string
		var grams *int64
		if err := rows.Scan(&e.ID, &e.Amount, &e.Source, &e.CreatedAt, &depositID, &partnerID, &partnerName, &grams, &redemptionID, &rewardName); err != nil {
			return nil, err
		}
		if depositID != nil {
			e.Deposit = &entity.PointHistoryDeposit{ID: *depositID, PartnerID: *partnerID, PartnerName: *partnerName, TotalWeightKG: service.FormatWeightKG(*grams)}
		}
		if redemptionID != nil {
			e.Redemption = &entity.PointHistoryRedemption{ID: *redemptionID, RewardName: *rewardName}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *WalletRepository) PartnerDeposits(ctx context.Context, uid string, f service.HistoryFilter, limit int, after *service.Cursor) ([]entity.PartnerDepositSummary, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer rollbackLater(ctx, tx)
	partner, err := partnerReaderTx(ctx, tx, uid)
	if err != nil {
		return nil, err
	}
	at, id := cursorArgs(after)
	rows, err := tx.Query(ctx, `SELECT d.id::text,d.status::text,d.total_weight_grams,d.total_points,d.uses_demo_rates,d.created_at,c.claimed_at
 FROM public.deposits d LEFT JOIN public.deposit_claims c ON c.deposit_id=d.id
 WHERE d.partner_id=$1::uuid AND ($2::text IS NULL OR d.status::text=$2)
 AND ($3::timestamptz IS NULL OR (d.created_at,d.id)<($3,$4::uuid))
 AND ($6::timestamptz IS NULL OR d.created_at>=$6) AND ($7::timestamptz IS NULL OR d.created_at<$7)
 ORDER BY d.created_at DESC,d.id DESC LIMIT $5`, partner, f.Kind, at, id, limit, f.From, f.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entity.PartnerDepositSummary{}
	for rows.Next() {
		var d entity.PartnerDepositSummary
		var grams int64
		if err = rows.Scan(&d.ID, &d.Status, &grams, &d.TotalPoints, &d.UsesDemoRates, &d.CreatedAt, &d.ClaimedAt); err != nil {
			return nil, err
		}
		d.TotalWeightKG = service.FormatWeightKG(grams)
		out = append(out, d)
	}
	return out, rows.Err()
}

// ClaimedDeposit lets the claiming user read the snapshot of a deposit they
// were credited for; anyone else gets not-found.
func (r *WalletRepository) ClaimedDeposit(ctx context.Context, uid, depositID string) (service.DepositRecord, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return service.DepositRecord{}, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return service.DepositRecord{}, err
	}
	var claimed bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.deposit_claims WHERE deposit_id=$1::uuid AND claimer_uid=$2)`, depositID, uid).Scan(&claimed); err != nil {
		return service.DepositRecord{}, err
	}
	if !claimed {
		return service.DepositRecord{}, service.ErrDepositNotFound
	}
	return loadDeposit(ctx, tx, depositID, nil)
}

// partnerReaderTx lets an active mitra read its own deposit history even when
// the location is suspended (D-21 read-only); writes still need
// authorizeMitraTx.
func partnerReaderTx(ctx context.Context, tx pgx.Tx, uid string) (string, error) {
	if err := favoriteAccount(ctx, tx, uid); err != nil {
		return "", err
	}
	var partner string
	err := tx.QueryRow(ctx, `SELECT p.id::text FROM public.partners p JOIN public.accounts a ON a.firebase_uid=p.mitra_uid WHERE p.mitra_uid=$1 AND a.role='mitra'`, uid).Scan(&partner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", service.ErrDepositForbidden
	}
	return partner, err
}
