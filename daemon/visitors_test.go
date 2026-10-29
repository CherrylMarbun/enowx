package daemon

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A member of the host's team connects through the team listener: nothing
// is served before their ticket, a ticket works once, and once in the host
// knows who they are and their role, its own connections see them, and no
// event reaches them unasked.
func TestVisitorsSignInWithATicket(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	bob := signInHost(t, db, cloud.URL, 2, "bob")
	team, _ := db.CreateTeam(ctx, ada.acc, "Design")
	inv, _ := db.CreateInvite(ctx, team.ID, ada.acc, api.InviteParams{Role: api.RoleMember})
	db.AcceptInvite(ctx, inv.Token, bob.acc)
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	adaHost, _ := ada.srv.cloud.store.Get()
	bobDev, _ := bob.srv.cloud.store.Get()
	ticket := func() string {
		tk, err := db.IssueTicket(ctx, team.ID, bob.acc, bobDev.DeviceID, adaHost.DeviceID)
		if err != nil {
			t.Fatal(err)
		}
		return tk.Ticket
	}
	var leaked atomic.Int32
	visit := func() *client.Client {
		a, b := net.Pipe()
		go ada.srv.ServeVisitorConn(a)
		c, err := client.New(b, func(proto.Event) { leaked.Add(1) })
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}

	v := visit()
	if _, err := v.Notes(); err == nil {
		t.Fatal("a visitor was served before their ticket")
	}
	if _, err := v.TeamAuth("enxt_forged"); err == nil {
		t.Fatal("a forged ticket let a visitor in")
	}
	used := ticket()
	v = visit()
	got, err := v.TeamAuth(used)
	if err != nil || got.GitHub != "bob" || got.Role != api.RoleMember || got.Team != team.ID {
		t.Fatalf("signed in: %+v %v", got, err)
	}
	if _, err := v.Notes(); err == nil {
		t.Fatal("a visitor read the host's notes, nothing shared")
	}
	if _, err := v.HostVisitors(); err == nil {
		t.Fatal("a visitor listed the host's visitors")
	}
	if list, err := ada.c.HostVisitors(); err != nil || len(list) != 1 || list[0].GitHub != "bob" {
		t.Fatalf("the host's own see who visits: %+v %v", list, err)
	}
	if _, err := visit().TeamAuth(used); err == nil {
		t.Fatal("a ticket worked twice")
	}
	ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Private"}})
	time.Sleep(200 * time.Millisecond)
	if n := leaked.Load(); n != 0 {
		t.Fatalf("%d events reached a visitor", n)
	}
	v.Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if list, _ := ada.c.HostVisitors(); len(list) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a visitor gone stays listed")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
