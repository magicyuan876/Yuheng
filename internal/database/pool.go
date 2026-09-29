package database

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Connection-pool defaults. database/sql's own default is unlimited open
// connections, so a burst of concurrent requests (each chat turn, ingestion
// task and scheduler tick holds one) could open connections until PostgreSQL
// answered "too many clients" — for every service sharing that server, not
// only this one. 50 leaves half of PostgreSQL's default max_connections=100
// for the migration runner, psql sessions, Langfuse (which shares the
// bundled server) and other replicas.
const (
	DefaultMaxOpenConns    = 50
	DefaultMaxIdleConns    = 10
	DefaultConnMaxLifetime = 10 * time.Minute
)

// PoolConfig is the size and lifetime of the *sql.DB connection pool.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// PoolConfigFromEnv reads DB_MAX_OPEN_CONNS, DB_MAX_IDLE_CONNS and
// DB_CONN_MAX_LIFETIME_MINUTES, falling back to the defaults for unset or empty
// variables. A value that is not a positive integer is an error rather than a
// silent fallback: an operator who typed a limit wants it applied or to be told
// why not. An idle count above the open count is lowered to it (the pool would
// do the same silently); the returned warning says so.
func PoolConfigFromEnv() (cfg PoolConfig, warning string, err error) {
	cfg = PoolConfig{
		MaxOpenConns:    DefaultMaxOpenConns,
		MaxIdleConns:    DefaultMaxIdleConns,
		ConnMaxLifetime: DefaultConnMaxLifetime,
	}
	if cfg.MaxOpenConns, err = positiveEnvInt("DB_MAX_OPEN_CONNS", cfg.MaxOpenConns); err != nil {
		return cfg, "", err
	}
	if cfg.MaxIdleConns, err = positiveEnvInt("DB_MAX_IDLE_CONNS", cfg.MaxIdleConns); err != nil {
		return cfg, "", err
	}
	minutes, err := positiveEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", int(DefaultConnMaxLifetime/time.Minute))
	if err != nil {
		return cfg, "", err
	}
	cfg.ConnMaxLifetime = time.Duration(minutes) * time.Minute

	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		warning = fmt.Sprintf("DB_MAX_IDLE_CONNS=%d exceeds DB_MAX_OPEN_CONNS=%d; using %d",
			cfg.MaxIdleConns, cfg.MaxOpenConns, cfg.MaxOpenConns)
		cfg.MaxIdleConns = cfg.MaxOpenConns
	}
	return cfg, warning, nil
}

func positiveEnvInt(name string, def int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s=%q is not a positive integer", name, raw)
	}
	return n, nil
}
