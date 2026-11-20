package service

import (
	"context"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
)

func (s *AccountService) Profile(ctx context.Context, uid string) (entity.AccountProfile, error) {
	p, err := s.store.Profile(ctx, uid)
	if err != nil {
		return entity.AccountProfile{}, err
	}
	if p.Status != "active" {
		return entity.AccountProfile{}, ErrAccountForbidden
	}
	return p, nil
}
