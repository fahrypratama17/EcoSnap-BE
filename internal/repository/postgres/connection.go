package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(startupCtx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan pool PostgreSQL: %w", err)
	}

	if err := pool.Ping(startupCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("menghubungi PostgreSQL: %w", err)
	}

	return pool, nil
}
