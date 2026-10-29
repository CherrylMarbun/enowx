package proto

import "time"

// Spaces (docs/cloud.md, Personal and team): the user's own notes and
// canvases, and each team's beside them. A note, canvas or group says
// which it is in with its Team, "" for the user's own; the lists hold
// every space's.
const (
	MethodSpaces = "space.list" // → []Space, the teams' (the user's own is always there)
	// MethodSpaceSync syncs a team's space with the cloud now, and says
	// how it went.
	MethodSpaceSync = "space.sync" // SpaceRef → Space
)

// SpaceRef names a team's space.
type SpaceRef struct {
	Team string `json:"team"`
}

// EventSpacesChanged carries the full []Space after a team's came or
// went, or was renamed.
const EventSpacesChanged = "spaces.changed"

// Space is a team's space on a host: its notes and canvases there, kept
// while the host is signed in as a device of a member.
type Space struct {
	Team string `json:"team"`
	Name string `json:"name"`
	// Role is the account's in the team.
	Role string `json:"role,omitempty"`
	// Synced is when its sync with the cloud last ended well; SyncErr why
	// the last didn't, or what it left out (a file holding a secret).
	Synced  time.Time `json:"synced,omitzero"`
	SyncErr string    `json:"syncErr,omitempty"`
}
