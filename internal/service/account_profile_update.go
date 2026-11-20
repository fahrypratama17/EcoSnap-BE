package service

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"strings"
	"unicode/utf8"
)

var ErrInvalidProfileUpdate = errors.New("perubahan profil tidak valid")

type AccountProfileUpdate struct {
	FullName *string
	Phone    *string
}

func (s *AccountService) UpdateProfile(ctx context.Context, uid string, in AccountProfileUpdate) (entity.AccountProfile, error) {
	if uid == "" {
		return entity.AccountProfile{}, ErrAccountNotFound
	}
	if in.FullName == nil && in.Phone == nil {
		return entity.AccountProfile{}, ErrInvalidProfileUpdate
	}
	if in.FullName != nil {
		value := strings.TrimSpace(*in.FullName)
		if !utf8.ValidString(value) || utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 150 {
			return entity.AccountProfile{}, ErrInvalidProfileUpdate
		}
		in.FullName = &value
	}
	if in.Phone != nil {
		value := strings.TrimSpace(*in.Phone)
		if !validPhone(value) {
			return entity.AccountProfile{}, ErrInvalidProfileUpdate
		}
		in.Phone = &value
	}
	p, err := s.store.UpdateProfile(ctx, uid, in)
	if err != nil {
		return entity.AccountProfile{}, err
	}
	if p.Status != "active" {
		return entity.AccountProfile{}, ErrAccountForbidden
	}
	return p, nil
}
