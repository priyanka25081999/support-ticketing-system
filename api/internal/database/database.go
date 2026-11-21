// Package database opens the Postgres connection pool and runs migrations.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// driver
	_ "github.com/golang-migrate/migrate/v4/source/file"     // registers the file:// source
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool and retries briefly so the API can start while the DB boots.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	const attempts = 10
	for i := 1; i <= attempts; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		slog.Warn("database not ready, retrying", "attempt", i, "err", err)
		time.Sleep(2 * time.Second)
	}
	pool.Close()
	return nil, fmt.Errorf("connect to database: %w", err)
}

// Migrate applies all pending "up" migrations from dir.
func Migrate(databaseURL, dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	// golang-migrate's pgx v5 driver uses the pgx5:// scheme.
	dbURL := "pgx5://" + strings.TrimPrefix(databaseURL, "postgres://")

	m, err := migrate.New("file://"+absDir, dbURL)
	if err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
