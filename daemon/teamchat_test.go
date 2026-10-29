package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// A team's chat: a member's message reaches the others' hosts sealed, the
// cloud holding ciphertext; one that looks like it holds a secret isn't
// sent; a viewer reads but doesn't write.
func TestTeamChatGoesSealedThroughTheCloud(t *testing.T) {
	ctx := context.Background()
	db := storetest.Open(t)
	cloud := httptest.NewServer(server.New(server.Config{URL: "http://cloud.test"}, db, slog.New(slog.NewTextHandler(io.Discard, nil))))
	t.Cleanup(cloud.Close)
	ada := signInHost(t, db, cloud.URL, 1, "ada")
	bob := signInHost(t, db, cloud.URL, 2, "bob")
	team, _ := db.CreateTeam(ctx, ada.acc, "Design")
	inv, _ := db.CreateInvite(ctx, team.ID, ada.acc, api.InviteParams{Role: api.RoleViewer})
	db.AcceptInvite(ctx, inv.Token, bob.acc)
	ada.srv.learnTeams(ada.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleOwner}})
	bob.srv.learnTeams(bob.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleViewer}})
	ada.sync(t, team.ID) // the team's key
	came := make(chan []proto.ChatMessage, 4)
	connect(t, bob.srv, func(e proto.Event) {
		var list []proto.ChatMessage
		if e.Name == proto.EventTeamChat && json.Unmarshal(e.Data, &list) == nil {
			came <- list
		}
	})

	if _, err := ada.c.SendChat(proto.ChatSendParams{Team: team.ID, Text: "token ghp_" + string(bytes.Repeat([]byte("c"), 36))}); err == nil {
		t.Fatal("a message holding a secret was sent")
	}
	sent, err := ada.c.SendChat(proto.ChatSendParams{Team: team.ID, Text: "the checkout page is ready to review",
		Mentions: []proto.ChatMention{{Kind: proto.MentionProject, ID: "git:github.com/enowx/app", Title: "app"}}})
	if err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team.ID)
	if err := bob.c.RefreshChat(team.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case list := <-came:
		if len(list) != 1 || list[0].ID != sent.ID || list[0].Name != "ada" || len(list[0].Mentions) != 1 {
			t.Fatalf("what reached bob: %+v", list)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bob wasn't told of the message")
	}
	if list, _ := bob.c.TeamChat(team.ID, 0); len(list) != 1 || list[0].Text != sent.Text {
		t.Fatalf("bob's chat: %+v", list)
	}
	if _, err := bob.c.SendChat(proto.ChatSendParams{Team: team.ID, Text: "hi"}); err == nil {
		t.Fatal("a viewer wrote in the chat")
	}
	rows, _ := db.QueryContext(ctx, `SELECT sealed FROM items WHERE team_id = $1 AND kind = $2`, team.ID, api.ItemChat)
	defer rows.Close()
	n := 0
	for rows.Next() {
		var b []byte
		rows.Scan(&b)
		if bytes.Contains(b, []byte("checkout")) {
			t.Fatal("the cloud holds a message in the clear")
		}
		n++
	}
	if n != 1 {
		t.Fatalf("chat items in the cloud: %d", n)
	}
}
