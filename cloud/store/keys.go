package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/enowx/enowx/cloud/api"
)

// TeamKeys are the sealed copies of team teamID's keys device deviceID of
// member accID holds.
func (s *DB) TeamKeys(ctx context.Context, teamID, accID, deviceID string) (api.TeamKeys, error) {
	if _, err := role(ctx, s, teamID, accID); err != nil {
		return api.TeamKeys{}, err
	}
	out := api.TeamKeys{Keys: []api.SealedKey{}}
	if err := s.QueryRowContext(ctx, `SELECT epoch FROM teams WHERE id = $1`, teamID).Scan(&out.Epoch); err != nil {
		return out, err
	}
	rows, err := s.QueryContext(ctx, `SELECT epoch, sealed FROM team_keys WHERE team_id = $1 AND device_id = $2 ORDER BY epoch`, teamID, deviceID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var k api.SealedKey
		if err := rows.Scan(&k.Epoch, &k.Sealed); err != nil {
			return out, err
		}
		out.Keys = append(out.Keys, k)
	}
	return out, rows.Err()
}

// TeamDevices are the devices of team teamID's members, with the epochs
// each holds the key of, for member accID.
func (s *DB) TeamDevices(ctx context.Context, teamID, accID string) ([]api.TeamDevice, error) {
	if _, err := role(ctx, s, teamID, accID); err != nil {
		return nil, err
	}
	rows, err := s.QueryContext(ctx, `SELECT a.id, a.name, d.id, d.name, d.kind, d.public_key, d.created, d.last_seen
		FROM members m JOIN accounts a ON a.id = m.account_id JOIN devices d ON d.account_id = a.id
		WHERE m.team_id = $1 ORDER BY a.name, d.created`, teamID)
	if err != nil {
		return nil, err
	}
	var out []api.TeamDevice
	index := map[string]int{}
	for rows.Next() {
		var td api.TeamDevice
		var created, seen string
		if err := rows.Scan(&td.Account, &td.Name, &td.Device.ID, &td.Device.Name, &td.Device.Kind, &td.Device.PublicKey, &created, &seen); err != nil {
			rows.Close()
			return nil, err
		}
		td.Device.Created, td.Device.LastSeen, td.Epochs = parseStamp(created), parseStamp(seen), []int{}
		index[td.Device.ID] = len(out)
		out = append(out, td)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	keys, err := s.QueryContext(ctx, `SELECT device_id, epoch FROM team_keys WHERE team_id = $1 ORDER BY epoch`, teamID)
	if err != nil {
		return nil, err
	}
	defer keys.Close()
	for keys.Next() {
		var dev string
		var epoch int
		if err := keys.Scan(&dev, &epoch); err != nil {
			return nil, err
		}
		if i, ok := index[dev]; ok {
			out[i].Epochs = append(out[i].Epochs, epoch)
		}
	}
	if out == nil {
		out = []api.TeamDevice{}
	}
	return out, keys.Err()
}

// ShareTeamKey keeps copies of team teamID's key of epoch, sealed to its
// members' devices, from device deviceID of member accID. The device must
// hold that key itself; an epoch nobody holds yet, the team's current one,
// is started by an admin. A device's copy, once kept, stays: a member can't
// spoil another's.
func (s *DB) ShareTeamKey(ctx context.Context, teamID, accID, deviceID string, epoch int, keys []api.DeviceKey) error {
	if len(keys) == 0 {
		return errors.New("store: no keys to share")
	}
	return s.asRole(ctx, teamID, accID, api.RoleViewer, func(tx *sql.Tx, r string) error {
		var current int
		if err := tx.QueryRowContext(ctx, `SELECT epoch FROM teams WHERE id = $1`, teamID).Scan(&current); err != nil {
			return err
		}
		if epoch < 1 || epoch > current {
			return fmt.Errorf("store: the team has no epoch %d", epoch)
		}
		var held, holds int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN device_id = $3 THEN 1 ELSE 0 END), 0) FROM team_keys WHERE team_id = $1 AND epoch = $2`,
			teamID, epoch, deviceID).Scan(&held, &holds); err != nil {
			return err
		}
		switch {
		case held == 0 && (epoch != current || rank[r] < rank[api.RoleAdmin]):
			return ErrForbidden // a new epoch's key comes from an admin
		case held > 0 && holds == 0:
			return ErrForbidden // shared only by a device that has it
		}
		members, err := memberDevices(ctx, tx, teamID)
		if err != nil {
			return err
		}
		var to []string
		for _, k := range keys {
			if !members[k.Device] {
				return fmt.Errorf("store: device %s isn't a member's", k.Device)
			}
			if len(k.Sealed) != api.SealedKeySize {
				return fmt.Errorf("store: a sealed key is %d bytes", api.SealedKeySize)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO team_keys (team_id, epoch, device_id, sealed) VALUES ($1, $2, $3, $4) ON CONFLICT (team_id, epoch, device_id) DO NOTHING`,
				teamID, epoch, k.Device, k.Sealed); err != nil {
				return err
			}
			to = append(to, k.Device)
		}
		if held == 0 && !slices.Contains(to, deviceID) {
			return errors.New("store: starting an epoch's key, keep a copy for this device too")
		}
		return audit(ctx, tx, teamID, accID, "key.shared", fmt.Sprintf("epoch %d to %d devices", epoch, len(to)))
	})
}

// memberDevices are the devices of team teamID's members.
func memberDevices(ctx context.Context, tx *sql.Tx, teamID string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT d.id FROM members m JOIN devices d ON d.account_id = m.account_id WHERE m.team_id = $1`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
