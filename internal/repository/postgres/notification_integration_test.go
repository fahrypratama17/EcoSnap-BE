package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func notifCount(ctx context.Context, t *testing.T, pool *pgxpool.Pool, uid, typ string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.notifications WHERE recipient_uid=$1 AND ($2='' OR event_type=$2)`, uid, typ).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestNotificationIntegration(t *testing.T) {
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)
	repo := postgres.NewNotificationRepository(pool)
	svc := service.NewNotificationService(repo)
	now := time.Now()

	t.Run("deposit_claim_notifies_claimant_and_owner_once", func(t *testing.T) {
		insertGamificationClaim(ctx, t, pool, 901, "plain-user", 1000, now)
		if notifCount(ctx, t, pool, "plain-user", "deposit_credited") != 1 || notifCount(ctx, t, pool, "owner-a", "deposit_claimed") != 1 {
			t.Fatal("expected one notification per recipient")
		}
		if notifCount(ctx, t, pool, "owner-a", "deposit_credited") != 0 || notifCount(ctx, t, pool, "plain-user", "deposit_claimed") != 0 {
			t.Fatal("roles must not cross")
		}
		// Replay of the same source cannot double: the unique key absorbs it.
		if _, err := pool.Exec(ctx, `SELECT public.notify_recipient('plain-user','deposit_credited','a0000000-0000-4000-8000-000000000901','x','x','deposit','x')`); err != nil {
			t.Fatal(err)
		}
		if notifCount(ctx, t, pool, "plain-user", "deposit_credited") != 1 {
			t.Fatal("replay duplicated")
		}
		page, err := svc.List(ctx, "owner-a", service.NotificationQuery{Limit: 10})
		if err != nil || len(page.Items) != 1 || page.Items[0].Target.Type != "deposit" || page.Items[0].SeenAt != nil || page.UnreadCount != 1 {
			t.Fatalf("owner list: %+v %v", page, err)
		}
		if strings.Contains(page.Items[0].Body, "plain-user") {
			t.Fatal("claimant identity leaked to owner")
		}
	})

	t.Run("preference_off_suppresses_new_but_keeps_inbox", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE public.user_settings SET notification_enabled=false WHERE firebase_uid='plain-user'`); err != nil {
			t.Fatal(err)
		}
		insertGamificationClaim(ctx, t, pool, 902, "plain-user", 1000, now)
		if notifCount(ctx, t, pool, "plain-user", "") != 1 {
			t.Fatal("suppressed event created a row or old one vanished")
		}
		if notifCount(ctx, t, pool, "owner-a", "deposit_claimed") != 2 {
			t.Fatal("owner preference is independent")
		}
		if _, err := pool.Exec(ctx, `UPDATE public.user_settings SET notification_enabled=true WHERE firebase_uid='plain-user'`); err != nil {
			t.Fatal(err)
		}
		if page, err := svc.List(ctx, "plain-user", service.NotificationQuery{Limit: 10}); err != nil || len(page.Items) != 1 {
			t.Fatalf("existing inbox still readable: %+v %v", page, err)
		}
	})

	t.Run("rollback_removes_notifications", func(t *testing.T) {
		var total int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM public.notifications`).Scan(&total)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.deposits(id,partner_id,created_by_uid,idempotency_key,request_hash,total_weight_grams,total_points,uses_demo_rates,rates_as_of,status)
 VALUES('a0000000-0000-4000-8000-000000000903','20000000-0000-4000-8000-000000000001','owner-a','rollback-0001',repeat('a',64),1000,0,true,now(),'claimed');
 INSERT INTO public.deposit_qr_tokens(id,deposit_id,token_hash,issued_by_uid,issued_at,expires_at,used_at) VALUES('b0000000-0000-4000-8000-000000000903','a0000000-0000-4000-8000-000000000903',md5('r'),'owner-a',now(),now()+interval '1 day',now());
 INSERT INTO public.deposit_claims(deposit_id,qr_token_id,claimer_uid,points,claimed_at) VALUES('a0000000-0000-4000-8000-000000000903','b0000000-0000-4000-8000-000000000903','user-2',5,now())`)
		if err != nil {
			t.Fatal(err)
		}
		var inTx int
		_ = tx.QueryRow(ctx, `SELECT count(*) FROM public.notifications`).Scan(&inTx)
		if inTx != total+2 {
			t.Fatalf("in-tx notifications %d want %d", inTx, total+2)
		}
		if err = tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		var after int
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM public.notifications`).Scan(&after)
		if after != total {
			t.Fatalf("rollback left notifications: %d want %d", after, total)
		}
	})

	t.Run("redemption_and_mission_events", func(t *testing.T) {
		_, err := pool.Exec(ctx, `INSERT INTO public.rewards(id,name,cost_points,stock) VALUES('c0000000-0000-4000-8000-000000000001','Kopi',100,5);
 INSERT INTO public.redemptions(id,user_uid,reward_id,idempotency_key,reward_name,cost_points,voucher_code,created_at,expires_at)
 VALUES('d0000000-0000-4000-8000-000000000001','user-2','c0000000-0000-4000-8000-000000000001','redeem-notif-1','Kopi',100,'NOTIF-0001',now(),now()+interval '1 day')`)
		if err != nil {
			t.Fatal(err)
		}
		page, err := svc.List(ctx, "user-2", service.NotificationQuery{Limit: 10})
		if err != nil || len(page.Items) != 1 || page.Items[0].Type != "voucher_issued" || page.Items[0].Target.ID != "d0000000-0000-4000-8000-000000000001" {
			t.Fatalf("voucher: %+v %v", page, err)
		}
		if strings.Contains(page.Items[0].Body+page.Items[0].Title, "NOTIF-0001") {
			t.Fatal("voucher code leaked")
		}
		start, end := service.WeekPeriod(now.In(service.WIB))
		_, err = pool.Exec(ctx, `INSERT INTO public.mission_claims(user_uid,mission_id,period_start,period_end,progress,target,xp,claimed_at) VALUES('user-2','first_deposit',$1,$2,1,1,50,$3)`, start, end, now)
		if err != nil {
			t.Fatal(err)
		}
		var src string
		if err = pool.QueryRow(ctx, `SELECT source_id FROM public.notifications WHERE recipient_uid='user-2' AND event_type='mission_xp_claimed'`).Scan(&src); err != nil || src != "first_deposit:"+start.In(service.WIB).Format("2006-01-02") {
			t.Fatalf("mission source %q %v", src, err)
		}
	})

	t.Run("ownership_inactive_and_idempotent_read", func(t *testing.T) {
		page, _ := svc.List(ctx, "owner-a", service.NotificationQuery{Limit: 10})
		id := page.Items[0].ID
		if _, err := svc.MarkRead(ctx, "user-3", id); !errors.Is(err, service.ErrNotificationNotFound) {
			t.Fatalf("foreign: %v", err)
		}
		if _, err := svc.MarkRead(ctx, "owner-a", "00000000-0000-4000-8000-000000000000"); !errors.Is(err, service.ErrNotificationNotFound) {
			t.Fatalf("missing: %v", err)
		}
		first, err := svc.MarkRead(ctx, "owner-a", id)
		if err != nil || first.SeenAt == nil {
			t.Fatalf("read: %+v %v", first, err)
		}
		second, err := svc.MarkRead(ctx, "owner-a", strings.ToUpper(id))
		if err != nil || !second.SeenAt.Equal(*first.SeenAt) {
			t.Fatalf("seen_at moved: %+v %v", second, err)
		}
		after, _ := svc.List(ctx, "owner-a", service.NotificationQuery{Limit: 10})
		if after.UnreadCount != int64(len(page.Items))-1 {
			t.Fatalf("unread %d", after.UnreadCount)
		}
		if _, err = pool.Exec(ctx, `UPDATE public.accounts SET status='suspended' WHERE firebase_uid='user-3'`); err != nil {
			t.Fatal(err)
		}
		if _, err = svc.List(ctx, "user-3", service.NotificationQuery{Limit: 10}); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("inactive list: %v", err)
		}
		if _, err = svc.MarkRead(ctx, "user-3", id); !errors.Is(err, service.ErrAccountForbidden) {
			t.Fatalf("inactive read: %v", err)
		}
	})

	t.Run("paging_and_unread_filter", func(t *testing.T) {
		for i := range 5 {
			if _, err := pool.Exec(ctx, `SELECT public.notify_recipient('user-3','deposit_credited',$1,'t','b','deposit','x')`, "page-"+string(rune('a'+i))); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := pool.Exec(ctx, `UPDATE public.accounts SET status='active' WHERE firebase_uid='user-3'`); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		cursor := ""
		var last entity.Notification
		for {
			p, err := svc.List(ctx, "user-3", service.NotificationQuery{Limit: 2, Cursor: cursor})
			if err != nil {
				t.Fatal(err)
			}
			if p.UnreadCount != 5 {
				t.Fatalf("unread %d", p.UnreadCount)
			}
			for _, n := range p.Items {
				if seen[n.ID] {
					t.Fatal("duplicate across pages")
				}
				seen[n.ID] = true
				last = n
			}
			if p.NextCursor == "" {
				break
			}
			cursor = p.NextCursor
		}
		if len(seen) != 5 {
			t.Fatalf("saw %d", len(seen))
		}
		if _, err := svc.MarkRead(ctx, "user-3", last.ID); err != nil {
			t.Fatal(err)
		}
		p, err := svc.List(ctx, "user-3", service.NotificationQuery{Limit: 10, UnreadOnly: true})
		if err != nil || len(p.Items) != 4 || p.UnreadCount != 4 {
			t.Fatalf("unread filter: %d %d %v", len(p.Items), p.UnreadCount, err)
		}
		if _, err = svc.List(ctx, "user-3", service.NotificationQuery{Limit: 10, Cursor: "bad"}); err == nil {
			t.Fatal("bad cursor accepted")
		}
	})

	t.Run("missing_settings_fail_closed", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `SELECT public.notify_recipient('ghost','deposit_credited','ghost-1','t','b','deposit','x')`); err != nil {
			t.Fatal(err)
		}
		if notifCount(ctx, t, pool, "ghost", "") != 0 {
			t.Fatal("notification created without settings row")
		}
	})

	t.Run("rls_enabled", func(t *testing.T) {
		var rls bool
		if err := pool.QueryRow(ctx, `SELECT relrowsecurity FROM pg_class WHERE oid='public.notifications'::regclass`).Scan(&rls); err != nil || !rls {
			t.Fatalf("rls=%v %v", rls, err)
		}
	})
}
