package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/enowx/enowx/cloud/api"
)

// A member's ticket into a host of their team works once, for that host
// only, and says who they are and their role; nobody outside the team
// gets one.
func TestHostTicketsLetMembersIn(t *testing.T) {
	ctx := context.Background()
	srv, db := startWithDB(t, Config{URL: "https://cloud.example.com"})
	device := func(gh int64, login string) (string, string, string) {
		acc, _ := db.GitHubAccount(ctx, gh, login, login, "", "")
		d, tok, _ := db.AddDevice(ctx, acc.ID, login+"'s", api.DeviceHost, make([]byte, 32))
		return acc.ID, d.ID, tok
	}
	_, adaHost, adaTok := device(1, "ada")
	_, _, bobTok := device(2, "bob")
	_, eveHost, eveTok := device(3, "eve")
	name := "Design"
	var team api.Team
	send(t, "POST", srv.URL+api.PathTeams, adaTok, api.TeamParams{Name: &name}, &team)
	var inv api.Invite
	send(t, "POST", srv.URL+"/v1/teams/"+team.ID+"/invites", adaTok, api.InviteParams{Role: api.RoleViewer}, &inv)
	send(t, "POST", srv.URL+api.PathJoin, bobTok, api.JoinParams{Token: inv.Token}, nil)
	tickets := srv.URL + "/v1/teams/" + team.ID + "/tickets"

	var tk api.HostTicket
	if code, msg := send(t, "POST", tickets, bobTok, api.HostTicketParams{Host: adaHost}, &tk); code != http.StatusOK || len(tk.Ticket) < 20 {
		t.Fatalf("a member's ticket: %d %s", code, msg)
	}
	if code, _ := send(t, "POST", tickets, bobTok, api.HostTicketParams{Host: eveHost}, nil); code != http.StatusNotFound {
		t.Fatalf("a ticket into a host no member's: %d", code)
	}
	if code, _ := send(t, "POST", tickets, eveTok, api.HostTicketParams{Host: adaHost}, nil); code != http.StatusNotFound {
		t.Fatalf("one not a member got a ticket: %d", code)
	}
	if code, _ := send(t, "POST", srv.URL+api.PathTicketRedeem, eveTok, api.RedeemParams{Ticket: tk.Ticket}, nil); code != http.StatusForbidden {
		t.Fatalf("another host redeemed it: %d", code)
	}
	var v api.Visitor
	if code, _ := send(t, "POST", srv.URL+api.PathTicketRedeem, adaTok, api.RedeemParams{Ticket: tk.Ticket}, &v); code != http.StatusOK ||
		v.Account.GitHub != "bob" || v.Role != api.RoleViewer || v.Team != team.ID {
		t.Fatalf("redeemed: %d %+v", code, v)
	}
	if code, _ := send(t, "POST", srv.URL+api.PathTicketRedeem, adaTok, api.RedeemParams{Ticket: tk.Ticket}, nil); code != http.StatusForbidden {
		t.Fatalf("a ticket worked twice: %d", code)
	}
}
