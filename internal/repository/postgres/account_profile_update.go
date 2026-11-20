package postgres

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"time"
)

func (r *AccountRepository) UpdateProfile(ctx context.Context, uid string, in service.AccountProfileUpdate) (entity.AccountProfile, error) {
	var empty entity.AccountProfile
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var status string
	err = tx.QueryRow(ctx, `SELECT status::text FROM public.accounts WHERE firebase_uid=$1 FOR UPDATE`, uid).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, service.ErrAccountNotFound
	}
	if err != nil {
		return empty, err
	}
	if status != "active" {
		return empty, service.ErrAccountForbidden
	}
	var display *string
	if in.FullName != nil {
		runes := []rune(*in.FullName)
		if len(runes) > 80 {
			runes = runes[:80]
		}
		value := string(runes)
		display = &value
	}
	_, err = tx.Exec(ctx, `UPDATE public.accounts SET full_name=COALESCE($2::text,full_name),display_name=COALESCE($3::text,display_name),phone=COALESCE($4::text,phone) WHERE firebase_uid=$1`, uid, in.FullName, display, in.Phone)
	if err != nil {
		return empty, err
	}
	p, err := readProfile(ctx, tx, uid)
	if err != nil {
		return empty, err
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, err
	}
	return p, nil
}
