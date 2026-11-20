-- +goose Up
CREATE TABLE public.point_rates (
 id UUID PRIMARY KEY,
 category_id UUID NOT NULL REFERENCES public.waste_categories(id),
 partner_id UUID REFERENCES public.partners(id),
 points_per_kg NUMERIC(12,2) NOT NULL CHECK(points_per_kg>0 AND points_per_kg<=1000000),
 active_from TIMESTAMPTZ NOT NULL,
 active_until TIMESTAMPTZ,
 is_demo BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(active_until IS NULL OR active_until>active_from)
);
CREATE UNIQUE INDEX point_rates_global_version_idx ON public.point_rates(category_id,active_from) WHERE partner_id IS NULL;
CREATE UNIQUE INDEX point_rates_partner_version_idx ON public.point_rates(partner_id,category_id,active_from) WHERE partner_id IS NOT NULL;
ALTER TABLE public.point_rates ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.point_rates FROM anon,authenticated;
-- +goose Down
DROP TABLE public.point_rates;
