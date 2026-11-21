package postgres

import (
	"context"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ActivityReportRepository struct{ pool *pgxpool.Pool }

func NewActivityReportRepository(pool *pgxpool.Pool) *ActivityReportRepository {
	return &ActivityReportRepository{pool: pool}
}

// wibDay buckets a timestamptz into its Asia/Jakarta calendar date. WIB is a
// fixed UTC+7 offset, so no time zone database is needed.
const wibDay = `((c.claimed_at AT TIME ZONE 'UTC') + interval '7 hours')::date::text`

// ActivityReport reads account, daily sums and categories in one
// REPEATABLE READ snapshot, so status, ownership and every figure agree.
// Sums run over deposit_claims joined to deposits only (one row per deposit);
// categories use a separate query over the immutable deposit_items snapshot,
// so multi-item deposits never multiply the totals.
func (r *ActivityReportRepository) ActivityReport(ctx context.Context, uid string, from, to time.Time) (service.ActivityReportRecord, error) {
	var rec service.ActivityReportRecord
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return rec, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return rec, err
	}
	if err = tx.QueryRow(ctx, `SELECT role::text,full_name FROM public.accounts WHERE firebase_uid=$1`, uid).Scan(&rec.Role, &rec.FullName); err != nil {
		return rec, err
	}
	// scope selects the claims that belong to the caller.
	var scope string
	var arg string
	switch rec.Role {
	case "user":
		scope, arg = `c.claimer_uid=$1`, uid
	case "mitra":
		partner, err := partnerReaderTx(ctx, tx, uid)
		if err != nil {
			return rec, err
		}
		var name string
		if err = tx.QueryRow(ctx, `SELECT business_name FROM public.partners WHERE id=$1::uuid`, partner).Scan(&name); err != nil {
			return rec, err
		}
		rec.BusinessName = &name
		scope, arg = `d.partner_id=$1::uuid`, partner
	default:
		return rec, service.ErrAccountForbidden
	}
	claims := ` FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id WHERE ` + scope + ` AND c.claimed_at>=$2 AND c.claimed_at<$3`

	rows, err := tx.Query(ctx, `SELECT `+wibDay+`,count(*),COALESCE(sum(d.total_weight_grams),0)::bigint,COALESCE(sum(c.points),0)::bigint`+claims+` GROUP BY 1 ORDER BY 1`, arg, from, to)
	if err != nil {
		return rec, err
	}
	for rows.Next() {
		var d service.ActivityDayRecord
		if err = rows.Scan(&d.Date, &d.Claimed, &d.Grams, &d.Points); err != nil {
			rows.Close()
			return rec, err
		}
		rec.Days = append(rec.Days, d)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return rec, err
	}

	if rec.Role == "user" {
		rows, err = tx.Query(ctx, `SELECT ((l.created_at AT TIME ZONE 'UTC') + interval '7 hours')::date::text,(-sum(l.amount))::bigint
 FROM public.point_ledger l WHERE l.user_uid=$1 AND l.source='redemption' AND l.created_at>=$2 AND l.created_at<$3 GROUP BY 1 ORDER BY 1`, uid, from, to)
		if err != nil {
			return rec, err
		}
		for rows.Next() {
			var d service.ActivityDayRecord
			if err = rows.Scan(&d.Date, &d.Spent); err != nil {
				rows.Close()
				return rec, err
			}
			rec.Days = append(rec.Days, d)
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			return rec, err
		}
	}

	// The name is the one snapshotted on the items (newest claim wins when a
	// category was renamed between deposits); live waste_categories is not read.
	rows, err = tx.Query(ctx, `SELECT i.category_id::text,
 (array_agg(i.category_name ORDER BY c.claimed_at DESC,d.id DESC))[1],
 count(*),COALESCE(sum(i.weight_grams),0)::bigint
 FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id JOIN public.deposit_items i ON i.deposit_id=d.id
 WHERE `+scope+` AND c.claimed_at>=$2 AND c.claimed_at<$3
 GROUP BY i.category_id ORDER BY sum(i.weight_grams) DESC,2,1`, arg, from, to)
	if err != nil {
		return rec, err
	}
	for rows.Next() {
		var c service.ActivityCategoryRecord
		if err = rows.Scan(&c.ID, &c.Name, &c.Deposits, &c.Gram); err != nil {
			rows.Close()
			return rec, err
		}
		rec.Categories = append(rec.Categories, c)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return rec, err
	}
	return rec, tx.Commit(ctx)
}
