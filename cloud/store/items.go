package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"github.com/enowx/enowx/cloud/api"
)

// ConflictError is a write of an item whose version was taken already:
// Item is the item as it is.
type ConflictError struct{ Item api.Item }

func (e *ConflictError) Error() string {
	return fmt.Sprintf("store: item %s is at version %d already", e.Item.ID, e.Item.Version)
}

// A page holds 200 items at most, and stops once past itemPageBytes of
// them (one item always fits).
const (
	itemPage      = 200
	itemPageBytes = 8 << 20
)

// itemIDPattern is an item's ID, chosen by the devices: opaque to the
// cloud.
var itemIDPattern = regexp.MustCompile(`^[0-9a-f]{16,64}$`)

var itemKinds = map[string]bool{api.ItemNote: true, api.ItemCanvas: true, api.ItemFile: true}

// Items are team teamID's items changed after seq, for member accID.
func (s *DB) Items(ctx context.Context, teamID, accID string, after int64) (api.Items, error) {
	if _, err := role(ctx, s, teamID, accID); err != nil {
		return api.Items{}, err
	}
	rows, err := s.QueryContext(ctx, `SELECT id, kind, version, epoch, sealed, deleted, seq, updated, updated_by FROM items
		WHERE team_id = $1 AND seq > $2 ORDER BY seq LIMIT $3`, teamID, after, itemPage+1)
	if err != nil {
		return api.Items{}, err
	}
	defer rows.Close()
	out := api.Items{Items: []api.Item{}, Seq: after}
	size := 0
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return api.Items{}, err
		}
		if len(out.Items) == itemPage || len(out.Items) > 0 && size+len(it.Sealed) > itemPageBytes {
			out.More = true
			break
		}
		size += len(it.Sealed)
		out.Items = append(out.Items, it)
		out.Seq = it.Seq
	}
	if err := rows.Err(); err != nil {
		return api.Items{}, err
	}
	if !out.More {
		// Nothing after: the team's latest, so the next ask starts there.
		if err := s.QueryRowContext(ctx, `SELECT item_seq FROM teams WHERE id = $1`, teamID).Scan(&out.Seq); err != nil {
			return api.Items{}, err
		}
	}
	return out, nil
}

func scanItem(r interface{ Scan(...any) error }) (api.Item, error) {
	var it api.Item
	var deleted int
	var updated string
	if err := r.Scan(&it.ID, &it.Kind, &it.Version, &it.Epoch, &it.Sealed, &deleted, &it.Seq, &updated, &it.UpdatedBy); err != nil {
		return api.Item{}, err
	}
	it.Deleted, it.Updated = deleted != 0, parseStamp(updated)
	return it, nil
}

// PutItem writes item id of team teamID from device deviceID of member
// accID (a member or above; viewers read only): its version p.Version,
// one after the one kept, sealed with the team's current key.
func (s *DB) PutItem(ctx context.Context, teamID, accID, deviceID, id string, p api.PutItemParams) (api.Item, error) {
	switch {
	case !itemIDPattern.MatchString(id):
		return api.Item{}, fmt.Errorf("store: no item %q", id)
	case !itemKinds[p.Kind]:
		return api.Item{}, fmt.Errorf("store: no kind of item %q", p.Kind)
	case p.Version < 1:
		return api.Item{}, errors.New("store: an item's version starts at 1")
	case !p.Deleted && len(p.Sealed) == 0:
		return api.Item{}, errors.New("store: an item needs its content")
	case len(p.Sealed) > api.MaxItemSize:
		return api.Item{}, fmt.Errorf("store: an item is %d MB at most", api.MaxItemSize>>20)
	}
	if p.Deleted {
		p.Sealed = nil
	}
	var out api.Item
	err := s.asRole(ctx, teamID, accID, api.RoleMember, func(tx *sql.Tx, _ string) error {
		var epoch int
		if err := tx.QueryRowContext(ctx, `SELECT epoch FROM teams WHERE id = $1`, teamID).Scan(&epoch); err != nil {
			return err
		}
		if p.Epoch != epoch {
			return fmt.Errorf("store: seal it with the team's key of epoch %d, not %d", epoch, p.Epoch)
		}
		cur, err := scanItem(tx.QueryRowContext(ctx, `SELECT id, kind, version, epoch, sealed, deleted, seq, updated, updated_by FROM items WHERE team_id = $1 AND id = $2`, teamID, id))
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if p.Version != 1 {
				return &ConflictError{Item: api.Item{ID: id}}
			}
		case err != nil:
			return err
		case cur.Version != p.Version-1:
			return &ConflictError{Item: cur}
		}
		var seq int64
		if err := tx.QueryRowContext(ctx, `UPDATE teams SET item_seq = item_seq + 1 WHERE id = $1 RETURNING item_seq`, teamID).Scan(&seq); err != nil {
			return err
		}
		out = api.Item{ID: id, Kind: p.Kind, Version: p.Version, Epoch: p.Epoch, Sealed: p.Sealed, Deleted: p.Deleted, Seq: seq, Updated: now(), UpdatedBy: deviceID}
		deleted := 0
		if p.Deleted {
			deleted = 1
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO items (team_id, id, kind, version, epoch, sealed, deleted, updated, updated_by, seq) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (team_id, id) DO UPDATE SET kind = $3, version = $4, epoch = $5, sealed = $6, deleted = $7, updated = $8, updated_by = $9, seq = $10`,
			teamID, id, p.Kind, p.Version, p.Epoch, p.Sealed, deleted, stamp(out.Updated), deviceID, seq)
		return err
	})
	return out, err
}
