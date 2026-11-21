-- +goose Up
-- In-app notification inbox (no push/email). Rows are created only by AFTER
-- INSERT triggers on the event tables, inside the same transaction as the event:
-- a rolled-back claim, redemption or mission claim leaves no notification.
-- Existing rows are not back-filled and there is no expiry job.
-- source_id is the event row UUID as text; for missions it is the canonical
-- '<mission_id>:<YYYY-MM-DD>' (period start, Asia/Jakarta). UNIQUE(recipient,
-- event_type, source_id) makes a replayed event unable to notify twice.
-- seen_at is set once, by the owner, and never moves afterwards.
CREATE TABLE public.notifications (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 recipient_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 event_type VARCHAR(32) NOT NULL CHECK(event_type IN ('deposit_credited','deposit_claimed','voucher_issued','mission_xp_claimed')),
 source_id VARCHAR(80) NOT NULL CHECK(length(source_id) > 0),
 title VARCHAR(120) NOT NULL,
 body VARCHAR(300) NOT NULL,
 target_type VARCHAR(16) NOT NULL CHECK(target_type IN ('deposit','voucher','mission')),
 target_id VARCHAR(64) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 seen_at TIMESTAMPTZ,
 UNIQUE(recipient_uid, event_type, source_id),
 CHECK(seen_at IS NULL OR seen_at >= created_at)
);
CREATE INDEX notifications_recipient_created_idx ON public.notifications(recipient_uid, created_at DESC, id DESC);
CREATE INDEX notifications_recipient_unread_idx ON public.notifications(recipient_uid, created_at DESC, id DESC) WHERE seen_at IS NULL;

-- Preference check: a plain (unlocked) read of user_settings.notification_enabled
-- sees whatever is committed when the trigger statement starts. It takes no row
-- lock, so it cannot add to the wallet/account lock order of the claim and
-- redemption transactions. A toggle committing concurrently with an event
-- applies to that event only if it was visible at that point.
-- A missing settings row fails closed: no notification is created.
-- +goose StatementBegin
CREATE FUNCTION public.notify_recipient(p_uid TEXT, p_type TEXT, p_source TEXT, p_title TEXT, p_body TEXT, p_target_type TEXT, p_target_id TEXT) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
 IF NOT COALESCE((SELECT notification_enabled FROM public.user_settings WHERE firebase_uid = p_uid), false) THEN
  RETURN;
 END IF;
 INSERT INTO public.notifications(recipient_uid,event_type,source_id,title,body,target_type,target_id)
 VALUES(p_uid,p_type,p_source,p_title,p_body,p_target_type,p_target_id)
 ON CONFLICT (recipient_uid,event_type,source_id) DO NOTHING;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION public.notify_deposit_claim() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
 v_owner TEXT;
 v_grams BIGINT;
BEGIN
 SELECT p.mitra_uid, d.total_weight_grams INTO v_owner, v_grams
 FROM public.deposits d JOIN public.partners p ON p.id = d.partner_id WHERE d.id = NEW.deposit_id;
 PERFORM public.notify_recipient(NEW.claimer_uid, 'deposit_credited', NEW.deposit_id::text,
  'Poin setoran diterima', 'Anda menerima ' || NEW.points || ' poin dari setoran sampah.',
  'deposit', NEW.deposit_id::text);
 IF v_owner IS NOT NULL THEN
  PERFORM public.notify_recipient(v_owner, 'deposit_claimed', NEW.deposit_id::text,
   'Setoran telah diklaim', 'Setoran seberat ' || to_char(v_grams / 1000.0, 'FM999990.000') || ' kg telah diklaim pengguna.',
   'deposit', NEW.deposit_id::text);
 END IF;
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER deposit_claims_notify AFTER INSERT ON public.deposit_claims
 FOR EACH ROW EXECUTE FUNCTION public.notify_deposit_claim();

-- +goose StatementBegin
CREATE FUNCTION public.notify_redemption() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM public.notify_recipient(NEW.user_uid, 'voucher_issued', NEW.id::text,
  'Voucher berhasil ditukar', 'Voucher ' || NEW.reward_name || ' sebesar ' || NEW.cost_points || ' poin telah terbit.',
  'voucher', NEW.id::text);
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER redemptions_notify AFTER INSERT ON public.redemptions
 FOR EACH ROW EXECUTE FUNCTION public.notify_redemption();

-- +goose StatementBegin
CREATE FUNCTION public.notify_mission_claim() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM public.notify_recipient(NEW.user_uid, 'mission_xp_claimed',
  NEW.mission_id || ':' || to_char((NEW.period_start AT TIME ZONE 'UTC') + interval '7 hours', 'YYYY-MM-DD'),
  'Misi selesai', 'Anda mendapat ' || NEW.xp || ' XP dari misi mingguan.',
  'mission', NEW.mission_id);
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER mission_claims_notify AFTER INSERT ON public.mission_claims
 FOR EACH ROW EXECUTE FUNCTION public.notify_mission_claim();

ALTER TABLE public.notifications ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.notifications FROM anon, authenticated;
REVOKE ALL PRIVILEGES ON FUNCTION public.notify_recipient(TEXT,TEXT,TEXT,TEXT,TEXT,TEXT,TEXT), public.notify_deposit_claim(), public.notify_redemption(), public.notify_mission_claim() FROM PUBLIC, anon, authenticated;
-- +goose Down
DROP TRIGGER mission_claims_notify ON public.mission_claims;
DROP TRIGGER redemptions_notify ON public.redemptions;
DROP TRIGGER deposit_claims_notify ON public.deposit_claims;
DROP FUNCTION public.notify_mission_claim();
DROP FUNCTION public.notify_redemption();
DROP FUNCTION public.notify_deposit_claim();
DROP FUNCTION public.notify_recipient(TEXT,TEXT,TEXT,TEXT,TEXT,TEXT,TEXT);
DROP TABLE public.notifications;
