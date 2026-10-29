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

// Notes and canvases go from one space to another: moved, or copied
// (Copy), the user's own to a team's after a scan for secrets, a team's
// out only when its admins let members copy its items out.
const (
	MethodNoteMoveSpace   = "note.moveSpace"   // SpaceMoveParams → NoteInfo (in the space it went to)
	MethodCanvasMoveSpace = "canvas.moveSpace" // SpaceMoveParams → CanvasInfo
)

// SpaceMoveParams takes note or canvas ID, with the pages under a note,
// to Team's space ("" the user's own); Copy leaves it where it was too.
type SpaceMoveParams struct {
	ID   string `json:"id"`
	Team string `json:"team,omitempty"`
	Copy bool   `json:"copy,omitempty"`
}
