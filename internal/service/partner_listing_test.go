package service_test

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"strings"
	"testing"
)

type partnerData struct{ items []entity.PartnerDetail }

func (p partnerData) List(_ context.Context, f service.PartnerFilter) ([]entity.PartnerDetail, error) {
	if f.Query != "Malang" || f.Material != "plastic" {
		return nil, errors.New("filters were not normalized")
	}
	return p.items, nil
}
func (partnerData) Detail(context.Context, string) (entity.PartnerDetail, error) {
	return entity.PartnerDetail{}, service.ErrPartnerNotFound
}
func (partnerData) Categories(context.Context) ([]entity.WasteCategory, error) { return nil, nil }
func TestPartnerPaginationAndEmptyResult(t *testing.T) {
	for _, items := range [][]entity.PartnerDetail{nil, {{Name: "Tunas"}, {Name: "Lestari"}, {Name: "Hijau"}}} {
		s := service.NewPartnerService(partnerData{items: items})
		page, err := s.List(context.Background(), service.PartnerFilter{Query: " Malang ", Material: " PLASTIC ", Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		if items == nil {
			if page.Items == nil || len(page.Items) != 0 || page.HasMore {
				t.Fatalf("wrong empty page: %+v", page)
			}
		} else if len(page.Items) != 2 || !page.HasMore || page.Items[0].Name != "Tunas" {
			t.Fatalf("wrong page: %+v", page)
		}
	}
}
func TestPartnerRejectsInvalidInputBeforeStorage(t *testing.T) {
	s := service.NewPartnerService(partnerData{})
	for _, filter := range []service.PartnerFilter{{Limit: -1}, {Limit: 101}, {Offset: -1}, {Offset: 10001}, {Query: strings.Repeat("x", 101)}, {Material: "plastic'"}} {
		if _, err := s.List(context.Background(), filter); !errors.Is(err, service.ErrInvalidPartnerQuery) {
			t.Fatalf("invalid filter: %+v %v", filter, err)
		}
	}
	if _, err := s.Detail(context.Background(), "tunas"); !errors.Is(err, service.ErrInvalidPartnerQuery) {
		t.Fatalf("invalid ID: %v", err)
	}
	if _, err := s.Detail(context.Background(), "20000000-0000-4000-8000-000000000001"); !errors.Is(err, service.ErrPartnerNotFound) {
		t.Fatalf("missing ID: %v", err)
	}
}
