package store

import (
	"context"
	"testing"
)

// A new database gets the whole schema; opened again, nothing runs twice
// and what it holds stays.
func TestOpenMigratesOnce(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	steps, _ := migrationSteps()
	want := steps[len(steps)-1].version
	if v, err := db.SchemaVersion(ctx); err != nil || v != want {
		t.Fatalf("schema %d, %v; want %d", v, err, want)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO accounts (id, name, created) VALUES ('a1', 'Ada', '2026-10-29')`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM accounts WHERE id = 'a1'`).Scan(&name); err != nil || name != "Ada" {
		t.Fatalf("after reopening: %q, %v", name, err)
	}
	// Foreign keys hold: a device needs its account.
	if _, err := db.ExecContext(ctx, `INSERT INTO devices (id, account_id, name, kind, public_key, created, last_seen) VALUES ('d1', 'nobody', 'Mac', 'app', x'00', '', '')`); err == nil {
		t.Fatal("a device of no account was kept")
	}
}
