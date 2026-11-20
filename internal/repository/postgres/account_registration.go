package postgres

import (
	"context"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"time"
)

func (r *AccountRepository) Register(ctx context.Context, uid, email string, in service.AccountRegistration) (entity.AccountProfile, bool, error) {
	var empty entity.AccountProfile
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return empty, false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	display := []rune(in.FullName)
	if len(display) > 80 {
		display = display[:80]
	}
	tag, err := tx.Exec(ctx, `INSERT INTO public.accounts(firebase_uid,role,full_name,display_name,email,phone)
 VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(firebase_uid) DO NOTHING`, uid, in.Role, in.FullName, string(display), email, in.Phone)
	if err != nil {
		return empty, false, err
	}
	created := tag.RowsAffected() == 1
	if created {
		if _, err = tx.Exec(ctx, `INSERT INTO public.user_settings(firebase_uid) VALUES($1)`, uid); err != nil {
			return empty, false, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.wallets(user_uid) VALUES($1)`, uid); err != nil {
			return empty, false, err
		}
		if in.Role == "mitra" {
			if _, err = tx.Exec(ctx, `INSERT INTO public.partner_registrations(user_uid,business_name,business_category) VALUES($1,$2,$3)`, uid, in.BusinessName, in.BusinessCategory); err != nil {
				return empty, false, err
			}
		}
	}
	p, err := readProfile(ctx, tx, uid)
	if err != nil {
		return empty, false, err
	}
	if p.Status != "active" {
		return empty, false, service.ErrAccountForbidden
	}
	if !created {
		same := p.Role == in.Role && p.FullName == in.FullName && p.Phone == in.Phone
		if in.Role == "mitra" {
			same = same && p.Business != nil && p.Business.Name == in.BusinessName && p.Business.Category == in.BusinessCategory
		}
		if !same {
			return empty, false, service.ErrAccountConflict
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return empty, false, err
	}
	return p, created, nil
}
