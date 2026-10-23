package chatview

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

func TestSlashMatchesPrefixFirst(t *testing.T) {
	ch := New()
	ch.commands = []proto.AgentCommand{
		{Name: "compact"}, {Name: "model"}, {Name: "remote-model"}, {Name: "review", Description: "pick a model to review"},
	}
	ch.input = "/mo"
	got := ch.slashMatches()
	if len(got) != 3 || got[0].Name != "model" || got[1].Name != "remote-model" || got[2].Name != "review" {
		t.Fatalf("matches = %+v", got)
	}
	for _, in := range []string{"/model ", "hello", "/a\nb"} {
		ch.input = in
		if m := ch.slashMatches(); m != nil {
			t.Errorf("%q matched %+v", in, m)
		}
	}
	ch.input = "/mo"
	ch.slashClosed = "/mo"
	if ch.slashMatches() != nil {
		t.Error("a closed menu matched")
	}
}
