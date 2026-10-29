package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/enowx/enowx/cloud/api"
)

// deviceActions are what devices record in a team's audit log.
var deviceActions = map[string]string{api.AuditRulesChanged: api.RoleMember}

// RecordAudit keeps an action member accID took on a device, when their
// role in team teamID allows the action.
func (s *DB) RecordAudit(ctx context.Context, teamID, accID string, p api.AuditParams) error {
	need, ok := deviceActions[p.Action]
	if !ok {
		return fmt.Errorf("store: devices don't record %q", p.Action)
	}
	if len(p.Detail) > 500 {
		p.Detail = p.Detail[:500]
	}
	return s.asRole(ctx, teamID, accID, need, func(tx *sql.Tx, _ string) error {
		return audit(ctx, tx, teamID, accID, p.Action, p.Detail)
	})
}
