package postgres

import (
	"context"
	"errors"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SupportRepository struct{ pool *pgxpool.Pool }

func NewSupportRepository(pool *pgxpool.Pool) *SupportRepository {
	return &SupportRepository{pool: pool}
}

const supportColumns = `t.id::text,t.kind::text,t.subject,t.message,t.status::text,t.staff_response,t.responded_at,t.created_at,t.updated_at`

func scanSupportTicket(row pgx.Row) (entity.SupportTicket, error) {
	var t entity.SupportTicket
	err := row.Scan(&t.ID, &t.Kind, &t.Subject, &t.Message, &t.Status, &t.StaffResponse, &t.RespondedAt, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

func scanSupportOwnerView(row pgx.Row) (entity.SupportTicketOwnerView, error) {
	var t entity.SupportTicketOwnerView
	err := row.Scan(&t.ID, &t.Kind, &t.Subject, &t.Message, &t.Status, &t.StaffResponse, &t.RespondedAt, &t.CreatedAt, &t.UpdatedAt, &t.UserUID)
	return t, err
}

// ActiveAccount allows any active account, user or mitra, without requiring a
// verified partner location.
func (r *SupportRepository) ActiveAccount(ctx context.Context, uid string) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// CreateTicket serialises per account with an advisory lock held until commit,
// so the idempotency lookup, the unresolved-ticket cap and the insert cannot
// race. The replay lookup runs before the cap so a retry still succeeds at the
// cap.
func (r *SupportRepository) CreateTicket(ctx context.Context, in service.NewSupportTicket) (entity.SupportTicket, bool, error) {
	var out entity.SupportTicket
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, false, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, in.UID); err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('support:'||$1,0))`, in.UID); err != nil {
		return out, false, err
	}
	var hash string
	err = tx.QueryRow(ctx, `SELECT request_hash FROM public.support_tickets WHERE user_uid=$1 AND idempotency_key=$2`, in.UID, in.IdempotencyKey).Scan(&hash)
	if err == nil {
		if hash != in.RequestHash {
			return out, false, service.ErrSupportTicketConflict
		}
		out, err = scanSupportTicket(tx.QueryRow(ctx, `SELECT `+supportColumns+` FROM public.support_tickets t WHERE t.user_uid=$1 AND t.idempotency_key=$2`, in.UID, in.IdempotencyKey))
		if err != nil {
			return out, false, err
		}
		return out, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, false, err
	}
	var unresolved int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.support_tickets WHERE user_uid=$1 AND status<>'resolved'`, in.UID).Scan(&unresolved); err != nil {
		return out, false, err
	}
	if unresolved >= service.MaxUnresolvedSupportTickets {
		return out, false, service.ErrSupportTicketLimit
	}
	out, err = scanSupportTicket(tx.QueryRow(ctx, `WITH clk AS (SELECT clock_timestamp() AS t)
 INSERT INTO public.support_tickets AS t(id,user_uid,idempotency_key,request_hash,kind,subject,message,created_at,updated_at)
 SELECT gen_random_uuid(),$1,$2,$3,$4::public.support_ticket_kind,$5,$6,t,t FROM clk RETURNING `+supportColumns,
		in.UID, in.IdempotencyKey, in.RequestHash, in.Kind, in.Subject, in.Message))
	if err != nil {
		return out, false, err
	}
	return out, true, tx.Commit(ctx)
}

func (r *SupportRepository) Tickets(ctx context.Context, uid string, status *string, limit int, after *service.Cursor) ([]entity.SupportTicket, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return nil, err
	}
	at, id := cursorArgs(after)
	rows, err := tx.Query(ctx, `SELECT `+supportColumns+` FROM public.support_tickets t
 WHERE t.user_uid=$1 AND ($2::text IS NULL OR t.status::text=$2)
 AND ($3::timestamptz IS NULL OR (t.created_at,t.id)<($3,$4::uuid))
 ORDER BY t.created_at DESC,t.id DESC LIMIT $5`, uid, status, at, id, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entity.SupportTicket{}
	for rows.Next() {
		t, err := scanSupportTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Ticket returns only the caller's ticket; others get not-found.
func (r *SupportRepository) Ticket(ctx context.Context, uid, id string) (entity.SupportTicket, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return entity.SupportTicket{}, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return entity.SupportTicket{}, err
	}
	t, err := scanSupportTicket(tx.QueryRow(ctx, `SELECT `+supportColumns+` FROM public.support_tickets t WHERE t.id=$1::uuid AND t.user_uid=$2`, id, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return t, service.ErrSupportTicketNotFound
	}
	if err != nil {
		return t, err
	}
	return t, tx.Commit(ctx)
}

func (r *SupportRepository) OwnerTickets(ctx context.Context, id, status *string, limit int) ([]entity.SupportTicketOwnerView, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+supportColumns+`,t.user_uid FROM public.support_tickets t
 WHERE ($1::uuid IS NULL OR t.id=$1::uuid) AND ($2::text IS NULL OR t.status::text=$2)
 ORDER BY t.created_at,t.id LIMIT $3`, id, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []entity.SupportTicketOwnerView{}
	for rows.Next() {
		t, err := scanSupportOwnerView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// OwnerUpdate locks the ticket, validates the status transition and the
// no-silent-overwrite rule against the locked row, and applies status and
// response together. A response is never replaced unless ReplaceResponse.
func (r *SupportRepository) OwnerUpdate(ctx context.Context, in service.SupportOwnerUpdate) (entity.SupportTicketOwnerView, error) {
	var out entity.SupportTicketOwnerView
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, err
	}
	defer rollbackLater(ctx, tx)
	var status string
	var hasResponse bool
	err = tx.QueryRow(ctx, `SELECT status::text,staff_response IS NOT NULL FROM public.support_tickets WHERE id=$1::uuid FOR UPDATE`, in.ID).Scan(&status, &hasResponse)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, service.ErrSupportTicketNotFound
	}
	if err != nil {
		return out, err
	}
	if in.Status != nil {
		if err = service.CheckSupportTransition(status, *in.Status); err != nil {
			return out, err
		}
	}
	if in.Response != nil && hasResponse && !in.ReplaceResponse {
		return out, service.ErrSupportReplyExists
	}
	out, err = scanSupportOwnerView(tx.QueryRow(ctx, `UPDATE public.support_tickets t SET
 status=COALESCE($2::public.support_ticket_status,t.status),
 staff_response=COALESCE($3,t.staff_response),
 responded_at=CASE WHEN $3::text IS NULL THEN t.responded_at ELSE clock_timestamp() END,
 updated_at=clock_timestamp()
 WHERE t.id=$1::uuid RETURNING `+supportColumns+`,t.user_uid`, in.ID, in.Status, in.Response))
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
