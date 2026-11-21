-- +goose Up
-- Weekly mission XP claims (D24/D25). One immutable row per user, mission and
-- week; the primary key makes a double claim impossible. XP is separate from
-- the EcoPoints wallet and ledger. Badges are derived from deposit_claims.
-- period_start/period_end are Asia/Jakarta Monday 00:00 boundaries (end excl.).
CREATE TABLE public.mission_claims (
 user_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 mission_id VARCHAR(32) NOT NULL CHECK(mission_id IN ('first_deposit','three_kilograms')),
 period_start TIMESTAMPTZ NOT NULL,
 period_end TIMESTAMPTZ NOT NULL,
 progress BIGINT NOT NULL CHECK(progress >= 0),
 target BIGINT NOT NULL CHECK(target > 0),
 xp INTEGER NOT NULL CHECK(xp > 0),
 claimed_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(user_uid, mission_id, period_start),
 CHECK(period_end > period_start),
 CHECK(progress >= target),
 CHECK(claimed_at >= period_start AND claimed_at < period_end)
);
CREATE INDEX mission_claims_period_idx ON public.mission_claims(period_start, user_uid) INCLUDE (xp);
CREATE INDEX deposit_claims_claimer_idx ON public.deposit_claims(claimer_uid, claimed_at);
-- Claims are history that badges and rankings derive from: never edited.
-- +goose StatementBegin
CREATE FUNCTION public.reject_claim_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION '% is immutable', TG_TABLE_NAME USING ERRCODE = 'restrict_violation';
END $$;
-- +goose StatementEnd
CREATE TRIGGER mission_claims_immutable BEFORE UPDATE OR DELETE ON public.mission_claims
 FOR EACH ROW EXECUTE FUNCTION public.reject_claim_mutation();
CREATE TRIGGER deposit_claims_immutable BEFORE UPDATE OR DELETE ON public.deposit_claims
 FOR EACH ROW EXECUTE FUNCTION public.reject_claim_mutation();
ALTER TABLE public.mission_claims ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.mission_claims FROM anon, authenticated;
-- +goose Down
DROP TRIGGER deposit_claims_immutable ON public.deposit_claims;
DROP INDEX public.deposit_claims_claimer_idx;
DROP TABLE public.mission_claims;
DROP FUNCTION public.reject_claim_mutation();
