-- +goose Up
CREATE TYPE public.partner_status AS ENUM ('pending','verified','rejected','suspended');
CREATE TABLE public.waste_categories (
 id UUID PRIMARY KEY, name VARCHAR(100) NOT NULL, slug VARCHAR(100) NOT NULL UNIQUE,
 handling_guide TEXT NOT NULL DEFAULT '', requires_special_handling BOOLEAN NOT NULL DEFAULT false
);
CREATE TABLE public.partners (
 id UUID PRIMARY KEY, mitra_uid VARCHAR(128) NOT NULL UNIQUE REFERENCES public.accounts(firebase_uid),
 business_name VARCHAR(150) NOT NULL, business_category VARCHAR(80), description TEXT NOT NULL DEFAULT '',
 area VARCHAR(150) NOT NULL, address TEXT NOT NULL CHECK(length(btrim(address))>0),
 latitude NUMERIC(10,7) NOT NULL CHECK(latitude BETWEEN -90 AND 90),
 longitude NUMERIC(10,7) NOT NULL CHECK(longitude BETWEEN -180 AND 180),
 phone VARCHAR(32) NOT NULL DEFAULT '', opening_hours VARCHAR(200) NOT NULL DEFAULT '',
 status public.partner_status NOT NULL DEFAULT 'pending', created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE public.partner_hours (
 id UUID PRIMARY KEY, partner_id UUID NOT NULL REFERENCES public.partners(id),
 day_of_week SMALLINT NOT NULL CHECK(day_of_week BETWEEN 0 AND 6),
 opens_at TIME, closes_at TIME, is_closed BOOLEAN NOT NULL DEFAULT false,
 UNIQUE(partner_id,day_of_week),
 CHECK((is_closed AND opens_at IS NULL AND closes_at IS NULL) OR
 (NOT is_closed AND opens_at IS NOT NULL AND closes_at IS NOT NULL AND closes_at>opens_at))
);
CREATE TABLE public.partner_accepted_waste (
 id UUID PRIMARY KEY, partner_id UUID NOT NULL REFERENCES public.partners(id),
 category_id UUID NOT NULL REFERENCES public.waste_categories(id), notes TEXT,
 UNIQUE(partner_id,category_id)
);
CREATE INDEX partner_accepted_waste_category_idx ON public.partner_accepted_waste(category_id,partner_id);
ALTER TABLE public.waste_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.partners ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.partner_hours ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.partner_accepted_waste ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.waste_categories,public.partners,public.partner_hours,public.partner_accepted_waste FROM anon,authenticated;
-- +goose Down
DROP TABLE public.partner_accepted_waste;
DROP TABLE public.partner_hours;
DROP TABLE public.partners;
DROP TABLE public.waste_categories;
DROP TYPE public.partner_status;
