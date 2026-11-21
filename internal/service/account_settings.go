package service

import (
	"context"
	"errors"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

var ErrInvalidSettingsUpdate = errors.New("perubahan pengaturan tidak valid")

// AccountSettingsUpdate carries only the fields a client may change; nil means
// "leave unchanged".
type AccountSettingsUpdate struct {
	NotificationEnabled *bool
	OnboardingCompleted *bool
}

// SettingsSnapshot is one consistent read of the account status and its settings.
type SettingsSnapshot struct {
	Status   string
	Settings entity.AccountSettings
}

type SettingsStore interface {
	// Settings reads status and settings in one statement. A missing account or
	// settings row must be reported as ErrAccountNotFound, never defaulted.
	Settings(ctx context.Context, uid string) (SettingsSnapshot, error)
	// UpdateSettings locks the account, rejects non-active accounts with
	// ErrAccountForbidden, then applies only non-nil fields.
	UpdateSettings(ctx context.Context, uid string, in AccountSettingsUpdate) (entity.AccountSettings, error)
}

type SettingsService struct{ store SettingsStore }

func NewSettingsService(store SettingsStore) *SettingsService { return &SettingsService{store: store} }

func (s *SettingsService) Get(ctx context.Context, uid string) (entity.AccountSettings, error) {
	if uid == "" {
		return entity.AccountSettings{}, ErrAccountNotFound
	}
	snap, err := s.store.Settings(ctx, uid)
	if err != nil {
		return entity.AccountSettings{}, err
	}
	if snap.Status != "active" {
		return entity.AccountSettings{}, ErrAccountForbidden
	}
	return snap.Settings, nil
}

func (s *SettingsService) Update(ctx context.Context, uid string, in AccountSettingsUpdate) (entity.AccountSettings, error) {
	if uid == "" {
		return entity.AccountSettings{}, ErrAccountNotFound
	}
	if in.NotificationEnabled == nil && in.OnboardingCompleted == nil {
		return entity.AccountSettings{}, ErrInvalidSettingsUpdate
	}
	return s.store.UpdateSettings(ctx, uid, in)
}
