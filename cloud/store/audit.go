package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/enowx/enowx/cloud/api"
)

// deviceActions are what devices record in a team's audit log.
var deviceActions = map[string]string{api.AuditRulesChanged: api.RoleMember, api.AuditMemberSession: api.RoleMember,
	api.AuditWatched: api.RoleMember, api.AuditTakeover: api.RoleMember, api.AuditRead: api.RoleMember}

// auditPage is the most entries a page of the log holds.
const auditPage = 100

// Audit is team teamID's log for admin accID, the latest first, before
// entry before (0: from the latest): the team's own entries, and its
// members' devices signing in and out.
func (s *DB) Audit(ctx context.Context, teamID, accID string, before int64) ([]api.AuditEntry, error) {
	out := []api.AuditEntry{}
	err := s.asRole(ctx, teamID, accID, api.RoleAdmin, func(tx *sql.Tx, _ string) error {
		if before <= 0 {
			before = 1 << 62
		}
		rows, err := tx.QueryContext(ctx, `SELECT a.id, a.at, COALESCE(a.account_id, ''), COALESCE(c.name, ''), COALESCE(c.github_login, ''), a.action, a.detail
			FROM audit a LEFT JOIN accounts c ON c.id = a.account_id
			WHERE a.id < $2 AND (a.team_id = $1 OR a.team_id IS NULL AND a.action IN ('device.added', 'device.removed')
				AND a.account_id IN (SELECT account_id FROM members WHERE team_id = $1))
			ORDER BY a.id DESC LIMIT $3`, teamID, before, auditPage)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e api.AuditEntry
			var at string
			if err := rows.Scan(&e.ID, &at, &e.Account.ID, &e.Account.Name, &e.Account.GitHub, &e.Action, &e.Detail); err != nil {
				return err
			}
			e.At = parseStamp(at)
			out = append(out, e)
		}
		return rows.Err()
	})
	return out, err
}

// RecordAudit keeps an action member accID took on a device, when their
// role in team teamID allows the action.
func (s *DB) RecordAudit(ctx context.Context, teamID, accID string, p api.AuditParams) error {
	need, ok := deviceActions[p.Action]
	if !ok {
		return fmt.Errorf("store: devices don't record %q", p.Action)
	}
	p.Detail = cleanName(p.Detail, 500)
	return s.asRole(ctx, teamID, accID, need, func(tx *sql.Tx, _ string) error {
		return audit(ctx, tx, teamID, accID, p.Action, p.Detail)
	})
}
