package api

import "time"

// A member's way into a host of their team (docs/cloud.md, Hosts and
// sessions): one of their devices asks the cloud for a ticket for that
// host, and hands it to the host when it connects; the host redeems it
// with its own token and learns who connects, and their role. A ticket
// works once, for that host only, within minutes; no token leaves the
// device that holds it.
const (
	PathHostTickets  = "/v1/teams/{team}/tickets" // POST HostTicketParams → HostTicket
	PathTicketRedeem = "/v1/tickets/redeem"       // POST RedeemParams → Visitor, by the ticket's host
)

// TicketLife is how long a ticket works.
const TicketLife = 2 * time.Minute

// HostTicketParams asks a ticket into host device Host.
type HostTicketParams struct {
	Host string `json:"host"`
}

// HostTicket is a ticket ("enxt_…"), shown once.
type HostTicket struct {
	Ticket  string    `json:"ticket"`
	Expires time.Time `json:"expires"`
}

// RedeemParams redeems a ticket handed to the host.
type RedeemParams struct {
	Ticket string `json:"ticket"`
}

// Visitor is who a redeemed ticket let in: a member of Team with Role,
// from their device Device.
type Visitor struct {
	Account Account `json:"account"`
	Team    string  `json:"team"`
	Role    string  `json:"role"`
	Device  string  `json:"device"`
}
