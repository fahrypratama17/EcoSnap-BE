package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
)

func rollbackLater(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}

// IssueQR revokes any live token and stores a new one (D-06). The deposit row
// lock serializes issuance against claims of the same deposit.
func (r *DepositRepository) IssueQR(ctx context.Context, uid, depositID, tokenHash string, ttl time.Duration) (service.QRRecord, error) {
	var out service.QRRecord
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, err
	}
	defer rollbackLater(ctx, tx)
	partner, err := authorizeMitraTx(ctx, tx, uid)
	if err != nil {
		return out, err
	}
	var status string
	err = tx.QueryRow(ctx, `SELECT status::text FROM public.deposits WHERE id=$1::uuid AND partner_id=$2::uuid FOR UPDATE`, depositID, partner).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, service.ErrDepositNotFound
	}
	if err != nil {
		return out, err
	}
	if status == "claimed" {
		return out, service.ErrDepositAlreadyClaimed
	}
	// clock_timestamp, not now(): the tx may have waited on the row lock.
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&out.IssuedAt); err != nil {
		return out, err
	}
	out.ExpiresAt = out.IssuedAt.Add(ttl)
	if _, err = tx.Exec(ctx, `UPDATE public.deposit_qr_tokens SET revoked_at=$2 WHERE deposit_id=$1::uuid AND revoked_at IS NULL AND used_at IS NULL`, depositID, out.IssuedAt); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.deposit_qr_tokens(id,deposit_id,token_hash,issued_by_uid,issued_at,expires_at) VALUES(gen_random_uuid(),$1::uuid,$2,$3,$4,$5)`, depositID, tokenHash, uid, out.IssuedAt, out.ExpiresAt); err != nil {
		return out, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.deposits SET status='qr_issued' WHERE id=$1::uuid`, depositID); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// authorizeClaimerTx allows only active accounts with role user (D-18).
func authorizeClaimerTx(ctx context.Context, tx pgx.Tx, uid string) error {
	var role, status string
	err := tx.QueryRow(ctx, `SELECT role::text,status::text FROM public.accounts WHERE firebase_uid=$1 FOR SHARE`, uid).Scan(&role, &status)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return service.ErrAccountNotFound
	case err != nil:
		return err
	case status != "active":
		return service.ErrAccountForbidden
	case role != "user":
		return service.ErrClaimForbidden
	}
	return nil
}

type qrState struct {
	id, depositID, depositStatus string
	expiresAt, now               time.Time
	revoked, used                bool
}

// lockQR finds the token, then locks its deposit row and rereads the token so
// a concurrent reissue or claim is observed before deciding.
func lockQR(ctx context.Context, tx pgx.Tx, tokenHash string, lockDeposit bool) (qrState, error) {
	var s qrState
	err := tx.QueryRow(ctx, `SELECT deposit_id::text FROM public.deposit_qr_tokens WHERE token_hash=$1`, tokenHash).Scan(&s.depositID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, service.ErrQRInvalid
	}
	if err != nil {
		return s, err
	}
	lock := ""
	if lockDeposit {
		lock = " FOR UPDATE"
	}
	if err = tx.QueryRow(ctx, `SELECT status::text FROM public.deposits WHERE id=$1::uuid`+lock, s.depositID).Scan(&s.depositStatus); err != nil {
		return s, err
	}
	err = tx.QueryRow(ctx, `SELECT id::text,expires_at,revoked_at IS NOT NULL,used_at IS NOT NULL,clock_timestamp() FROM public.deposit_qr_tokens WHERE token_hash=$1`, tokenHash).
		Scan(&s.id, &s.expiresAt, &s.revoked, &s.used, &s.now)
	return s, err
}

// usable applies D-05: at exactly expires_at the token is already expired.
func (s qrState) usable() error {
	switch {
	case s.used || s.depositStatus == "claimed":
		return service.ErrDepositAlreadyClaimed
	case s.revoked:
		return service.ErrQRRevoked
	case !s.now.Before(s.expiresAt):
		return service.ErrQRExpired
	}
	return nil
}

func (r *DepositRepository) PreviewQR(ctx context.Context, uid, tokenHash string) (service.ClaimPreviewRecord, error) {
	var out service.ClaimPreviewRecord
	// Not ReadOnly: authorizeClaimerTx takes FOR SHARE. Nothing is written.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return out, err
	}
	defer rollbackLater(ctx, tx)
	if err = authorizeClaimerTx(ctx, tx, uid); err != nil {
		return out, err
	}
	s, err := lockQR(ctx, tx, tokenHash, false)
	if err != nil {
		return out, err
	}
	if err = s.usable(); err != nil {
		return out, err
	}
	if out.Deposit, err = loadDeposit(ctx, tx, s.depositID, nil); err != nil {
		return out, err
	}
	out.ExpiresAt, out.ServerTime = s.expiresAt, s.now
	if err = tx.QueryRow(ctx, `SELECT business_name FROM public.partners WHERE id=$1::uuid`, out.Deposit.PartnerID).Scan(&out.PartnerName); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// ClaimQR credits the first claimer: claim row, ledger entry, wallet update,
// token use, and deposit status commit together or not at all.
func (r *DepositRepository) ClaimQR(ctx context.Context, uid, tokenHash string) (service.ClaimRecord, bool, error) {
	var out service.ClaimRecord
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, false, err
	}
	defer rollbackLater(ctx, tx)
	if err = authorizeClaimerTx(ctx, tx, uid); err != nil {
		return out, false, err
	}
	s, err := lockQR(ctx, tx, tokenHash, true)
	if err != nil {
		return out, false, err
	}
	if s.depositStatus == "claimed" {
		// A retry by the original claimer (lost response) replays the result.
		var claimer string
		err = tx.QueryRow(ctx, `SELECT c.claimer_uid,c.deposit_id::text,d.partner_id::text,c.points,c.claimed_at,w.balance
 FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id JOIN public.wallets w ON w.user_uid=c.claimer_uid
 WHERE c.deposit_id=$1::uuid AND c.qr_token_id=$2::uuid`, s.depositID, s.id).Scan(&claimer, &out.DepositID, &out.PartnerID, &out.Points, &out.ClaimedAt, &out.Balance)
		if err == nil && claimer == uid {
			return out, false, tx.Commit(ctx)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, false, err
		}
		return service.ClaimRecord{}, false, service.ErrDepositAlreadyClaimed
	}
	if err = s.usable(); err != nil {
		return out, false, err
	}
	out.DepositID, out.ClaimedAt = s.depositID, s.now
	if err = tx.QueryRow(ctx, `SELECT partner_id::text,total_points FROM public.deposits WHERE id=$1::uuid`, s.depositID).Scan(&out.PartnerID, &out.Points); err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.deposit_claims(deposit_id,qr_token_id,claimer_uid,points,claimed_at) VALUES($1::uuid,$2::uuid,$3,$4,$5)`, s.depositID, s.id, uid, out.Points, s.now); err != nil {
		return out, false, err
	}
	if out.Points > 0 {
		if _, err = tx.Exec(ctx, `INSERT INTO public.point_ledger(id,user_uid,amount,source,deposit_id,idempotency_key,created_at) VALUES(gen_random_uuid(),$1,$2,'deposit',$3::uuid,'deposit:'||$3,$4)`, uid, out.Points, s.depositID, s.now); err != nil {
			return out, false, err
		}
	}
	err = tx.QueryRow(ctx, `UPDATE public.wallets SET balance=balance+$2,total_earned=total_earned+$2,updated_at=$3 WHERE user_uid=$1 RETURNING balance`, uid, out.Points, s.now).Scan(&out.Balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, false, errors.New("wallet pengguna tidak ditemukan")
	}
	if err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.deposit_qr_tokens SET used_at=$2 WHERE id=$1::uuid`, s.id, s.now); err != nil {
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.deposits SET status='claimed' WHERE id=$1::uuid`, s.depositID); err != nil {
		return out, false, err
	}
	return out, true, tx.Commit(ctx)
}
