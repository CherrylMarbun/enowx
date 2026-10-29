package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enowx/enowx/cloud/api"
)

// ErrNotFound is the error for what isn't there, or isn't the asker's.
var ErrNotFound = errors.New("store: not found")

// tokenPrefix marks a device's token, so a leaked one is recognizable
// (secret scanners, the vault's redaction).
const tokenPrefix = "enxc_"

// seenEvery is how often a device's last_seen is written as it uses its
// token: not at every request.
const seenEvery = time.Minute

func newID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

// stampLayout keeps every digit, so stamps compare as text in the
// database the way the times they stand for do.
const stampLayout = "2006-01-02T15:04:05.000000000Z"

func stamp(t time.Time) string { return t.UTC().Format(stampLayout) }

func parseStamp(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// GitHubAccount is the account of GitHub user id, made or updated with
// what GitHub says of them now.
func (s *DB) GitHubAccount(ctx context.Context, id int64, login, name, email, avatar string) (api.Account, error) {
	if name == "" {
		name = login
	}
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return api.Account{}, err
	}
	defer tx.Rollback()
	var accID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE github_id = $1`, id).Scan(&accID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		accID = newID()
		_, err = tx.ExecContext(ctx, `INSERT INTO accounts (id, name, email, avatar, github_id, github_login, created) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			accID, name, email, avatar, id, login, stamp(now()))
		if err == nil {
			err = audit(ctx, tx, "", accID, "account.created", "github:"+login)
		}
	case err == nil:
		_, err = tx.ExecContext(ctx, `UPDATE accounts SET name = $1, email = $2, avatar = $3, github_login = $4 WHERE id = $5`, name, email, avatar, login, accID)
	}
	if err != nil {
		return api.Account{}, fmt.Errorf("store: github account: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return api.Account{}, err
	}
	return s.Account(ctx, accID)
}

// Account is account id.
func (s *DB) Account(ctx context.Context, id string) (api.Account, error) {
	var a api.Account
	var created string
	err := s.QueryRowContext(ctx, `SELECT id, name, email, avatar, github_login, created FROM accounts WHERE id = $1`, id).
		Scan(&a.ID, &a.Name, &a.Email, &a.Avatar, &a.GitHub, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	a.Created = parseStamp(created)
	return a, err
}

// AddDevice signs a device in to account accID: its name, kind (api.DeviceApp
// or api.DeviceHost) and public key. It returns the device and its token,
// which only the device gets: the database keeps its hash.
func (s *DB) AddDevice(ctx context.Context, accID, name, kind string, publicKey []byte) (api.Device, string, error) {
	if kind != api.DeviceApp && kind != api.DeviceHost {
		return api.Device{}, "", fmt.Errorf("store: a device is %q or %q, not %q", api.DeviceApp, api.DeviceHost, kind)
	}
	if len(publicKey) != 32 {
		return api.Device{}, "", errors.New("store: a device's public key is 32 bytes (X25519)")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = kind
	}
	var raw [32]byte
	rand.Read(raw[:])
	token := tokenPrefix + base64.RawURLEncoding.EncodeToString(raw[:])
	d := api.Device{ID: newID(), Name: name, Kind: kind, PublicKey: publicKey, Created: now()}
	d.LastSeen = d.Created
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return api.Device{}, "", err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO devices (id, account_id, name, kind, public_key, created, last_seen) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.ID, accID, d.Name, d.Kind, d.PublicKey, stamp(d.Created), stamp(d.LastSeen)); err != nil {
		return api.Device{}, "", fmt.Errorf("store: add device: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tokens (hash, device_id, created) VALUES ($1, $2, $3)`, hashToken(token), d.ID, stamp(d.Created)); err != nil {
		return api.Device{}, "", err
	}
	if err := audit(ctx, tx, "", accID, "device.added", d.Kind+": "+d.Name); err != nil {
		return api.Device{}, "", err
	}
	return d, token, tx.Commit()
}

// Me is who token belongs to: its account and device, ErrNotFound for a
// token revoked or never given.
func (s *DB) Me(ctx context.Context, token string) (api.Me, error) {
	var me api.Me
	var accCreated, devCreated, seen string
	err := s.QueryRowContext(ctx, `SELECT a.id, a.name, a.email, a.avatar, a.github_login, a.created,
			d.id, d.name, d.kind, d.public_key, d.created, d.last_seen
		FROM tokens t JOIN devices d ON d.id = t.device_id JOIN accounts a ON a.id = d.account_id
		WHERE t.hash = $1`, hashToken(token)).
		Scan(&me.Account.ID, &me.Account.Name, &me.Account.Email, &me.Account.Avatar, &me.Account.GitHub, &accCreated,
			&me.Device.ID, &me.Device.Name, &me.Device.Kind, &me.Device.PublicKey, &devCreated, &seen)
	if errors.Is(err, sql.ErrNoRows) {
		return me, ErrNotFound
	}
	if err != nil {
		return me, err
	}
	me.Account.Created, me.Device.Created, me.Device.LastSeen = parseStamp(accCreated), parseStamp(devCreated), parseStamp(seen)
	me.Device.Current = true
	if t := now(); t.Sub(me.Device.LastSeen) > seenEvery {
		me.Device.LastSeen = t
		s.ExecContext(ctx, `UPDATE devices SET last_seen = $1 WHERE id = $2`, stamp(t), me.Device.ID)
	}
	return me, nil
}

// Devices are account accID's devices, the latest seen first.
func (s *DB) Devices(ctx context.Context, accID string) ([]api.Device, error) {
	rows, err := s.QueryContext(ctx, `SELECT id, name, kind, public_key, created, last_seen FROM devices WHERE account_id = $1 ORDER BY last_seen DESC`, accID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Device{}
	for rows.Next() {
		var d api.Device
		var created, seen string
		if err := rows.Scan(&d.ID, &d.Name, &d.Kind, &d.PublicKey, &created, &seen); err != nil {
			return nil, err
		}
		d.Created, d.LastSeen = parseStamp(created), parseStamp(seen)
		out = append(out, d)
	}
	return out, rows.Err()
}

// RemoveDevice signs device id of account accID out: its tokens go, and
// the team keys sealed to it (ON DELETE CASCADE).
func (s *DB) RemoveDevice(ctx context.Context, accID, id string) error {
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var name string
	err = tx.QueryRowContext(ctx, `SELECT name FROM devices WHERE id = $1 AND account_id = $2`, id, accID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE id = $1`, id); err != nil {
		return fmt.Errorf("store: remove device: %w", err)
	}
	if err := audit(ctx, tx, "", accID, "device.removed", name); err != nil {
		return err
	}
	return tx.Commit()
}

// audit records an action in the audit log, in transaction tx.
func audit(ctx context.Context, tx *sql.Tx, teamID, accID, action, detail string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit (at, team_id, account_id, action, detail) VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5)`,
		stamp(now()), teamID, accID, action, detail)
	return err
}
