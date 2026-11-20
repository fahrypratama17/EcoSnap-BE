package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DepositRepository struct{ pool *pgxpool.Pool }

func NewDepositRepository(pool *pgxpool.Pool) *DepositRepository {
	return &DepositRepository{pool: pool}
}

type depositQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *DepositRepository) CreateDeposit(ctx context.Context, in service.NewDeposit, price service.DepositPricer) (service.DepositRecord, bool, error) {
	var out service.DepositRecord
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	// Share locks keep the account and partner from being suspended between
	// this authorization check and the insert.
	var role, status string
	err = tx.QueryRow(ctx, `SELECT role::text,status::text FROM public.accounts WHERE firebase_uid=$1 FOR SHARE`, in.CreatorUID).Scan(&role, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, service.ErrAccountNotFound
	}
	if err != nil {
		return out, false, err
	}
	if status != "active" {
		return out, false, service.ErrAccountForbidden
	}
	var partner, partnerStatus string
	err = tx.QueryRow(ctx, `SELECT id::text,status::text FROM public.partners WHERE mitra_uid=$1 FOR SHARE`, in.CreatorUID).Scan(&partner, &partnerStatus)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (role != "mitra" || partnerStatus != "verified")) {
		return out, false, service.ErrDepositForbidden
	}
	if err != nil {
		return out, false, err
	}
	// Serialize concurrent retries of one key; the unique constraint remains
	// the final guard.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1||':'||$2,0))`, partner, in.IdempotencyKey); err != nil {
		return out, false, err
	}
	var existing, existingHash string
	err = tx.QueryRow(ctx, `SELECT id::text,request_hash FROM public.deposits WHERE partner_id=$1::uuid AND idempotency_key=$2`, partner, in.IdempotencyKey).Scan(&existing, &existingHash)
	if err == nil {
		if existingHash != in.RequestHash {
			return out, false, service.ErrDepositConflict
		}
		out, err = loadDeposit(ctx, tx, existing, in.CreatorUID)
		return out, false, err
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, false, err
	}
	selection := service.PointRateSelection{}
	if err = tx.QueryRow(ctx, `SELECT now()`).Scan(&selection.AsOf); err != nil {
		return out, false, err
	}
	ids := make([]string, len(in.Lines))
	for i, line := range in.Lines {
		ids[i] = line.CategoryID
	}
	if selection.Rates, err = selectRatesTx(ctx, tx, &partner, ids, selection.AsOf); err != nil {
		return out, false, err
	}
	estimate, err := price(selection)
	if err != nil {
		return out, false, err
	}
	out = service.DepositRecord{PartnerID: partner, TotalGrams: in.TotalGrams, TotalPoint: estimate.EstimatedPoints, UsesDemoRates: estimate.UsesDemoRates, RatesAsOf: selection.AsOf, Items: make([]service.DepositRecordItem, len(in.Lines))}
	err = tx.QueryRow(ctx, `INSERT INTO public.deposits(id,partner_id,created_by_uid,idempotency_key,request_hash,total_weight_grams,total_points,uses_demo_rates,rates_as_of)
 VALUES(gen_random_uuid(),$1::uuid,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text,status::text,created_at`,
		partner, in.CreatorUID, in.IdempotencyKey, in.RequestHash, in.TotalGrams, out.TotalPoint, out.UsesDemoRates, out.RatesAsOf).Scan(&out.ID, &out.Status, &out.CreatedAt)
	if err != nil {
		return out, false, err
	}
	batch := &pgx.Batch{}
	for i, line := range in.Lines {
		rate := selection.Rates[line.CategoryID]
		item := service.DepositRecordItem{CategoryID: line.CategoryID, CategoryName: rate.Name, RateID: rate.RateID, Grams: line.Grams, RateHundredths: rate.RateHundredths, IsDemo: rate.IsDemo}
		out.Items[i] = item
		batch.Queue(`INSERT INTO public.deposit_items(deposit_id,line_no,category_id,category_name,weight_grams,rate_id,rate_hundredths,exact_points_e5,is_demo)
 VALUES($1::uuid,$2,$3::uuid,$4,$5,$6::uuid,$7,$5::bigint*$7::bigint,$8)`, out.ID, i+1, item.CategoryID, item.CategoryName, item.Grams, item.RateID, item.RateHundredths, item.IsDemo)
	}
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return out, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, false, err
	}
	return out, true, nil
}

func (r *DepositRepository) DepositForMitra(ctx context.Context, uid, depositID string) (service.DepositRecord, error) {
	return loadDeposit(ctx, r.pool, depositID, uid)
}

// loadDeposit returns the deposit only when its partner belongs to uid; other
// callers get not-found so deposit IDs do not reveal another partner's data.
func loadDeposit(ctx context.Context, q depositQuerier, depositID, uid string) (service.DepositRecord, error) {
	var out service.DepositRecord
	err := q.QueryRow(ctx, `SELECT d.id::text,d.partner_id::text,d.status::text,d.total_weight_grams,d.total_points,d.uses_demo_rates,d.rates_as_of,d.created_at
 FROM public.deposits d JOIN public.partners p ON p.id=d.partner_id WHERE d.id=$1::uuid AND p.mitra_uid=$2`, depositID, uid).
		Scan(&out.ID, &out.PartnerID, &out.Status, &out.TotalGrams, &out.TotalPoint, &out.UsesDemoRates, &out.RatesAsOf, &out.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, service.ErrDepositNotFound
	}
	if err != nil {
		return out, err
	}
	rows, err := q.Query(ctx, `SELECT category_id::text,category_name,rate_id::text,weight_grams,rate_hundredths,is_demo FROM public.deposit_items WHERE deposit_id=$1::uuid ORDER BY line_no`, depositID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item service.DepositRecordItem
		if err = rows.Scan(&item.CategoryID, &item.CategoryName, &item.RateID, &item.Grams, &item.RateHundredths, &item.IsDemo); err != nil {
			return out, err
		}
		out.Items = append(out.Items, item)
	}
	return out, rows.Err()
}
