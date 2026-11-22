package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestBuildPoolConfigDefaultsAreDeterministic(t *testing.T) {
	cfg, err := buildPoolConfig("postgres://u:pw@127.0.0.1:5432/db?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxConns != 4 || cfg.MinConns != 0 {
		t.Fatalf("conns = %d/%d, want 4/0", cfg.MaxConns, cfg.MinConns)
	}
	if cfg.MaxConnLifetime != 30*time.Minute || cfg.MaxConnIdleTime != 5*time.Minute || cfg.HealthCheckPeriod != time.Minute {
		t.Fatalf("durations = %v/%v/%v", cfg.MaxConnLifetime, cfg.MaxConnIdleTime, cfg.HealthCheckPeriod)
	}
	if cfg.ConnConfig.ConnectTimeout != 10*time.Second {
		t.Fatalf("connect timeout = %v", cfg.ConnConfig.ConnectTimeout)
	}
}

func TestBuildPoolConfigRespectsExplicitOptions(t *testing.T) {
	for name, dsn := range map[string]string{
		"uri":     "postgres://u:pw@127.0.0.1:5432/db?sslmode=disable&pool_max_conns=9&pool_min_conns=2&pool_max_conn_lifetime=1h&pool_max_conn_idle_time=2m&pool_health_check_period=7s&connect_timeout=3",
		"keyword": "host=127.0.0.1 port=5432 user=u password=pw dbname=db sslmode=disable pool_max_conns=9 pool_min_conns=2 pool_max_conn_lifetime=1h pool_max_conn_idle_time=2m pool_health_check_period=7s connect_timeout=3",
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := buildPoolConfig(dsn)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.MaxConns != 9 || cfg.MinConns != 2 {
				t.Fatalf("conns = %d/%d, want 9/2", cfg.MaxConns, cfg.MinConns)
			}
			if cfg.MaxConnLifetime != time.Hour || cfg.MaxConnIdleTime != 2*time.Minute || cfg.HealthCheckPeriod != 7*time.Second {
				t.Fatalf("durations = %v/%v/%v", cfg.MaxConnLifetime, cfg.MaxConnIdleTime, cfg.HealthCheckPeriod)
			}
			if cfg.ConnConfig.ConnectTimeout != 3*time.Second {
				t.Fatalf("connect timeout = %v", cfg.ConnConfig.ConnectTimeout)
			}
		})
	}
}

func TestBuildPoolConfigKeepsTransactionPoolerExecMode(t *testing.T) {
	cfg, err := buildPoolConfig("postgres://u:pw@127.0.0.1:6543/db?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.DefaultQueryExecMode != pgx.QueryExecModeExec {
		t.Fatalf("exec mode = %v", cfg.ConnConfig.DefaultQueryExecMode)
	}
}

func TestBuildPoolConfigRejectsInvalidWithoutLeakingSecret(t *testing.T) {
	const secret = "s3cr3t-pw"
	for name, dsn := range map[string]string{
		"max zero":      "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_max_conns=0",
		"max negative":  "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_max_conns=-1",
		"max garbage":   "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_max_conns=abc",
		"min negative":  "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_min_conns=-1",
		"min above max": "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_max_conns=2&pool_min_conns=3",
		"lifetime":      "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_max_conn_lifetime=forever",
		"health zero":   "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_health_check_period=0",
		"health neg":    "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_health_check_period=-1s",
		"lifetime neg":  "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_max_conn_lifetime=-1s",
		"idle neg":      "postgres://u:" + secret + "@127.0.0.1:5432/db?pool_max_conn_idle_time=-1s",
		"malformed":     "postgres://u:" + secret + "@127.0.0.1:notaport/db",
		"keyword":       "host=127.0.0.1 password=" + secret + " pool_max_conns=abc",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := buildPoolConfig(dsn)
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error leaks secret: %v", err)
			}
		})
	}
}

func TestNewPoolConfigErrorRedacted(t *testing.T) {
	const secret = "s3cr3t-pw"
	_, err := NewPool(context.Background(), "postgres://u:"+secret+"@127.0.0.1:notaport/db")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}
}

func TestRedactConnectErrorKeepsContextErrors(t *testing.T) {
	wrapped := errors.Join(errors.New("host=h password=s3cr3t-pw"), context.DeadlineExceeded)
	got := redactConnectError(wrapped)
	if !errors.Is(got, context.DeadlineExceeded) || strings.Contains(got.Error(), "s3cr3t") {
		t.Fatalf("got %v", got)
	}
	if !errors.Is(redactConnectError(context.Canceled), context.Canceled) {
		t.Fatal("canceled lost")
	}
}
