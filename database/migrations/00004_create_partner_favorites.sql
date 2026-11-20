-- +goose Up
CREATE TABLE public.partner_favorites (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 user_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 partner_id UUID NOT NULL REFERENCES public.partners(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(user_uid,partner_id)
);
CREATE INDEX partner_favorites_listing_idx ON public.partner_favorites(user_uid,created_at DESC,partner_id);
ALTER TABLE public.partner_favorites ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.partner_favorites FROM anon,authenticated;
-- +goose Down
DROP TABLE public.partner_favorites;
