package postgres

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountRepository struct{ pool *pgxpool.Pool }

func NewAccountRepository(pool *pgxpool.Pool) *AccountRepository {
	return &AccountRepository{pool: pool}
}

type accountQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func readProfile(ctx context.Context, q accountQuerier, uid string) (entity.AccountProfile, error) {
	var p entity.AccountProfile
	var name, category, status *string
	err := q.QueryRow(ctx, `SELECT a.firebase_uid,a.role::text,a.full_name,a.display_name,COALESCE(a.email,''),COALESCE(a.phone,''),a.status::text,a.created_at,
 w.balance,w.total_earned,w.total_spent,s.notification_enabled,s.show_on_leaderboard,s.onboarding_completed,
 b.business_name,b.business_category,b.status
 FROM public.accounts a JOIN public.wallets w ON w.user_uid=a.firebase_uid
 JOIN public.user_settings s ON s.firebase_uid=a.firebase_uid
 LEFT JOIN public.partner_registrations b ON b.user_uid=a.firebase_uid WHERE a.firebase_uid=$1`, uid).Scan(
		&p.UID, &p.Role, &p.FullName, &p.DisplayName, &p.Email, &p.Phone, &p.Status, &p.CreatedAt,
		&p.Wallet.Balance, &p.Wallet.TotalEarned, &p.Wallet.TotalSpent, &p.Settings.NotificationEnabled, &p.Settings.ShowOnLeaderboard, &p.Settings.OnboardingCompleted,
		&name, &category, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, service.ErrAccountNotFound
	}
	if err != nil {
		return p, err
	}
	if name != nil {
		p.Business = &entity.BusinessRegistration{Name: *name, Category: *category, Status: *status}
	}
	return p, nil
}
func (r *AccountRepository) Profile(ctx context.Context, uid string) (entity.AccountProfile, error) {
	return readProfile(ctx, r.pool, uid)
}
