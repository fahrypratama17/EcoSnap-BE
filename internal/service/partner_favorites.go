package service

import (
	"context"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"strings"
)

type FavoriteStore interface {
	Set(context.Context, string, string, bool) error
	List(context.Context, string, int, int) ([]entity.FavoritePartner, error)
}
type FavoriteService struct{ store FavoriteStore }
type FavoritePage struct {
	Items         []entity.FavoritePartner
	Limit, Offset int
	HasMore       bool
}

func NewFavoriteService(store FavoriteStore) *FavoriteService { return &FavoriteService{store: store} }
func (s *FavoriteService) Set(ctx context.Context, uid, id string, on bool) error {
	if uid == "" {
		return ErrAccountNotFound
	}
	if !partnerID.MatchString(id) {
		return ErrInvalidPartnerQuery
	}
	return s.store.Set(ctx, uid, strings.ToLower(id), on)
}
func (s *FavoriteService) List(ctx context.Context, uid string, limit, offset int) (FavoritePage, error) {
	if uid == "" {
		return FavoritePage{}, ErrAccountNotFound
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return FavoritePage{}, ErrInvalidPartnerQuery
	}
	items, err := s.store.List(ctx, uid, limit+1, offset)
	if err != nil {
		return FavoritePage{}, err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	if items == nil {
		items = []entity.FavoritePartner{}
	}
	return FavoritePage{Items: items, Limit: limit, Offset: offset, HasMore: more}, nil
}
