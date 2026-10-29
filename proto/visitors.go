package proto

import "time"

// Visitors (docs/cloud.md, Hosts and sessions): a member of one of the
// host's teams connecting from their own device, through the host's team
// listener. Before anything else the connection hands the host a ticket
// the cloud gave the member for this host (team.auth); the host redeems it
// and serves the visitor as their role in the team allows.
const (
	MethodTeamAuth = "team.auth" // TeamAuthParams → Visitor
	// MethodHostVisitors lists who is connected as a visitor; the host's
	// own connections only.
	MethodHostVisitors = "host.visitors" // → []Visitor
)

// EventHostVisitors carries the full []Visitor after one came or went.
const EventHostVisitors = "host.visitors"

// TeamAuthParams hands the host a ticket ("enxt_…").
type TeamAuthParams struct {
	Ticket string `json:"ticket"`
}

// Visitor is a member of a team connected to the host.
type Visitor struct {
	Account string    `json:"account"` // their account's ID
	Name    string    `json:"name"`
	GitHub  string    `json:"github,omitempty"`
	Team    string    `json:"team"`
	Role    string    `json:"role"`
	Device  string    `json:"device"` // the device they connect from
	Since   time.Time `json:"since"`
	// Watching are the sessions they have open; Controls those they may
	// type into, the host's user having let them take over.
	Watching []string `json:"watching,omitempty"`
	Controls []string `json:"controls,omitempty"`
}

// Taking over a session (docs/cloud.md, Hosts and sessions): a member
// watching it read-only asks; the host's user lets them type into it, or
// not, and takes it back when they will.
const (
	MethodTakeoverAsk = "session.takeover.ask" // SessionRef: a visitor asks to take over a session
	// MethodTakeoverAnswer lets a visitor take over, or not: the host's
	// own connections.
	MethodTakeoverAnswer = "session.takeover.answer" // TakeoverAnswer
	// MethodTakeoverEnd takes a session back (the host's user) or gives it
	// back (the visitor).
	MethodTakeoverEnd = "session.takeover.end" // TakeoverAnswer (Session, and Account for the host's user)
)

// EventTakeoverAsked carries a TakeoverAsk to the host's own connections;
// EventTakeoverAnswered a TakeoverAnswer to the visitor who asked, and to
// the host's own as one ends.
const (
	EventTakeoverAsked    = "session.takeover.asked"
	EventTakeoverAnswered = "session.takeover.answered"
)

// TakeoverAsk is a visitor asking to take over a session.
type TakeoverAsk struct {
	Session string  `json:"session"`
	Name    string  `json:"name,omitempty"` // the session's
	Visitor Visitor `json:"visitor"`
}

// TakeoverAnswer lets visitor Account take over Session, or not (Allow).
type TakeoverAnswer struct {
	Session string `json:"session"`
	Account string `json:"account,omitempty"`
	Allow   bool   `json:"allow"`
}

// SessionMember is the member of a team who started a session on another
// member's host (SessionInfo.Member): it runs with their agent account and
// git identity, none of the host's credentials.
type SessionMember struct {
	Account string `json:"account"`
	Name    string `json:"name"`
	GitHub  string `json:"github,omitempty"`
	Team    string `json:"team"`
}
