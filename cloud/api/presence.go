package api

// Who of a team is here now, and who is typing in its chat: told by each
// member's app as it runs, kept by the cloud a minute at most, in memory
// only. Nothing of what is typed is sent.
const (
	// PathTeamPresence takes the asking account's presence (POST
	// PresenceParams) and answers the team's (Presence).
	PathTeamPresence = "/v1/teams/{team}/presence"
)

// PresenceParams is the asking account's presence in a team: here, and
// typing in its chat or not.
type PresenceParams struct {
	Typing bool `json:"typing,omitempty"`
}

// Presence is a team's members here now, and those typing in its chat,
// by account ID.
type Presence struct {
	Online []string `json:"online"`
	Typing []string `json:"typing"`
}
