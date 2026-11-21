-- +goose Up
-- Simulated vouchers (D-08). Expiry and no-refund rules: D-20.
CREATE TABLE public.rewards (
 id UUID PRIMARY KEY,
 name VARCHAR(120) NOT NULL CHECK(length(btrim(name))>0),
 description TEXT NOT NULL DEFAULT '',
 terms TEXT NOT NULL DEFAULT '',
 cost_points INTEGER NOT NULL CHECK(cost_points BETWEEN 1 AND 100000000),
 stock INTEGER NOT NULL CHECK(stock >= 0),
 is_active BOOLEAN NOT NULL DEFAULT true,
 is_demo BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX rewards_active_cost_idx ON public.rewards(cost_points, id) WHERE is_active;
-- 'expired' is not stored: an issued voucher past expires_at is expired.
CREATE TYPE public.voucher_status AS ENUM ('issued','used');
CREATE TABLE public.redemptions (
 id UUID PRIMARY KEY,
 user_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 reward_id UUID NOT NULL REFERENCES public.rewards(id),
 idempotency_key VARCHAR(128) NOT NULL CHECK(idempotency_key ~ '^[A-Za-z0-9_-]{8,128}$'),
 reward_name VARCHAR(120) NOT NULL,
 cost_points INTEGER NOT NULL CHECK(cost_points > 0),
 voucher_code VARCHAR(20) NOT NULL UNIQUE,
 status public.voucher_status NOT NULL DEFAULT 'issued',
 created_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 used_at TIMESTAMPTZ,
 UNIQUE(user_uid, idempotency_key),
 CHECK(expires_at > created_at),
 CHECK((status='used') = (used_at IS NOT NULL)),
 CHECK(used_at IS NULL OR used_at < expires_at)
);
CREATE INDEX redemptions_user_created_idx ON public.redemptions(user_uid, created_at DESC, id DESC);
ALTER TABLE public.point_ledger ADD COLUMN redemption_id UUID UNIQUE REFERENCES public.redemptions(id);
ALTER TABLE public.point_ledger ADD CONSTRAINT point_ledger_redemption_debit
 CHECK(source <> 'redemption' OR (redemption_id IS NOT NULL AND amount < 0));
ALTER TABLE public.rewards ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.redemptions ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.rewards, public.redemptions FROM anon, authenticated;
-- +goose Down
ALTER TABLE public.point_ledger DROP CONSTRAINT point_ledger_redemption_debit;
ALTER TABLE public.point_ledger DROP COLUMN redemption_id;
DROP TABLE public.redemptions;
DROP TYPE public.voucher_status;
DROP TABLE public.rewards;
