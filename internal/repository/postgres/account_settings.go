package postgres

import (
	"context"
	"errors"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepository struct{ pool *pgxpool.Pool }

func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

func (r *SettingsRepository) Settings(ctx context.Context, uid string) (service.SettingsSnapshot, error) {
	var out service.SettingsSnapshot
	err := r.pool.QueryRow(ctx, `SELECT a.status::text,s.notification_enabled,s.show_on_leaderboard,s.onboarding_completed
 FROM public.accounts a JOIN public.user_settings s ON s.firebase_uid=a.firebase_uid WHERE a.firebase_uid=$1`, uid).Scan(
		&out.Status, &out.Settings.NotificationEnabled, &out.Settings.ShowOnLeaderboard, &out.Settings.OnboardingCompleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, service.ErrAccountNotFound
	}
	return out, err
}

func (r *SettingsRepository) UpdateSettings(ctx context.Context, uid string, in service.AccountSettingsUpdate) (entity.AccountSettings, error) {
	var out entity.AccountSettings
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, err
	}
	defer rollbackLater(ctx, tx)
	var status string
	err = tx.QueryRow(ctx, `SELECT status::text FROM public.accounts WHERE firebase_uid=$1 FOR SHARE`, uid).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, service.ErrAccountNotFound
	}
	if err != nil {
		return out, err
	}
	if status != "active" {
		return out, service.ErrAccountForbidden
	}
	err = tx.QueryRow(ctx, `UPDATE public.user_settings SET
 notification_enabled=COALESCE($2::boolean,notification_enabled),
 onboarding_completed=COALESCE($3::boolean,onboarding_completed)
 WHERE firebase_uid=$1
 RETURNING notification_enabled,show_on_leaderboard,onboarding_completed`, uid, in.NotificationEnabled, in.OnboardingCompleted).Scan(
		&out.NotificationEnabled, &out.ShowOnLeaderboard, &out.OnboardingCompleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.AccountSettings{}, service.ErrAccountNotFound
	}
	if err != nil {
		return entity.AccountSettings{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return entity.AccountSettings{}, err
	}
	return out, nil
}
