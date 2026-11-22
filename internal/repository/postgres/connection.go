package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// supabaseTransactionPoolerPort is Supavisor's transaction mode. It does not
// keep a server session per client, so named prepared statements break.
const supabaseTransactionPoolerPort = 6543

// Deterministic pool defaults. They never depend on the host CPU count and are
// kept small because Supabase pooler connections are a shared, limited resource.
// Each can be overridden through the native pgxpool pool_* connection options
// (URI query or keyword/value form), or connect_timeout for connectTimeout.
const (
	defaultMaxConns          int32 = 4
	defaultMinConns          int32 = 0
	defaultMaxConnLifetime         = 30 * time.Minute
	defaultMaxConnIdleTime         = 5 * time.Minute
	defaultHealthCheckPeriod       = time.Minute
	defaultConnectTimeout          = 10 * time.Second
	startupTimeout                 = 10 * time.Second
)

// errInvalidPoolConfig and its siblings carry no connection string so that
// credentials cannot leak through logs.
var errInvalidPoolConfig = errors.New("konfigurasi koneksi PostgreSQL tidak valid")

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	startupCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	cfg, err := buildPoolConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan pool PostgreSQL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(startupCtx, cfg)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan pool PostgreSQL: %w", redactConnectError(err))
	}

	if err := pool.Ping(startupCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("menghubungi PostgreSQL: %w", redactConnectError(err))
	}

	return pool, nil
}

// buildPoolConfig parses databaseURL with pgx's native parsers and applies the
// defaults for every pool option the URL does not set explicitly.
func buildPoolConfig(databaseURL string) (*pgxpool.Config, error) {
	// pgx.ParseConfig keeps unknown keys, including pool_*, in RuntimeParams;
	// pgxpool.ParseConfig strips them, so explicit options are detected here.
	// This runs once at startup, not per query.
	raw, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, errInvalidPoolConfig
	}
	_, explicitMax := raw.RuntimeParams["pool_max_conns"]
	_, explicitMin := raw.RuntimeParams["pool_min_conns"]
	_, explicitLifetime := raw.RuntimeParams["pool_max_conn_lifetime"]
	_, explicitIdle := raw.RuntimeParams["pool_max_conn_idle_time"]
	_, explicitHealth := raw.RuntimeParams["pool_health_check_period"]

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// pgxpool's parse error text (e.g. a bad pool_max_conns value) is not
		// guaranteed free of connection details, so it is not wrapped.
		return nil, errInvalidPoolConfig
	}
	if !explicitMax {
		cfg.MaxConns = defaultMaxConns
	}
	if !explicitMin {
		cfg.MinConns = defaultMinConns
	}
	if !explicitLifetime {
		cfg.MaxConnLifetime = defaultMaxConnLifetime
	}
	if !explicitIdle {
		cfg.MaxConnIdleTime = defaultMaxConnIdleTime
	}
	if !explicitHealth {
		cfg.HealthCheckPeriod = defaultHealthCheckPeriod
	}
	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = defaultConnectTimeout
	}
	if cfg.HealthCheckPeriod <= 0 || cfg.MaxConnLifetime < 0 || cfg.MaxConnIdleTime < 0 {
		return nil, fmt.Errorf("%w: durasi pool di luar rentang", errInvalidPoolConfig)
	}
	if cfg.MaxConns < 1 || cfg.MinConns < 0 {
		return nil, fmt.Errorf("%w: pool_max_conns/pool_min_conns di luar rentang", errInvalidPoolConfig)
	}
	if cfg.MinConns > cfg.MaxConns {
		return nil, fmt.Errorf("%w: pool_min_conns melebihi pool_max_conns", errInvalidPoolConfig)
	}
	if cfg.ConnConfig.Port == supabaseTransactionPoolerPort {
		// Exec mode uses unnamed statements only, which transaction poolers
		// support; it costs an extra round trip per query.
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}
	return cfg, nil
}

// redactConnectError drops pgx's error text, which can carry host, user and
// database details, while keeping context cancellation and the SQLSTATE.
func redactConnectError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, context.Canceled):
		return context.Canceled
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return fmt.Errorf("koneksi ditolak server (SQLSTATE %s)", pgErr.Code)
	}
	return errors.New("koneksi gagal")
}
