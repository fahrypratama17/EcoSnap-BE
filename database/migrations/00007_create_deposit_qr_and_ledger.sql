-- +goose Up
-- Every issuance is kept (D-06 history). Only the SHA-256 of the bearer token
-- is stored, so a database read does not expose claimable tokens.
CREATE TABLE public.deposit_qr_tokens (
 id UUID PRIMARY KEY,
 deposit_id UUID NOT NULL REFERENCES public.deposits(id),
 token_hash CHAR(64) NOT NULL UNIQUE,
 issued_by_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 issued_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 used_at TIMESTAMPTZ,
 CHECK(expires_at > issued_at),
 CHECK(revoked_at IS NULL OR used_at IS NULL)
);
-- At most one token per deposit can still be claimed.
CREATE UNIQUE INDEX deposit_qr_tokens_live_idx ON public.deposit_qr_tokens(deposit_id) WHERE revoked_at IS NULL AND used_at IS NULL;
-- One claim per deposit (D-18): first successful claimer only.
CREATE TABLE public.deposit_claims (
 deposit_id UUID PRIMARY KEY REFERENCES public.deposits(id),
 qr_token_id UUID NOT NULL UNIQUE REFERENCES public.deposit_qr_tokens(id),
 claimer_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 points BIGINT NOT NULL CHECK(points >= 0),
 claimed_at TIMESTAMPTZ NOT NULL
);
CREATE TYPE public.ledger_source AS ENUM ('deposit');
-- Append-only. Corrections (future appeals, Q-17) add entries; they never
-- edit or delete a credit.
CREATE TABLE public.point_ledger (
 id UUID PRIMARY KEY,
 user_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 amount BIGINT NOT NULL CHECK(amount <> 0),
 source public.ledger_source NOT NULL,
 deposit_id UUID REFERENCES public.deposits(id),
 idempotency_key VARCHAR(150) NOT NULL UNIQUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(source <> 'deposit' OR deposit_id IS NOT NULL)
);
CREATE INDEX point_ledger_user_created_idx ON public.point_ledger(user_uid, created_at DESC);
ALTER TABLE public.deposit_qr_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.deposit_claims ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.point_ledger ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.deposit_qr_tokens, public.deposit_claims, public.point_ledger FROM anon, authenticated;
-- +goose Down
DROP TABLE public.point_ledger;
DROP TYPE public.ledger_source;
DROP TABLE public.deposit_claims;
DROP TABLE public.deposit_qr_tokens;
