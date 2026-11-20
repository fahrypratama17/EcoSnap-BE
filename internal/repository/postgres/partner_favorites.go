package postgres

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type FavoriteRepository struct{ pool *pgxpool.Pool }

func NewFavoriteRepository(pool *pgxpool.Pool) *FavoriteRepository {
	return &FavoriteRepository{pool: pool}
}
func favoriteAccount(ctx context.Context, tx pgx.Tx, uid string) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT status::text FROM public.accounts WHERE firebase_uid=$1 FOR SHARE`, uid).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrAccountNotFound
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return service.ErrAccountForbidden
	}
	return nil
}
func rollbackFavorite(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}
func (r *FavoriteRepository) Set(ctx context.Context, uid, id string, on bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer rollbackFavorite(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return err
	}
	if on {
		var visible string
		err = tx.QueryRow(ctx, `SELECT p.id::text FROM public.partners p JOIN public.accounts a ON a.firebase_uid=p.mitra_uid WHERE p.id=$1::uuid AND p.status='verified' AND a.status='active' AND a.role='mitra' FOR SHARE OF p,a`, id).Scan(&visible)
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ErrPartnerNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.partner_favorites(user_uid,partner_id) VALUES($1,$2::uuid) ON CONFLICT(user_uid,partner_id) DO NOTHING`, uid, id)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM public.partner_favorites WHERE user_uid=$1 AND partner_id=$2::uuid`, uid, id)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (r *FavoriteRepository) List(ctx context.Context, uid string, limit, offset int) ([]entity.FavoritePartner, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackFavorite(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT f.partner_id::text,f.created_at FROM public.partner_favorites f JOIN public.partners p ON p.id=f.partner_id JOIN public.accounts a ON a.firebase_uid=p.mitra_uid WHERE f.user_uid=$1 AND p.status='verified' AND a.status='active' AND a.role='mitra' ORDER BY f.created_at DESC,f.partner_id LIMIT $2 OFFSET $3`, uid, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []entity.FavoritePartner{}
	for rows.Next() {
		var p entity.FavoritePartner
		if err = rows.Scan(&p.PartnerID, &p.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}
