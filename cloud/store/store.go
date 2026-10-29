// Package store is the cloud server's database: SQLite in its data folder,
// its schema brought up to date by numbered migrations when it opens.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // registers "sqlite"
)

// migrations are the schema's steps, NNNN_name.sql, run once each in order.
// A released one never changes: a change is a new one.
//
//go:embed migrations/*.sql
var migrations embed.FS

// DB is the server's database.
type DB struct {
	*sql.DB
}

// Open opens the database in folder dir, made if missing, and migrates it.
func Open(ctx context.Context, dir string) (*DB, error) {
	path := filepath.Join(dir, "cloud.db")
	// WAL: readers don't wait for the writer. One connection writes at a
	// time (SQLite's own rule), busy_timeout queues the others.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	s := &DB{db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// SchemaVersion is the last migration run.
func (s *DB) SchemaVersion(ctx context.Context) (int, error) {
	var v int
	err := s.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
	return v, err
}

func (s *DB) migrate(ctx context.Context) error {
	if _, err := s.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`); err != nil {
		return fmt.Errorf("store: migrations table: %w", err)
	}
	done, err := s.SchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("store: schema version: %w", err)
	}
	steps, err := migrationSteps()
	if err != nil {
		return err
	}
	for _, m := range steps {
		if m.version <= done {
			continue
		}
		tx, err := s.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: migration %s: %w", m.name, err)
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

func migrationSteps() ([]migration, error) {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		num, _, ok := strings.Cut(e.Name(), "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil {
			return nil, fmt.Errorf("store: migration %s isn't named NNNN_name.sql", e.Name())
		}
		data, err := migrations.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, migration{v, e.Name(), string(data)})
	}
	slices.SortFunc(out, func(a, b migration) int { return a.version - b.version })
	return out, nil
}
