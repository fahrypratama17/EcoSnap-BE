-- +goose Up
-- Registration details are not a verified partner location or permission to issue QR.
CREATE TABLE public.partner_registrations (
 user_uid VARCHAR(128) PRIMARY KEY REFERENCES public.accounts(firebase_uid),
 business_name VARCHAR(150) NOT NULL CHECK (length(btrim(business_name)) > 0),
 business_category VARCHAR(80) NOT NULL CHECK (length(btrim(business_category)) > 0),
 status VARCHAR(24) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE public.partner_registrations ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.partner_registrations FROM anon, authenticated;

-- +goose Down
DROP TABLE public.partner_registrations;
