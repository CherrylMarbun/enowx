package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enowx/enowx/cloud/api"
)

// SetRecovery makes publicKey account accID's recovery key, in place of
// the one before (and the team keys sealed to it).
func (s *DB) SetRecovery(ctx context.Context, accID string, publicKey []byte) (api.Device, error) {
	if len(publicKey) != 32 {
		return api.Device{}, errors.New("store: a recovery key's public half is 32 bytes (X25519)")
	}
	d := api.Device{ID: newID(), Name: "Recovery key", Kind: api.DeviceApp, PublicKey: publicKey, Created: now(), Recovery: true}
	d.LastSeen = d.Created
	tx, err := s.BeginTx(ctx, nil)
	if err != nil {
		return api.Device{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE account_id = $1 AND recovery = 1`, accID); err != nil {
		return api.Device{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO devices (id, account_id, name, kind, public_key, created, last_seen, recovery) VALUES ($1, $2, $3, $4, $5, $6, $7, 1)`,
		d.ID, accID, d.Name, d.Kind, d.PublicKey, stamp(d.Created), stamp(d.LastSeen)); err != nil {
		return api.Device{}, fmt.Errorf("store: recovery key: %w", err)
	}
	if err := audit(ctx, tx, "", accID, "recovery.set", ""); err != nil {
		return api.Device{}, err
	}
	return d, tx.Commit()
}

// recoveryDevice is account accID's recovery key, "" for none.
func recoveryDevice(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, accID string) (string, error) {
	var id string
	err := q.QueryRowContext(ctx, `SELECT id FROM devices WHERE account_id = $1 AND recovery = 1`, accID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// RecoveryKeys are team teamID's keys sealed to member accID's recovery
// key.
func (s *DB) RecoveryKeys(ctx context.Context, teamID, accID string) (api.TeamKeys, error) {
	if _, err := role(ctx, s, teamID, accID); err != nil {
		return api.TeamKeys{}, err // a team is invisible to others
	}
	dev, err := recoveryDevice(ctx, s, accID)
	if err != nil {
		return api.TeamKeys{}, err
	}
	if dev == "" {
		return api.TeamKeys{}, fmt.Errorf("store: no recovery key is set for this account")
	}
	return s.TeamKeys(ctx, teamID, accID, dev)
}

// Recover keeps team teamID's key of epoch, sealed to device deviceID of
// member accID, when their recovery key holds that epoch's: the device
// opened it with the recovery key. A copy the device has already stays.
func (s *DB) Recover(ctx context.Context, teamID, accID, deviceID string, p api.RecoverParams) error {
	if len(p.Sealed) != api.SealedKeySize {
		return fmt.Errorf("store: a sealed key is %d bytes", api.SealedKeySize)
	}
	return s.asRole(ctx, teamID, accID, api.RoleViewer, func(tx *sql.Tx, _ string) error {
		dev, err := recoveryDevice(ctx, tx, accID)
		if err != nil {
			return err
		}
		var holds int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_keys WHERE team_id = $1 AND epoch = $2 AND device_id = $3`, teamID, p.Epoch, dev).Scan(&holds); err != nil {
			return err
		}
		if dev == "" || holds == 0 {
			return ErrForbidden // the recovery key never had that epoch's
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO team_keys (team_id, epoch, device_id, sealed) VALUES ($1, $2, $3, $4) ON CONFLICT (team_id, epoch, device_id) DO NOTHING`,
			teamID, p.Epoch, deviceID, p.Sealed); err != nil {
			return err
		}
		return audit(ctx, tx, teamID, accID, "key.recovered", fmt.Sprintf("epoch %d", p.Epoch))
	})
}
