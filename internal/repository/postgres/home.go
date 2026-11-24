package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
)

const homeAccountSQL = `SELECT status::text,role::text,full_name FROM public.accounts WHERE firebase_uid=$1 FOR SHARE`

// homeWalletSQL args: uid=$1, month start=$2, month end=$3.
const homeWalletSQL = `SELECT w.balance,w.total_earned,w.total_spent,w.updated_at,
 (SELECT count(*) FROM public.deposit_claims c WHERE c.claimer_uid=$1),
 (SELECT count(*) FROM public.deposit_claims c WHERE c.claimer_uid=$1 AND c.claimed_at>=$2 AND c.claimed_at<$3),
 (SELECT COALESCE(sum(d.total_weight_grams),0) FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id WHERE c.claimer_uid=$1),
 (SELECT count(*) FROM public.redemptions r WHERE r.user_uid=$1 AND r.status='issued' AND r.expires_at>$3)
 FROM public.wallets w WHERE w.user_uid=$1`

// UserHome reads every home card in one repeatable-read snapshot on one
// connection, so the numbers, periods and cards agree with each other. It
// uses two pipelined batches: the account lock and clock sample first (the
// clock must be read after the lock), then every card query.
func (r *WalletRepository) UserHome(ctx context.Context, uid string) (entity.UserHome, error) {
	var h entity.UserHome
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return h, err
	}
	defer rollbackLater(ctx, tx)
	role, err := homeAccount(ctx, tx, uid, &h)
	if err != nil {
		return h, err
	}
	// The month is the WIB calendar month so far, like the activity report.
	nowWIB := h.ServerTime.In(service.WIB)
	h.Month = entity.HomeMonth{
		From:     time.Date(nowWIB.Year(), nowWIB.Month(), 1, 0, 0, 0, 0, service.WIB),
		To:       h.ServerTime,
		Timezone: "Asia/Jakarta",
	}
	if err = homeCards(ctx, tx, uid, role == "user", &h); err != nil {
		return h, err
	}
	return h, tx.Commit(ctx)
}

// homeAccount runs phase one: lock and check the account, then sample the
// clock in the same batch so it is ordered after the lock.
func homeAccount(ctx context.Context, tx pgx.Tx, uid string, h *entity.UserHome) (string, error) {
	b := pgx.Batch{QueuedQueries: make([]*pgx.QueuedQuery, 0, 2)}
	b.Queue(homeAccountSQL, uid)
	b.Queue(`SELECT clock_timestamp()`)
	br := tx.SendBatch(ctx, &b)
	var status, role string
	err := br.QueryRow().Scan(&status, &role, &h.UserName)
	if err == nil {
		err = br.QueryRow().Scan(&h.ServerTime)
	}
	if cerr := br.Close(); err == nil {
		err = cerr
	}
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", service.ErrAccountNotFound
	case err != nil:
		return "", err
	case status != "active":
		return "", service.ErrAccountForbidden
	}
	return role, nil
}

// homeCards runs phase two: every card query in one pipelined batch.
// Mitra keep access but use the partner home; no mission card for them.
func homeCards(ctx context.Context, tx pgx.Tx, uid string, withMissions bool, h *entity.UserHome) (err error) {
	b := pgx.Batch{QueuedQueries: make([]*pgx.QueuedQuery, 0, 6)}
	b.Queue(homeWalletSQL, uid, h.Month.From, h.Month.To)
	hsql, hargs := pointHistoryQuery(uid, service.HistoryFilter{}, service.HomeRecent, nil)
	b.Queue(hsql, hargs...)
	var snap service.MissionSnapshot
	if withMissions {
		snap = missionSnapshotAt(h.ServerTime)
		b.Queue(missionSnapshotSQL, uid, snap.PeriodStart, snap.PeriodEnd, snap.Now)
	}
	psql, pargs := partnersQuery(service.PartnerFilter{Limit: 1})
	b.Queue(psql, pargs...)
	asql, aargs := articlesQuery(service.ArticleFilter{}, 1, nil)
	b.Queue(asql, aargs...)
	b.Queue(unreadCountSQL, uid)

	br := tx.SendBatch(ctx, &b)
	defer func() {
		if cerr := br.Close(); err == nil {
			err = cerr
		}
	}()
	var grams int64
	if err = br.QueryRow().Scan(&h.Wallet.Balance, &h.Wallet.TotalEarned, &h.Wallet.TotalSpent, &h.Wallet.UpdatedAt, &h.DepositsClaimed, &h.MonthlyDeposits, &grams, &h.ActiveVouchers); err != nil {
		return err
	}
	h.TotalWeightKG = service.FormatWeightKG(grams)
	rows, err := br.Query()
	if err != nil {
		return err
	}
	if h.RecentActivity, err = scanPointHistory(rows); err != nil {
		return err
	}
	if withMissions {
		if err = scanMissionSnapshot(br.QueryRow(), &snap); err != nil {
			return err
		}
		missions := service.MissionListFromSnapshot(snap)
		h.Missions = &missions
	}
	if rows, err = br.Query(); err != nil {
		return err
	}
	partners, err := scanPartners(rows)
	if err != nil {
		return err
	}
	if len(partners) > 0 {
		h.Partner = &partners[0]
	}
	if rows, err = br.Query(); err != nil {
		return err
	}
	articles, err := scanArticles(rows)
	if err != nil {
		return err
	}
	if len(articles) > 0 {
		h.Article = &articles[0]
	}
	return br.QueryRow().Scan(&h.UnreadCount)
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
