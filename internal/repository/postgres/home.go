package postgres

import (
	"context"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
)

// UserHome reads wallet and totals in one snapshot so the numbers agree.
func (r *WalletRepository) UserHome(ctx context.Context, uid string) (entity.UserHome, error) {
	var h entity.UserHome
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return h, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return h, err
	}
	var grams int64
	err = tx.QueryRow(ctx, `SELECT w.balance,w.total_earned,w.total_spent,w.updated_at,
 (SELECT count(*) FROM public.deposit_claims c WHERE c.claimer_uid=$1),
 (SELECT COALESCE(sum(d.total_weight_grams),0) FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id WHERE c.claimer_uid=$1),
 (SELECT count(*) FROM public.redemptions r WHERE r.user_uid=$1 AND r.status='issued' AND r.expires_at>clock_timestamp())
 FROM public.wallets w WHERE w.user_uid=$1`, uid).
		Scan(&h.Wallet.Balance, &h.Wallet.TotalEarned, &h.Wallet.TotalSpent, &h.Wallet.UpdatedAt, &h.DepositsClaimed, &grams, &h.ActiveVouchers)
	if err != nil {
		return h, err
	}
	h.TotalWeightKG = service.FormatWeightKG(grams)
	return h, tx.Commit(ctx)
}

func (r *WalletRepository) PartnerHome(ctx context.Context, uid string) (entity.PartnerHome, error) {
	var h entity.PartnerHome
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return h, err
	}
	defer rollbackLater(ctx, tx)
	if h.PartnerID, err = partnerReaderTx(ctx, tx, uid); err != nil {
		return h, err
	}
	var grams int64
	err = tx.QueryRow(ctx, `SELECT p.business_name,p.status::text,
 count(d.id) FILTER (WHERE d.status='created'),
 count(d.id) FILTER (WHERE d.status='qr_issued'),
 count(d.id) FILTER (WHERE d.status='claimed'),
 COALESCE(sum(d.total_weight_grams) FILTER (WHERE d.status='claimed'),0),
 COALESCE(sum(d.total_points) FILTER (WHERE d.status='claimed'),0)
 FROM public.partners p LEFT JOIN public.deposits d ON d.partner_id=p.id
 WHERE p.id=$1::uuid GROUP BY p.id`, h.PartnerID).
		Scan(&h.BusinessName, &h.PartnerStatus, &h.Deposits.Created, &h.Deposits.QRIssued, &h.Deposits.Claimed, &grams, &h.ClaimedPoints)
	if err != nil {
		return h, err
	}
	h.ClaimedWeightKG = service.FormatWeightKG(grams)
	return h, tx.Commit(ctx)
}
