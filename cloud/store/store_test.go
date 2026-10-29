package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/store"
)

// A new database gets the whole schema; opened again, nothing runs twice
// and what it holds stays.
func TestOpenMigratesOnce(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, store.Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	first, err := db.SchemaVersion(ctx)
	if err != nil || first < 3 {
		t.Fatalf("schema %d, %v", first, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO accounts (id, name, created) VALUES ('a1', 'Ada', '2026-10-29')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = store.Open(ctx, store.Config{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v, _ := db.SchemaVersion(ctx); v != first {
		t.Fatalf("schema %d after reopening, was %d", v, first)
	}
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM accounts WHERE id = 'a1'`).Scan(&name); err != nil || name != "Ada" {
		t.Fatalf("after reopening: %q, %v", name, err)
	}
	// Foreign keys hold: a device needs its account.
	if _, err := db.ExecContext(ctx, `INSERT INTO devices (id, account_id, name, kind, public_key, created, last_seen) VALUES ('d1', 'nobody', 'Mac', 'app', $1, '', '')`, []byte{0}); err == nil {
		t.Fatal("a device of no account was kept")
	}
}

// A Postgres URL that doesn't parse says so without showing it: it holds a
// password.
func TestPostgresURLStaysOutOfErrors(t *testing.T) {
	_, err := store.Open(context.Background(), store.Config{Postgres: "postgres://u:s3cret@host:5432/db\x7f"})
	if err == nil {
		t.Fatal("opened")
	}
	if got := err.Error(); len(got) == 0 || strings.Contains(got, "s3cret") {
		t.Fatalf("error: %q", got)
	}
}
