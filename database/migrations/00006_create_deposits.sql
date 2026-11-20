-- +goose Up
-- created -> qr_issued (locked, D-19) -> claimed. Reissue keeps qr_issued.
-- Migration belum diterapkan ke Supabase saat status ditambahkan.
CREATE TYPE public.deposit_status AS ENUM ('created','qr_issued','claimed');
CREATE TABLE public.deposits (
 id UUID PRIMARY KEY,
 partner_id UUID NOT NULL REFERENCES public.partners(id),
 created_by_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 idempotency_key VARCHAR(128) NOT NULL CHECK(idempotency_key ~ '^[A-Za-z0-9_-]{8,128}$'),
 request_hash CHAR(64) NOT NULL,
 status public.deposit_status NOT NULL DEFAULT 'created',
 total_weight_grams BIGINT NOT NULL CHECK(total_weight_grams BETWEEN 1 AND 1000000),
 total_points BIGINT NOT NULL CHECK(total_points >= 0),
 uses_demo_rates BOOLEAN NOT NULL,
 rates_as_of TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(partner_id, idempotency_key)
);
CREATE INDEX deposits_partner_created_idx ON public.deposits(partner_id, created_at DESC);
-- Snapshot of the tariff applied when the deposit was created. Later tariff
-- edits must not change the history, so values are copied, not joined.
CREATE TABLE public.deposit_items (
 deposit_id UUID NOT NULL REFERENCES public.deposits(id) ON DELETE CASCADE,
 line_no SMALLINT NOT NULL CHECK(line_no BETWEEN 1 AND 50),
 category_id UUID NOT NULL REFERENCES public.waste_categories(id),
 category_name VARCHAR(100) NOT NULL,
 weight_grams BIGINT NOT NULL CHECK(weight_grams BETWEEN 1 AND 1000000),
 rate_id UUID NOT NULL REFERENCES public.point_rates(id),
 rate_hundredths BIGINT NOT NULL CHECK(rate_hundredths BETWEEN 1 AND 100000000),
 exact_points_e5 BIGINT NOT NULL CHECK(exact_points_e5 = weight_grams * rate_hundredths),
 is_demo BOOLEAN NOT NULL,
 PRIMARY KEY(deposit_id, line_no),
 UNIQUE(deposit_id, category_id)
);
ALTER TABLE public.deposits ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.deposit_items ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.deposits, public.deposit_items FROM anon, authenticated;
-- +goose Down
DROP TABLE public.deposit_items;
DROP TABLE public.deposits;
DROP TYPE public.deposit_status;
