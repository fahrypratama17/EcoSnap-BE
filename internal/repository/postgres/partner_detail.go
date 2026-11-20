package postgres

import (
	"context"
	"errors"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
)

func (r *PartnerRepository) Detail(ctx context.Context, id string) (entity.PartnerDetail, error) {
	p, err := scanPartner(r.pool.QueryRow(ctx, partnerSelect+`AND p.id=$1::uuid`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, service.ErrPartnerNotFound
	}
	return p, err
}
