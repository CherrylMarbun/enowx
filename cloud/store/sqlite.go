//go:build !nosqlite

package store

// SQLite, for tests and for a server run without Postgres. The image
// builds without it (-tags nosqlite): its driver is a large package to
// compile, and the hosted cloud runs on Postgres.
import _ "modernc.org/sqlite" // registers "sqlite"
