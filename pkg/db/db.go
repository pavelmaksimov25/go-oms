package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const connectTimeout = 10 * time.Second

// Connect opens a pgx connection pool against the given DSN and verifies
// reachability with a Ping. The caller owns the pool and must call Close.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// Migrate runs a single idempotent DDL block on startup. For this learning
// project we just CREATE TABLE IF NOT EXISTS — if migrations grow more
// complex, swap in golang-migrate.
func Migrate(ctx context.Context, pool *pgxpool.Pool, ddl string) error {
	_, err := pool.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
