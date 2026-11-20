package postgres

import (
	"context"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PointRateRepository struct{ pool *pgxpool.Pool }

func NewPointRateRepository(pool *pgxpool.Pool) *PointRateRepository {
	return &PointRateRepository{pool: pool}
}
func (r *PointRateRepository) SelectRates(ctx context.Context, partner *string, ids []string) (service.PointRateSelection, error) {
	out := service.PointRateSelection{Rates: map[string]service.PointTariff{}}
	// Visibility, accepted materials, and tariffs share one read-only snapshot.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err = tx.QueryRow(ctx, `SELECT now()`).Scan(&out.AsOf); err != nil {
		return out, err
	}
	if partner != nil {
		var visible bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.partners p JOIN public.accounts a ON a.firebase_uid=p.mitra_uid WHERE p.id=$1::uuid AND p.status='verified' AND a.status='active' AND a.role='mitra')`, *partner).Scan(&visible)
		if err != nil {
			return out, err
		}
		if !visible {
			return out, service.ErrPartnerNotFound
		}
	}
	rows, err := tx.Query(ctx, `SELECT c.id::text,c.name,COALESCE(rate.id::text,''),COALESCE((rate.points_per_kg*100)::bigint,0),COALESCE(rate.is_demo,false),
 ($2::uuid IS NULL OR EXISTS(SELECT 1 FROM public.partner_accepted_waste aw WHERE aw.partner_id=$2::uuid AND aw.category_id=c.id))
 FROM public.waste_categories c
 LEFT JOIN LATERAL(
 SELECT r.id,r.points_per_kg,r.is_demo FROM public.point_rates r
 WHERE r.category_id=c.id AND (r.partner_id IS NULL OR r.partner_id=$2::uuid)
 AND r.active_from<=$3 AND (r.active_until IS NULL OR $3<r.active_until)
 ORDER BY (r.partner_id IS NOT NULL) DESC,r.active_from DESC,r.id LIMIT 1
 ) rate ON true WHERE c.id=ANY($1::uuid[])`, ids, partner, out.AsOf)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var rate service.PointTariff
		if err = rows.Scan(&rate.CategoryID, &rate.Name, &rate.RateID, &rate.RateHundredths, &rate.IsDemo, &rate.Accepted); err != nil {
			return out, err
		}
		out.Rates[rate.CategoryID] = rate
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	if len(out.Rates) != len(ids) {
		return out, service.ErrWasteCategoryNotFound
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}
