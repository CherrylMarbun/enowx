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
}
