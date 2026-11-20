package service

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrInvalidPartnerQuery = errors.New("parameter mitra tidak valid")
var ErrPartnerNotFound = errors.New("mitra tidak ditemukan")
var materialSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var partnerID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type PartnerFilter struct {
	Query, Material string
	Limit, Offset   int
}
type PartnerPage struct {
	Items         []entity.PartnerDetail
	HasMore       bool
	Limit, Offset int
}
type PartnerStore interface {
	List(context.Context, PartnerFilter) ([]entity.PartnerDetail, error)
	Detail(context.Context, string) (entity.PartnerDetail, error)
	Categories(context.Context) ([]entity.WasteCategory, error)
}
type PartnerService struct{ store PartnerStore }

func NewPartnerService(store PartnerStore) *PartnerService { return &PartnerService{store: store} }
func (s *PartnerService) List(ctx context.Context, f PartnerFilter) (PartnerPage, error) {
	f.Query = strings.TrimSpace(f.Query)
	f.Material = strings.ToLower(strings.TrimSpace(f.Material))
	if f.Limit == 0 {
		f.Limit = 20
	}
	if !utf8.ValidString(f.Query) || utf8.RuneCountInString(f.Query) > 100 || len(f.Material) > 100 || (f.Material != "" && !materialSlug.MatchString(f.Material)) || f.Limit < 1 || f.Limit > 100 || f.Offset < 0 || f.Offset > 10000 {
		return PartnerPage{}, ErrInvalidPartnerQuery
	}
	requested := f.Limit
	f.Limit++
	items, err := s.store.List(ctx, f)
	if err != nil {
		return PartnerPage{}, err
	}
	more := len(items) > requested
	if more {
		items = items[:requested]
	}
	if items == nil {
		items = []entity.PartnerDetail{}
	}
	return PartnerPage{Items: items, HasMore: more, Limit: requested, Offset: f.Offset}, nil
}
func (s *PartnerService) Categories(ctx context.Context) ([]entity.WasteCategory, error) {
	items, err := s.store.Categories(ctx)
	if items == nil && err == nil {
		items = []entity.WasteCategory{}
	}
	return items, err
}
