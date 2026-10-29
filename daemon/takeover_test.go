package daemon

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	cloudclient "github.com/enowx/enowx/cloud/client"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A member watches a shared session live: what they type reaches it only
// once the host's user lets them take over, which the host's user sees
// asked, and takes back; the host's own see who watches and who controls.
func TestVisitorWatchesThenTakesOver(t *testing.T) {
	t.Setenv(proto.EnvTeamListen, "127.0.0.1:0")
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

	project := gitRepo(t)
	ada.c.AddProject(project)
	typed := filepath.Join(project, "typed.txt")
	sess, err := ada.c.CreateSession(proto.SessionCreateParams{Name: "pair", Dir: project, Cols: 80, Rows: 10, Command: []string{"sh", "-c", "stty -echo; cat > typed.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	sh, err := ada.c.SetHostShare(team.ID, []string{project})
	if err != nil {
		t.Fatal(err)
	}
	asked := make(chan proto.TakeoverAsk, 1)
	own := connect(t, ada.srv, func(e proto.Event) {
		var a proto.TakeoverAsk
		if e.Name == proto.EventTakeoverAsked && json.Unmarshal(e.Data, &a) == nil {
			asked <- a
		}
	})

	adaDev, _ := ada.srv.cloud.store.Get()
	bobDev, _ := bob.srv.cloud.store.Get()
	bc, _ := cloudclient.New(cloud.URL, bobDev.Token)
	tk, err := bc.HostTicket(ctx, team.ID, adaDev.DeviceID)
	if err != nil {
		t.Fatal(err)
	}
	nc, err := net.Dial("tcp", sh.Addr)
	if err != nil {
		t.Fatal(err)
	}
	answered := make(chan proto.TakeoverAnswer, 2)
	v, err := client.New(nc, func(e proto.Event) {
		var a proto.TakeoverAnswer
		if e.Name == proto.EventTakeoverAnswered && json.Unmarshal(e.Data, &a) == nil {
			answered <- a
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	me, err := v.TeamAuth(tk.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	st, err := v.Attach(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	st.Write([]byte("watching\n"))
	time.Sleep(300 * time.Millisecond)
	if data, _ := os.ReadFile(typed); strings.Contains(string(data), "watching") {
		t.Fatal("a watcher's typing reached the session")
	}
	if list, _ := own.HostVisitors(); len(list) != 1 || len(list[0].Watching) != 1 || list[0].Watching[0] != sess.ID {
		t.Fatalf("who watches what: %+v", list)
	}

	if err := v.AskTakeover(sess.ID); err != nil {
		t.Fatal(err)
	}
	var a proto.TakeoverAsk
	select {
	case a = <-asked:
	case <-time.After(5 * time.Second):
		t.Fatal("the host's user wasn't asked")
	}
	if a.Visitor.GitHub != "bob" || a.Session != sess.ID {
		t.Fatalf("asked: %+v", a)
	}
	if err := own.AnswerTakeover(sess.ID, me.Account, true); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-answered:
		if !got.Allow {
			t.Fatalf("answered: %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the visitor wasn't answered")
	}
	st.Write([]byte("typing\n"))
	deadline := time.Now().Add(5 * time.Second)
	for {
		if data, _ := os.ReadFile(typed); strings.Contains(string(data), "typing") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the visitor in control typed nothing in")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if list, _ := own.HostVisitors(); len(list) != 1 || len(list[0].Controls) != 1 {
		t.Fatalf("who controls: %+v", list)
	}

	if err := own.EndTakeover(sess.ID, me.Account); err != nil {
		t.Fatal(err)
	}
	st.Write([]byte("after\n"))
	time.Sleep(300 * time.Millisecond)
	if data, _ := os.ReadFile(typed); strings.Contains(string(data), "after") {
		t.Fatal("taken back, the visitor still typed in")
	}
	// The team's log has it: bob entered, watched, took over.
	waitFor(t, "the team's log", func() bool {
		log, err := db.Audit(ctx, team.ID, ada.acc, 0)
		got := map[string]bool{}
		for _, e := range log {
			got[e.Action] = true
		}
		return err == nil && got["host.entered"] && got[api.AuditWatched] && got[api.AuditTakeover]
	})
}
