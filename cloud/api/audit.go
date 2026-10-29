package api

// A team's audit log (docs/cloud.md): what its members did. The cloud
// records its own (sign-ins, invites, roles, keys); devices record what
// only they see, as a team's project's rules changing.
const PathAudit = "/v1/teams/{team}/audit"

// Actions devices record.
const AuditRulesChanged = "rules.changed"

// AuditParams records an action of the asking member: one of the actions
// devices record, and what it was about.
type AuditParams struct {
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
}
