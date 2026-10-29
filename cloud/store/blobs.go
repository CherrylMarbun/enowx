package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/enowx/enowx/cloud/api"
)

// A team's files (api.PathTeamBlob): their content is in the server's file
// store, sealed; here who sent each and its size, so a team keeps no more
// than maxTeamBlobs and its sender or an admin deletes one.

// maxTeamBlobs is the most a team keeps in files.
const maxTeamBlobs = 2 << 30

// AddBlob records file id of team teamID, size bytes, sent by accID, a
// member or more; one already there is refused.
func (s *DB) AddBlob(ctx context.Context, teamID, accID, id string, size int64) error {
	return s.asRole(ctx, teamID, accID, api.RoleMember, func(tx *sql.Tx, _ string) error {
		// The team's row first, so two adding at once count each other.
		if _, err := tx.ExecContext(ctx, `UPDATE teams SET epoch = epoch WHERE id = $1`, teamID); err != nil {
			return err
		}
		var used int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(size), 0) FROM blobs WHERE team_id = $1`, teamID).Scan(&used); err != nil {
			return err
		}
		if used+size > maxTeamBlobs {
			return fmt.Errorf("store: the team keeps %d GB of files at most: delete some messages' images", maxTeamBlobs>>30)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO blobs (team_id, id, size, created_by, created) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
			teamID, id, size, accID, stamp(now()))
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errors.New("store: that file is there already")
		}
		return nil
	})
}

// Blob says file id of team teamID is there, for accID, a member.
func (s *DB) Blob(ctx context.Context, teamID, accID, id string) error {
	if _, err := role(ctx, s, teamID, accID); err != nil {
		return err
	}
	var n int
	if err := s.QueryRowContext(ctx, `SELECT COUNT(*) FROM blobs WHERE team_id = $1 AND id = $2`, teamID, id).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteBlob forgets file id of team teamID, as accID: its sender, or an
// admin.
func (s *DB) DeleteBlob(ctx context.Context, teamID, accID, id string) error {
	return s.asRole(ctx, teamID, accID, api.RoleViewer, func(tx *sql.Tx, r string) error {
		var by string
		err := tx.QueryRowContext(ctx, `SELECT created_by FROM blobs WHERE team_id = $1 AND id = $2`, teamID, id).Scan(&by)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if by != accID && rank[r] < rank[api.RoleAdmin] {
			return ErrForbidden
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM blobs WHERE team_id = $1 AND id = $2`, teamID, id)
		return err
	})
}

// TeamBlobs are team teamID's files, for its owner deleting the team.
func (s *DB) TeamBlobs(ctx context.Context, teamID string) ([]string, error) {
	rows, err := s.QueryContext(ctx, `SELECT id FROM blobs WHERE team_id = $1`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
