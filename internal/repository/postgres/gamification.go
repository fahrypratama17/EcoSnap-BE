package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GamificationRepository struct{ pool *pgxpool.Pool }

func NewGamificationRepository(pool *pgxpool.Pool) *GamificationRepository {
	return &GamificationRepository{pool: pool}
}

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// readTx is one repeatable-read snapshot and may take the account share lock.
func (r *GamificationRepository) readTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
}

// checkUserTx requires an active account with role user and holds a share lock
// so suspension/deletion cannot race this operation.
func checkUserTx(ctx context.Context, q rowQuerier, uid string) error {
	var role, status string
	err := q.QueryRow(ctx, `SELECT role::text,status::text FROM public.accounts WHERE firebase_uid=$1 FOR SHARE`, uid).Scan(&role, &status)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return service.ErrAccountNotFound
	case err != nil:
		return err
	case status != "active":
		return service.ErrAccountForbidden
	case role != "user":
		return service.ErrGamificationForbidden
	}
	return nil
}

// missionSnapshotSQL reads the whole mission snapshot in one statement.
// Args: uid=$1, period start=$2, period end=$3, now=$4. The claimed map
// intentionally does not filter claimed_at<=now; XP totals do.
const missionSnapshotSQL = `SELECT dep.deposits,dep.weight_grams,xp.weekly_xp,xp.total_xp,cl.claimed
FROM (SELECT count(*)::bigint AS deposits,COALESCE(sum(d.total_weight_grams),0)::bigint AS weight_grams
 FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id
 WHERE c.claimer_uid=$1 AND c.claimed_at>=$2::timestamptz AND c.claimed_at<$3::timestamptz AND c.claimed_at<=$4::timestamptz) dep
CROSS JOIN (SELECT COALESCE(jsonb_object_agg(mission_id,claimed_at),'{}'::jsonb) AS claimed
 FROM public.mission_claims WHERE user_uid=$1 AND period_start=$2::timestamptz) cl
CROSS JOIN (SELECT COALESCE(sum(xp) FILTER (WHERE period_start=$2::timestamptz AND claimed_at<=$4::timestamptz),0)::bigint AS weekly_xp,
 COALESCE(sum(xp) FILTER (WHERE claimed_at<=$4::timestamptz),0)::bigint AS total_xp
 FROM public.mission_claims WHERE user_uid=$1) xp`

// missionSnapshotAt returns a snapshot initialized with the period for now.
func missionSnapshotAt(now time.Time) service.MissionSnapshot {
	start, end := service.WeekPeriod(now)
	return service.MissionSnapshot{PeriodStart: start, PeriodEnd: end, Now: now}
}

// scanMissionSnapshot decodes a missionSnapshotSQL row into snapshot.
// Claimed is always non-nil on success.
func scanMissionSnapshot(row pgx.Row, snapshot *service.MissionSnapshot) error {
	return row.Scan(&snapshot.Deposits, &snapshot.WeightGrams, &snapshot.WeeklyXP, &snapshot.TotalXP, &snapshot.Claimed)
}

// missionSnapshotTx reads the current week as of now (claims after now are ignored).
func missionSnapshotTx(ctx context.Context, q rowQuerier, uid string, now time.Time) (service.MissionSnapshot, error) {
	s := missionSnapshotAt(now)
	err := scanMissionSnapshot(q.QueryRow(ctx, missionSnapshotSQL, uid, s.PeriodStart, s.PeriodEnd, now), &s)
	return s, err
}

func (r *GamificationRepository) Missions(ctx context.Context, uid string) (service.MissionSnapshot, error) {
	var out service.MissionSnapshot
	tx, err := r.readTx(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackLater(ctx, tx)
	if err = checkUserTx(ctx, tx, uid); err != nil {
		return out, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return out, err
	}
	if out, err = missionSnapshotTx(ctx, tx, uid, now); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// ClaimMission serializes per user (account share lock blocks suspension, an
// advisory lock blocks concurrent claims), samples the clock after the locks,
// rejects a stale period token, and inserts the claim once per mission and week.
func (r *GamificationRepository) ClaimMission(ctx context.Context, uid string, def service.MissionDefinition, periodStart time.Time) (service.MissionSnapshot, bool, error) {
	var out service.MissionSnapshot
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return out, false, err
	}
	defer rollbackLater(ctx, tx)
	if err = authorizeClaimerTx(ctx, tx, uid); err != nil {
		if errors.Is(err, service.ErrClaimForbidden) {
			err = service.ErrGamificationForbidden
		}
		return out, false, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('mission-claim:'||$1,0))`, uid); err != nil {
		return out, false, err
	}
	var now time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return out, false, err
	}
	if out, err = missionSnapshotTx(ctx, tx, uid, now); err != nil {
		return out, false, err
	}
	if !out.PeriodStart.Equal(periodStart) {
		return out, false, service.ErrMissionExpired
	}
	if _, claimed := out.Claimed[def.ID]; claimed {
		return out, false, tx.Commit(ctx)
	}
	progress := out.Deposits
	if def.Metric == service.MetricClaimedWeightGram {
		progress = out.WeightGrams
	}
	if progress < def.Target {
		return out, false, service.ErrMissionIncomplete
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.mission_claims(user_uid,mission_id,period_start,period_end,progress,target,xp,claimed_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		uid, def.ID, out.PeriodStart, out.PeriodEnd, progress, def.Target, def.RewardXP, now); err != nil {
		return out, false, err
	}
	if out, err = missionSnapshotTx(ctx, tx, uid, now); err != nil {
		return out, false, err
	}
	return out, true, tx.Commit(ctx)
}

func (r *GamificationRepository) Badges(ctx context.Context, uid string) (service.BadgeSnapshot, error) {
	var out service.BadgeSnapshot
	tx, err := r.readTx(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackLater(ctx, tx)
	if err = checkUserTx(ctx, tx, uid); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&out.Now); err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `WITH r AS (
 SELECT c.claimed_at, row_number() OVER w AS n, sum(d.total_weight_grams) OVER w AS s
 FROM public.deposit_claims c JOIN public.deposits d ON d.id=c.deposit_id
 WHERE c.claimer_uid=$1 AND c.claimed_at<=$2
 WINDOW w AS (ORDER BY c.claimed_at, c.deposit_id))
 SELECT COALESCE(max(n),0)::bigint, COALESCE(max(s),0)::bigint,
  min(claimed_at) FILTER (WHERE n>=$3), min(claimed_at) FILTER (WHERE n>=$4), min(claimed_at) FILTER (WHERE s>=$5)
 FROM r`, uid, out.Now, service.BadgeDepositsSmall, service.BadgeDepositsLarge, service.BadgeWeightGrams).
		Scan(&out.Deposits, &out.WeightGrams, &out.SmallAt, &out.LargeAt, &out.WeightAt)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

// weeklyRanking: eligible = active role user who opted in; positive XP only;
// RANK() gives ties the same rank (1,1,3).
const weeklyRanking = `WITH weekly AS (
 SELECT m.user_uid, sum(m.xp)::bigint AS total
 FROM public.mission_claims m
 JOIN public.accounts a ON a.firebase_uid=m.user_uid AND a.status='active' AND a.role='user'
 JOIN public.user_settings s ON s.firebase_uid=a.firebase_uid AND s.show_on_leaderboard
 WHERE m.period_start=$1 GROUP BY m.user_uid HAVING sum(m.xp)>0),
 ranked AS (
 SELECT w.user_uid, a.display_name, a.photo_path, w.total, rank() OVER (ORDER BY w.total DESC)::bigint AS rnk
 FROM weekly w JOIN public.accounts a ON a.firebase_uid=w.user_uid)
`

func (r *GamificationRepository) Leaderboard(ctx context.Context, uid string, limit int) (service.LeaderboardSnapshot, error) {
	var out service.LeaderboardSnapshot
	tx, err := r.readTx(ctx)
	if err != nil {
		return out, err
	}
	defer rollbackLater(ctx, tx)
	if err = checkUserTx(ctx, tx, uid); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&out.Now); err != nil {
		return out, err
	}
	out.PeriodStart, out.PeriodEnd = service.WeekPeriod(out.Now)
	rows, err := tx.Query(ctx, weeklyRanking+`SELECT rnk,display_name,photo_path,total FROM ranked ORDER BY rnk,display_name,user_uid LIMIT $2`, out.PeriodStart, limit)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	out.Entries = []entity.LeaderboardEntry{}
	for rows.Next() {
		var e entity.LeaderboardEntry
		if err = rows.Scan(&e.Rank, &e.DisplayName, &e.PhotoPath, &e.XP); err != nil {
			return out, err
		}
		out.Entries = append(out.Entries, e)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	var me entity.LeaderboardMe
	err = tx.QueryRow(ctx, weeklyRanking+`SELECT rnk,total FROM ranked WHERE user_uid=$2`, out.PeriodStart, uid).Scan(&me.Rank, &me.XP)
	switch {
	case err == nil:
		out.Me = &me
	case !errors.Is(err, pgx.ErrNoRows):
		return out, err
	}
	return out, tx.Commit(ctx)
}

// SetLeaderboardVisibility updates only show_on_leaderboard, holding the
// account share lock so a concurrent suspension cannot interleave.
func (r *GamificationRepository) SetLeaderboardVisibility(ctx context.Context, uid string, visible bool) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer rollbackLater(ctx, tx)
	if err = authorizeClaimerTx(ctx, tx, uid); err != nil {
		if errors.Is(err, service.ErrClaimForbidden) {
			err = service.ErrGamificationForbidden
		}
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.user_settings(firebase_uid,show_on_leaderboard) VALUES($1,$2)
 ON CONFLICT (firebase_uid) DO UPDATE SET show_on_leaderboard=EXCLUDED.show_on_leaderboard`, uid, visible); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
