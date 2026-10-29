package daemon

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/server"
	"github.com/enowx/enowx/cloud/store/storetest"
	"github.com/enowx/enowx/proto"
)

// Members answer a message and react to it; each takes back their own
// reaction only, one written as another member isn't taken, and a
// message deleted takes its reactions along on every host.
func TestTeamChatRepliesAndReactions(t *testing.T) {
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
	bob.srv.learnTeams(bob.acc, []api.Team{{ID: team.ID, Name: "Design", Role: api.RoleMember}})
	both := func() {
		ada.sync(t, team.ID)
		bob.sync(t, team.ID)
		ada.sync(t, team.ID)
	}
	both()
	bob.sync(t, team.ID)

	asked, err := ada.c.SendChat(proto.ChatSendParams{Team: team.ID, Text: "ship it today?"})
	if err != nil {
		t.Fatal(err)
	}
	both()
	answer, err := bob.c.SendChat(proto.ChatSendParams{Team: team.ID, Text: "yes", ReplyTo: asked.ID})
	if err != nil || answer.ReplyTo != asked.ID {
		t.Fatalf("answer: %+v %v", answer, err)
	}
	if _, err := bob.c.SendChat(proto.ChatSendParams{Team: team.ID, Text: "x", ReplyTo: "../team"}); err == nil {
		t.Fatal("an answer to a message that can't be")
	}
	for _, e := range []string{"hello", "", "👍 👍", "1"} {
		if _, err := bob.c.ReactChat(proto.ChatReactParams{Team: team.ID, ID: asked.ID, Emoji: e}); err == nil {
			t.Errorf("a reaction %q was taken", e)
		}
	}
	if _, err := bob.c.ReactChat(proto.ChatReactParams{Team: team.ID, ID: asked.ID, Emoji: "👍"}); err != nil {
		t.Fatal(err)
	}
	ada.c.ReactChat(proto.ChatReactParams{Team: team.ID, ID: asked.ID, Emoji: "👍"})
	ada.c.ReactChat(proto.ChatReactParams{Team: team.ID, ID: asked.ID, Emoji: "🎉"})
	both()
	bob.sync(t, team.ID)
	want := []proto.ChatReaction{{Emoji: "👍", Accounts: sorted(ada.acc, bob.acc)}, {Emoji: "🎉", Accounts: []string{ada.acc}}}
	for _, h := range []struct {
		name string
		c    interface {
			TeamChat(string, int) ([]proto.ChatMessage, error)
		}
	}{{"ada", ada.c}, {"bob", bob.c}} {
		list, _ := h.c.TeamChat(team.ID, 0)
		if len(list) != 2 || !reactionsEqual(list[0].Reactions, want) || list[1].ReplyTo != asked.ID {
			t.Fatalf("%s's chat: %+v", h.name, list)
		}
	}

	// Bob takes his back; his file written as ada's isn't taken by her.
	bob.c.ReactChat(proto.ChatReactParams{Team: team.ID, ID: asked.ID, Emoji: "👍", Off: true})
	forged := filepath.Join(bob.srv.teamsDir(), team.ID, "chat", "reactions", asked.ID, ada.acc+".json")
	os.WriteFile(forged, []byte(`{"account":"`+bob.acc+`","emojis":[{"emoji":"💩"}]}`), 0o600)
	both()
	list, _ := ada.c.TeamChat(team.ID, 0)
	if want := []proto.ChatReaction{{Emoji: "👍", Accounts: []string{ada.acc}}, {Emoji: "🎉", Accounts: []string{ada.acc}}}; !reactionsEqual(list[0].Reactions, want) {
		t.Fatalf("ada's reactions after bob's: %+v", list[0].Reactions)
	}

	if err := ada.c.DeleteChat(team.ID, asked.ID); err != nil {
		t.Fatal(err)
	}
	both()
	bob.sync(t, team.ID)
	for _, h := range []teamHost{ada, bob} {
		if _, err := os.Stat(filepath.Join(h.srv.teamsDir(), team.ID, "chat", "reactions", asked.ID)); !os.IsNotExist(err) {
			t.Errorf("reactions to the deleted message stay on a host: %v", err)
		}
	}
}

func sorted(s ...string) []string { slices.Sort(s); return s }

// reactionsEqual compares reactions whatever their order: two in the same
// millisecond keep none.
func reactionsEqual(a, b []proto.ChatReaction) bool {
	byEmoji := func(x, y proto.ChatReaction) int { return strings.Compare(x.Emoji, y.Emoji) }
	a, b = slices.Clone(a), slices.Clone(b)
	slices.SortFunc(a, byEmoji)
	slices.SortFunc(b, byEmoji)
	return slices.EqualFunc(a, b, func(x, y proto.ChatReaction) bool {
		return x.Emoji == y.Emoji && slices.Equal(x.Accounts, y.Accounts)
	})
}
