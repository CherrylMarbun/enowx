package api

import "time"

// A team's audit log (docs/cloud.md): what its members did. The cloud
// records its own (sign-ins, invites, roles, keys); devices record what
// only they see, as a team's project's rules changing.
// PathAudit: POST records an action (members), GET lists the log, the
// latest first, ?before= an entry's ID for the page after (admins).
const PathAudit = "/v1/teams/{team}/audit"

// Actions devices record: a team's project's rules changed; on a shared
// host, a member's session started, a session watched, one taken over.
const (
	AuditRulesChanged  = "rules.changed"
	AuditMemberSession = "session.started"
	AuditWatched       = "session.watched"
	AuditTakeover      = "session.taken_over"
	AuditRead          = "session.read" // a member's agent read its conversation
)

// AuditEntry is an action of the team's log: who, what, when. Account is
// empty for the cloud's own.
type AuditEntry struct {
	ID      int64     `json:"id"`
	At      time.Time `json:"at"`
	Account Account   `json:"account"`
	Action  string    `json:"action"`
	Detail  string    `json:"detail,omitempty"`
}

// AuditParams records an action of the asking member: one of the actions
// devices record, and what it was about.
type AuditParams struct {
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
}
