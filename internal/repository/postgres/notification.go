package postgres

import (
	"context"
	"errors"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationRepository struct{ pool *pgxpool.Pool }

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

const notificationColumns = `id::text,event_type,title,body,target_type,target_id,created_at,seen_at`

func scanNotification(row pgx.Row) (entity.Notification, error) {
	var n entity.Notification
	err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Body, &n.Target.Type, &n.Target.ID, &n.CreatedAt, &n.SeenAt)
	return n, err
}

// List reads the page and the unread count in one repeatable-read snapshot, so
// the count is consistent with the page.
func (r *NotificationRepository) List(ctx context.Context, uid string, unreadOnly bool, limit int, after *service.Cursor) ([]entity.Notification, int64, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return nil, 0, err
	}
	at, id := cursorArgs(after)
	rows, err := tx.Query(ctx, `SELECT `+notificationColumns+` FROM public.notifications
 WHERE recipient_uid=$1 AND (NOT $5 OR seen_at IS NULL) AND ($2::timestamptz IS NULL OR (created_at,id)<($2,$3::uuid))
 ORDER BY created_at DESC,id DESC LIMIT $4`, uid, at, id, limit, unreadOnly)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []entity.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()
	unread, err := unreadCountTx(ctx, tx, uid)
	if err != nil {
		return nil, 0, err
	}
	return out, unread, nil
}

// unreadCountSQL counts the caller's unread notifications; arg $1 is the uid.
const unreadCountSQL = `SELECT count(*) FROM public.notifications WHERE recipient_uid=$1 AND seen_at IS NULL`

// unreadCountTx is the caller's total unread notifications, whatever the page.
func unreadCountTx(ctx context.Context, q rowQuerier, uid string) (int64, error) {
	var unread int64
	err := q.QueryRow(ctx, unreadCountSQL, uid).Scan(&unread)
	return unread, err
}

func (r *NotificationRepository) MarkRead(ctx context.Context, uid, id string) (entity.Notification, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return entity.Notification{}, err
	}
	defer rollbackLater(ctx, tx)
	if err = favoriteAccount(ctx, tx, uid); err != nil {
		return entity.Notification{}, err
	}
	// Only the first read writes; later reads keep the original timestamp.
	if _, err = tx.Exec(ctx, `UPDATE public.notifications SET seen_at=clock_timestamp() WHERE id=$1::uuid AND recipient_uid=$2 AND seen_at IS NULL`, id, uid); err != nil {
		return entity.Notification{}, err
	}
	n, err := scanNotification(tx.QueryRow(ctx, `SELECT `+notificationColumns+` FROM public.notifications WHERE id=$1::uuid AND recipient_uid=$2`, id, uid))
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Notification{}, service.ErrNotificationNotFound
	}
	if err != nil {
		return entity.Notification{}, err
	}
	return n, tx.Commit(ctx)
}
