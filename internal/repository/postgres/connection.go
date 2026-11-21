package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// supabaseTransactionPoolerPort is Supavisor's transaction mode. It does not
// keep a server session per client, so named prepared statements break.
const supabaseTransactionPoolerPort = 6543

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan pool PostgreSQL: %w", err)
	}
	if cfg.ConnConfig.Port == supabaseTransactionPoolerPort {
		// Exec mode uses unnamed statements only, which transaction poolers
		// support; it costs an extra round trip per query.
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}

	pool, err := pgxpool.NewWithConfig(startupCtx, cfg)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan pool PostgreSQL: %w", err)
	}

	if err := pool.Ping(startupCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("menghubungi PostgreSQL: %w", err)
	}

	return pool, nil
}
