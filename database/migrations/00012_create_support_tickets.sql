-- +goose Up
-- Support tickets (problem reports and feedback). There is no admin role:
-- the repo owner reads and answers tickets with cmd/support-tickets using the
-- owner DATABASE_URL. The original text is immutable; only status, the
-- single staff response and its timestamps ever change.
CREATE TYPE public.support_ticket_kind AS ENUM ('problem','feedback');
CREATE TYPE public.support_ticket_status AS ENUM ('open','in_progress','resolved');
CREATE TABLE public.support_tickets (
 id UUID PRIMARY KEY,
 user_uid VARCHAR(128) NOT NULL REFERENCES public.accounts(firebase_uid),
 idempotency_key VARCHAR(128) NOT NULL CHECK(idempotency_key ~ '^[A-Za-z0-9_-]{8,128}$'),
 request_hash CHAR(64) NOT NULL CHECK(request_hash ~ '^[0-9a-f]{64}$'),
 kind public.support_ticket_kind NOT NULL,
 subject VARCHAR(150) NOT NULL CHECK(length(btrim(subject)) > 0),
 message TEXT NOT NULL CHECK(length(btrim(message)) > 0 AND char_length(message) <= 4000),
 status public.support_ticket_status NOT NULL DEFAULT 'open',
 staff_response TEXT CHECK(staff_response IS NULL OR (length(btrim(staff_response)) > 0 AND char_length(staff_response) <= 4000)),
 responded_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL,
 UNIQUE(user_uid, idempotency_key),
 CHECK((staff_response IS NULL) = (responded_at IS NULL)),
 CHECK(updated_at >= created_at)
);
CREATE INDEX support_tickets_user_created_idx ON public.support_tickets(user_uid, created_at DESC, id DESC);
CREATE INDEX support_tickets_open_idx ON public.support_tickets(user_uid) WHERE status <> 'resolved';
CREATE INDEX support_tickets_status_created_idx ON public.support_tickets(status, created_at, id);
-- Tickets are never deleted and the user's original words never change.
-- +goose StatementBegin
CREATE FUNCTION public.guard_support_ticket() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP = 'DELETE' THEN
  RAISE EXCEPTION 'support_tickets rows cannot be deleted' USING ERRCODE = 'restrict_violation';
 END IF;
 IF NEW.id IS DISTINCT FROM OLD.id OR NEW.user_uid IS DISTINCT FROM OLD.user_uid
  OR NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key OR NEW.request_hash IS DISTINCT FROM OLD.request_hash
  OR NEW.kind IS DISTINCT FROM OLD.kind OR NEW.subject IS DISTINCT FROM OLD.subject
  OR NEW.message IS DISTINCT FROM OLD.message OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
  RAISE EXCEPTION 'support ticket original data is immutable' USING ERRCODE = 'restrict_violation';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER support_tickets_guard BEFORE UPDATE OR DELETE ON public.support_tickets
 FOR EACH ROW EXECUTE FUNCTION public.guard_support_ticket();
ALTER TABLE public.support_tickets ENABLE ROW LEVEL SECURITY;
REVOKE ALL PRIVILEGES ON TABLE public.support_tickets FROM anon, authenticated;
-- +goose Down
DROP TRIGGER support_tickets_guard ON public.support_tickets;
DROP TABLE public.support_tickets;
DROP FUNCTION public.guard_support_ticket();
DROP TYPE public.support_ticket_status;
DROP TYPE public.support_ticket_kind;
