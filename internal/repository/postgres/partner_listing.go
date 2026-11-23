package postgres

import (
	"context"
	"encoding/json"
	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PartnerRepository struct{ pool *pgxpool.Pool }

func NewPartnerRepository(pool *pgxpool.Pool) *PartnerRepository {
	return &PartnerRepository{pool: pool}
}

const partnerSelect = `SELECT p.id::text,p.business_name,p.area,p.address,p.phone,p.opening_hours,p.description,p.latitude::float8,p.longitude::float8,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',c.id::text,'name',c.name,'slug',c.slug,'code',upper(c.slug),'handling_guide',c.handling_guide,'requires_special_handling',c.requires_special_handling) ORDER BY c.slug)
 FROM public.partner_accepted_waste aw JOIN public.waste_categories c ON c.id=aw.category_id WHERE aw.partner_id=p.id),'[]'::jsonb),
 COALESCE((SELECT jsonb_agg(jsonb_build_object('day_of_week',h.day_of_week,'opens_at',COALESCE(to_char(h.opens_at,'HH24:MI'),''),'closes_at',COALESCE(to_char(h.closes_at,'HH24:MI'),''),'is_closed',h.is_closed) ORDER BY h.day_of_week)
 FROM public.partner_hours h WHERE h.partner_id=p.id),'[]'::jsonb)
 FROM public.partners p JOIN public.accounts a ON a.firebase_uid=p.mitra_uid
 WHERE p.status='verified' AND a.status='active' AND a.role='mitra' `

type partnerScanner interface{ Scan(...any) error }

func scanPartner(row partnerScanner) (entity.PartnerDetail, error) {
	var p entity.PartnerDetail
	var materials, hours []byte
	err := row.Scan(&p.ID, &p.Name, &p.Area, &p.Address, &p.Phone, &p.OpeningHours, &p.Description, &p.Location.Latitude, &p.Location.Longitude, &materials, &hours)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(materials, &p.AcceptedMaterials); err != nil {
		return p, err
	}
	err = json.Unmarshal(hours, &p.Hours)
	return p, err
}
func (r *PartnerRepository) List(ctx context.Context, f service.PartnerFilter) ([]entity.PartnerDetail, error) {
	return partnersTx(ctx, r.pool, f)
}

// partnersTx lists visible partners in stable name,id order; shared by the
// partner list and the home suggestion.
func partnersTx(ctx context.Context, q rowQuerier, f service.PartnerFilter) ([]entity.PartnerDetail, error) {
	rows, err := q.Query(ctx, partnerSelect+`AND ($1='' OR strpos(lower(p.business_name),lower($1))>0 OR strpos(lower(p.area),lower($1))>0)
 AND ($2='' OR EXISTS(SELECT 1 FROM public.partner_accepted_waste aw JOIN public.waste_categories c ON c.id=aw.category_id WHERE aw.partner_id=p.id AND c.slug=$2))
 ORDER BY p.business_name,p.id LIMIT $3 OFFSET $4`, f.Query, f.Material, f.Limit, f.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []entity.PartnerDetail{}
	for rows.Next() {
		p, err := scanPartner(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (r *PartnerRepository) Categories(ctx context.Context) ([]entity.WasteCategory, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text,name,slug,upper(slug),handling_guide,requires_special_handling FROM public.waste_categories ORDER BY name,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []entity.WasteCategory{}
	for rows.Next() {
		var c entity.WasteCategory
		if err = rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Code, &c.HandlingGuide, &c.RequiresSpecialHandling); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}
