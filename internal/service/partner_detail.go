package service

import (
	"context"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"strings"
)

func (s *PartnerService) Detail(ctx context.Context, id string) (entity.PartnerDetail, error) {
	if !partnerID.MatchString(id) {
		return entity.PartnerDetail{}, ErrInvalidPartnerQuery
	}
	return s.store.Detail(ctx, strings.ToLower(id))
}
