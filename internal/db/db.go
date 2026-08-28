// Package db wraps the PostgreSQL connection pool and migration runner.
package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps a pgx connection pool. Kept as a thin type alias so callers
// import internal/db instead of pgx directly, matching the "database" layer
// boundary in docs/architecture.md.
type Pool = pgxpool.Pool

func Connect(ctx context.Context, databaseURL string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

// Migrate applies every "*.up.sql" file under migrationsDir, in filename
// order, that hasn't already been recorded in schema_migrations. This is a
// deliberately minimal hand-rolled runner rather than a full migration
// library: this project only ever needs "run pending .sql files against
// Postgres in order," and a general-purpose migration library (e.g.
// golang-migrate) pulls in a large, mostly-unused dependency tree (Docker
// client, OpenTelemetry, multiple cloud SDKs) for every database driver it
// supports besides the one (Postgres) this project actually uses — see the
// "don't introduce unnecessary libraries" project rule.
func Migrate(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var versions []string
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		version := strings.TrimSuffix(e.Name(), ".up.sql")
		versions = append(versions, version)
		files[version] = filepath.Join(migrationsDir, e.Name())
	}
	sort.Strings(versions)

	for _, version := range versions {
		var alreadyApplied bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&alreadyApplied); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if alreadyApplied {
			continue
		}

		sqlBytes, err := os.ReadFile(files[version])
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
	}
	return nil
}
