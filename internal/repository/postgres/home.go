package postgres

import (
	"context"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
)

// UserHome reads every home card in one repeatable-read snapshot on one
// connection, so the numbers, periods and cards agree with each other.
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
	var role string
	if err = tx.QueryRow(ctx, `SELECT role::text,full_name FROM public.accounts WHERE firebase_uid=$1`, uid).Scan(&role, &h.UserName); err != nil {
		return h, err
	}
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&h.ServerTime); err != nil {
		return h, err
	}
	// The month is the WIB calendar month so far, like the activity report.
	nowWIB := h.ServerTime.In(service.WIB)
	h.Month = entity.HomeMonth{
		From:     time.Date(nowWIB.Year(), nowWIB.Month(), 1, 0, 0, 0, 0, service.WIB),
		To:       h.ServerTime,
		Timezone: "Asia/Jakarta",
	}
	var grams int64
	err = tx.QueryRow(ctx, `SELECT w.balance,w.total_earned,w.total_spent,w.updated_at,
 (SELECT count(*) FROM public.deposit_claims c WHERE c.claimer_uid=$1),
 (SELECT count(*) FROM public.deposit_claims c WHERE c.claimer_uid=$1 AND c.claimed_at>=$2 AND c.claimed_at<$3),
 (SELECT COALESCE(sum(d.total_weight_grams),0) FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id WHERE c.claimer_uid=$1),
 (SELECT count(*) FROM public.redemptions r WHERE r.user_uid=$1 AND r.status='issued' AND r.expires_at>$3)
 FROM public.wallets w WHERE w.user_uid=$1`, uid, h.Month.From, h.Month.To).
		Scan(&h.Wallet.Balance, &h.Wallet.TotalEarned, &h.Wallet.TotalSpent, &h.Wallet.UpdatedAt, &h.DepositsClaimed, &h.MonthlyDeposits, &grams, &h.ActiveVouchers)
	if err != nil {
		return h, err
	}
	h.TotalWeightKG = service.FormatWeightKG(grams)
	if h.RecentActivity, err = pointHistoryTx(ctx, tx, uid, service.HistoryFilter{}, service.HomeRecent, nil); err != nil {
		return h, err
	}
	// Mitra keep access but use the partner home; no mission card for them.
	if role == "user" {
		snap, err := missionSnapshotTx(ctx, tx, uid, h.ServerTime)
		if err != nil {
			return h, err
		}
		missions := service.MissionListFromSnapshot(snap)
		h.Missions = &missions
	}
	partners, err := partnersTx(ctx, tx, service.PartnerFilter{Limit: 1})
	if err != nil {
		return h, err
	}
	if len(partners) > 0 {
		h.Partner = &partners[0]
	}
	articles, err := articlesTx(ctx, tx, service.ArticleFilter{}, 1, nil)
	if err != nil {
		return h, err
	}
	if len(articles) > 0 {
		h.Article = &articles[0]
	}
	if h.UnreadCount, err = unreadCountTx(ctx, tx, uid); err != nil {
		return h, err
	}
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
