package postgres_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/bootstrap"
	"github.com/fahrypratama17/EcoSnap-BE/internal/config"
	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

// readyForQueryDelay is the simulated network round trip charged to every
// completed PostgreSQL command cycle. Each handler has a 5 s budget, so a read
// path needing more than ~11 sequential cycles (cold connection state, extra
// prepares, serial queries) cannot answer in time, while a lean one does.
const readyForQueryDelay = 450 * time.Millisecond

// latencyConn delays delivery of backend ReadyForQuery ('Z') frames. It only
// understands the plaintext PostgreSQL v3 backend stream (type byte, int32
// length including itself, body), so it must wrap a loopback sslmode=disable
// connection. Frames are tracked incrementally and never copied; Read is only
// ever called from the pgx goroutine that owns the connection, so the parser
// state needs no lock. Close and the deadline setters may run concurrently.
type latencyConn struct {
	net.Conn
	delay time.Duration
	ready *atomic.Int64 // diagnostic count of ReadyForQuery frames seen

	closed    chan struct{}
	closeOnce sync.Once
	wake      chan struct{} // buffered(1): a deadline changed while Read may be waiting
	deadline  atomic.Int64  // read deadline in UnixNano; 0 means none

	header  [5]byte
	headerN int
	body    int64 // bytes of the current frame body still to arrive
	typ     byte
}

func newLatencyConn(conn net.Conn, delay time.Duration, ready *atomic.Int64) *latencyConn {
	return &latencyConn{Conn: conn, delay: delay, ready: ready, closed: make(chan struct{}), wake: make(chan struct{}, 1)}
}

// scan advances the frame parser over p and returns how many ReadyForQuery
// frames completed inside it.
func (c *latencyConn) scan(p []byte) (ready int, err error) {
	for len(p) > 0 {
		if c.body > 0 {
			skip := int64(len(p))
			if skip > c.body {
				skip = c.body
			}
			p = p[skip:]
			c.body -= skip
			if c.body == 0 && c.typ == 'Z' {
				ready++
			}
			continue
		}
		k := copy(c.header[c.headerN:], p)
		c.headerN += k
		p = p[k:]
		if c.headerN < len(c.header) {
			continue
		}
		c.headerN = 0
		size := binary.BigEndian.Uint32(c.header[1:])
		if size < 4 {
			return ready, io.ErrUnexpectedEOF
		}
		c.typ = c.header[0]
		c.body = int64(size - 4)
		if c.body == 0 && c.typ == 'Z' {
			ready++
		}
	}
	return ready, nil
}

func (c *latencyConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	ready, scanErr := c.scan(p[:n])
	if scanErr != nil {
		return n, scanErr
	}
	c.ready.Add(int64(ready))
	for ; ready > 0; ready-- {
		if waitErr := c.wait(); waitErr != nil {
			return 0, waitErr
		}
	}
	return n, err
}

// wait sleeps one delay but ends early when the connection is closed or a read
// deadline (set by pgx when its context ends) has passed.
func (c *latencyConn) wait() error {
	timer := time.NewTimer(c.delay)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return c.deadlineErr()
		case <-c.closed:
			return net.ErrClosed
		case <-c.wake:
			if err := c.deadlineErr(); err != nil {
				return err
			}
		}
	}
}

func (c *latencyConn) deadlineErr() error {
	if d := c.deadline.Load(); d != 0 && time.Now().UnixNano() >= d {
		return os.ErrDeadlineExceeded
	}
	return nil
}

func (c *latencyConn) setDeadline(t time.Time) {
	if t.IsZero() {
		c.deadline.Store(0)
		return
	}
	c.deadline.Store(t.UnixNano())
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *latencyConn) SetDeadline(t time.Time) error {
	c.setDeadline(t)
	return c.Conn.SetDeadline(t)
}

func (c *latencyConn) SetReadDeadline(t time.Time) error {
	c.setDeadline(t)
	return c.Conn.SetReadDeadline(t)
}

func (c *latencyConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func findMission(list entity.MissionList, id string) (entity.Mission, bool) {
	for _, m := range list.Missions {
		if m.ID == id {
			return m, true
		}
	}
	return entity.Mission{}, false
}

// requireMissionCatalog rejects a bare 200: the catalog must be a real one and
// first_deposit progress must match whether the fixture claim falls inside the
// returned week (not after the returned server time).
func requireMissionCatalog(t *testing.T, list *entity.MissionList, claimedAt time.Time) {
	t.Helper()
	if list == nil || len(list.Missions) == 0 || list.PeriodStart.IsZero() || !list.PeriodEnd.After(list.PeriodStart) || list.ServerTime.IsZero() {
		t.Fatalf("empty mission catalog: %+v", list)
	}
	for _, m := range list.Missions {
		if m.ID == "" || m.Title == "" || m.Target <= 0 || m.RewardXP <= 0 {
			t.Fatalf("incomplete mission: %+v", m)
		}
		switch m.Status {
		case entity.MissionInProgress, entity.MissionClaimable, entity.MissionClaimed:
		default:
			t.Fatalf("unknown mission status: %+v", m)
		}
	}
	wantProgress, wantStatus := int64(0), entity.MissionInProgress
	if !claimedAt.Before(list.PeriodStart) && claimedAt.Before(list.PeriodEnd) && !claimedAt.After(list.ServerTime) {
		wantProgress, wantStatus = 1, entity.MissionClaimable
	}
	first, ok := findMission(*list, "first_deposit")
	if !ok || first.Progress != wantProgress || first.Status != wantStatus || first.Unit != "deposit" {
		t.Fatalf("first_deposit want progress=%d status=%s (claimedAt=%s period=%s..%s): %+v ok=%v", wantProgress, wantStatus, claimedAt, list.PeriodStart, list.PeriodEnd, first, ok)
	}
}

// TestReadPathsUnderNetworkLatency drives the real HTTP stack against the
// real fixture database while every ReadyForQuery arrives 450 ms late. Reads
// must still finish inside their 5 s handler deadline; a read path that spends
// too many sequential round trips surfaces as the 504 seen in production.
func TestReadPathsUnderNetworkLatency(t *testing.T) {
	// readyFrames is diagnostic only (logged, never asserted).
	var readyFrames atomic.Int64
	pool := accountTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	depositFixture(ctx, t, pool)

	// Seed one genuinely claimed deposit (owner-a deposit, plain-user claim) on
	// the fast, undelayed fixture pool.
	deposits := service.NewDepositService(postgres.NewDepositRepository(pool))
	qr := service.NewDepositQRService(postgres.NewDepositRepository(pool))
	d, _, err := deposits.Create(ctx, "owner-a", "read-latency-claim", entity.DepositCreateRequest{
		Items: []entity.PointEstimateInput{{CategoryID: plastic, WeightKG: "1.500"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	code, err := qr.Issue(ctx, "owner-a", d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := qr.Claim(ctx, "plain-user", code.Token); err != nil {
		t.Fatal(err)
	}
	// Setup metadata read on the fast pool: the claim instant decides which
	// month and week the claim belongs to, so assertions derive from it.
	var claimedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT claimed_at FROM public.deposit_claims WHERE deposit_id=$1::uuid AND claimer_uid='plain-user'`, d.ID).Scan(&claimedAt); err != nil {
		t.Fatal(err)
	}

	// Clone the pool so only the endpoint traffic is delayed. One connection is
	// enough for every read path.
	cfg := pool.Config()
	cfg.MaxConns, cfg.MinConns = 1, 0
	if cfg.ConnConfig.TLSConfig != nil {
		t.Fatal("latency wrapper requires a plaintext (sslmode=disable) loopback connection")
	}
	dial := cfg.ConnConfig.DialFunc
	if dial == nil {
		dial = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
	}
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return newLatencyConn(conn, readyForQueryDelay, &readyFrames), nil
	}
	slowPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer slowPool.Close()
	// Warm only the connection (startup + auth); no SQL cache is primed, so the
	// endpoints below pay their own cold prepare/describe round trips.
	if err := slowPool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(bootstrap.NewServer(config.Config{}, homePackageVerifier{}, slowPool).Handler)
	defer server.Close()
	client := &http.Client{Timeout: 15 * time.Second}

	get := func(t *testing.T, path, uid string, out any) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+uid)
		readyBefore := readyFrames.Load()
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		content, err := io.ReadAll(resp.Body)
		elapsed := time.Since(start)
		t.Logf("GET %s as %s: status=%d elapsed=%s ready_frames=%d", path, uid, resp.StatusCode, elapsed, readyFrames.Load()-readyBefore)
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s: status=%d want=200 elapsed=%s err=%v body=%s", path, resp.StatusCode, elapsed, err, content)
		}
		if err := json.Unmarshal(content, out); err != nil {
			t.Fatalf("invalid JSON: %v body=%s", err, content)
		}
	}

	t.Run("home_plain_user", func(t *testing.T) {
		var response struct {
			Data homePackageWire `json:"data"`
		}
		get(t, "/api/v1/me/home", "plain-user", &response)
		h := response.Data
		if h.UserName != "User" || h.ServerTime.IsZero() || h.Month.From.IsZero() || !h.Month.To.Equal(h.ServerTime) {
			t.Fatalf("home identity/month: %+v", h)
		}
		// The fixture claim is "now"; whether it still counts for the month
		// depends only on the month window the server returned.
		wantMonthly := int64(0)
		if !claimedAt.Before(h.Month.From) && !claimedAt.After(h.Month.To) {
			wantMonthly = 1
		}
		if h.Wallet.Balance != 225 || h.MonthlyDeposits != wantMonthly || h.DepositsClaimed != 1 || h.TotalWeightKG != "1.500" {
			t.Fatalf("home counters (claimedAt=%s want monthly=%d): %+v", claimedAt, wantMonthly, h)
		}
		if len(h.RecentActivity) != 1 || h.RecentActivity[0].Amount != 225 || h.RecentActivity[0].Deposit == nil || h.RecentActivity[0].Deposit.ID != d.ID {
			t.Fatalf("home recent activity: %+v", h.RecentActivity)
		}
		requireMissionCatalog(t, h.Missions, claimedAt)
	})

	t.Run("home_mitra_owner", func(t *testing.T) {
		var response struct {
			Data homePackageWire `json:"data"`
		}
		get(t, "/api/v1/me/home", "owner-a", &response)
		h := response.Data
		if h.UserName != "Owner" || h.ServerTime.IsZero() || h.Missions != nil || h.Wallet.Balance != 0 {
			t.Fatalf("mitra home: %+v", h)
		}
	})

	t.Run("missions_plain_user", func(t *testing.T) {
		var response struct {
			Data entity.MissionList `json:"data"`
		}
		get(t, "/api/v1/me/missions", "plain-user", &response)
		requireMissionCatalog(t, &response.Data, claimedAt)
	})

	// Healthy neighbours: they already pass today and must keep doing so with
	// the same latency; they guard against fixing one endpoint at another's cost.
	t.Run("wallet_plain_user", func(t *testing.T) {
		var response struct {
			Data entity.Wallet `json:"data"`
		}
		get(t, "/api/v1/me/wallet", "plain-user", &response)
		if response.Data.Balance != 225 || response.Data.TotalEarned != 225 || response.Data.TotalSpent != 0 {
			t.Fatalf("wallet: %+v", response.Data)
		}
	})

	t.Run("badges_plain_user", func(t *testing.T) {
		var response struct {
			Data entity.BadgeList `json:"data"`
		}
		get(t, "/api/v1/me/badges", "plain-user", &response)
		b := response.Data
		if b.ServerTime.IsZero() || b.ClaimedDeposits != 1 || b.ClaimedWeightGrams != 1500 || len(b.Badges) == 0 {
			t.Fatalf("badge totals: %+v", b)
		}
		foundFirst := false
		for _, badge := range b.Badges {
			if badge.ID == "" || badge.Target <= 0 || badge.Earned != (badge.UnlockedAt != nil) {
				t.Fatalf("badge state: %+v", badge)
			}
			if badge.ID == "deposit_1" {
				foundFirst = true
				if !badge.Earned || badge.Progress != 1 || !badge.UnlockedAt.Equal(claimedAt) {
					t.Fatalf("deposit_1 must unlock at the fixture claim %s: %+v", claimedAt, badge)
				}
			}
		}
		if !foundFirst {
			t.Fatal("deposit_1 badge missing")
		}
	})

	if acquired := slowPool.Stat().AcquiredConns(); acquired != 0 {
		t.Fatalf("connection retained after requests: %d", acquired)
	}
}
