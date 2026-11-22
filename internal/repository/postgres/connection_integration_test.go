package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
)

// localcfgConnString derives a connection string from the disposable local
// cluster configured for the other integration tests, capped at one connection.
func localcfgConnString(t *testing.T) string {
	t.Helper()
	base := accountTestDatabase(t).Config().ConnConfig.ConnString()
	if strings.Contains(base, "://") {
		sep := "?"
		if strings.Contains(base, "?") {
			sep = "&"
		}
		return base + sep + "pool_max_conns=1"
	}
	return base + " pool_max_conns=1"
}

func TestNewPoolExhaustionAndRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.NewPool(ctx, localcfgConnString(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if got := pool.Config().MaxConns; got != 1 {
		t.Fatalf("MaxConns = %d, want 1", got)
	}

	held, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, waitCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, err = pool.Acquire(waitCtx)
	waitCancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		held.Release()
		t.Fatalf("second acquire err = %v, want deadline exceeded", err)
	}

	held.Release()
	again, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	again.Release()
}

func TestNewPoolCanceledQueryFreesConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := postgres.NewPool(ctx, localcfgConnString(t))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	queryCtx, queryCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, err = pool.Exec(queryCtx, "SELECT pg_sleep(10)")
	queryCancel()
	if err == nil {
		t.Fatal("pg_sleep should have been canceled")
	}

	var one int
	if err := pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatalf("query after cancel: %v (%d)", err, one)
	}
	if n := pool.Stat().AcquiredConns(); n != 0 {
		t.Fatalf("acquired conns = %d, want 0", n)
	}
}
