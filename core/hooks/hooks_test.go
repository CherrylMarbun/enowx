package hooks

import (
	"testing"

	"github.com/enowx/enowx/proto"
)

// A hook runs only when every condition it sets holds.
func TestMatches(t *testing.T) {
	ev := proto.HookEvent{Event: proto.HookToolBefore, Agent: "codex", Dir: "/w/enowx-wt", Project: "/code/enowx", Tool: "Bash", Command: "git push origin main", Paths: []string{"ui/app.go"}}
	cases := []struct {
		name string
		hook proto.Hook
		want bool
	}{
		{"any", proto.Hook{On: true, Event: proto.HookToolBefore}, true},
		{"off", proto.Hook{Event: proto.HookToolBefore}, false},
		{"another event", proto.Hook{On: true, Event: proto.HookToolAfter}, false},
		{"its agent", proto.Hook{On: true, Event: proto.HookToolBefore, Agents: []string{"claude", "codex"}}, true},
		{"another agent", proto.Hook{On: true, Event: proto.HookToolBefore, Agents: []string{"claude"}}, false},
		{"project by name", proto.Hook{On: true, Event: proto.HookToolBefore, Projects: []string{"enowx"}}, true},
		{"another project", proto.Hook{On: true, Event: proto.HookToolBefore, Projects: []string{"site"}}, false},
		{"command, any case", proto.Hook{On: true, Event: proto.HookToolBefore, Tool: `GIT PUSH.*\bmain\b`}, true},
		{"tool name", proto.Hook{On: true, Event: proto.HookToolBefore, Tool: `^Bash$`}, true},
		{"another command", proto.Hook{On: true, Event: proto.HookToolBefore, Tool: `rm -rf`}, false},
		{"files", proto.Hook{On: true, Event: proto.HookToolBefore, Paths: []string{"*.go"}}, true},
		{"other files", proto.Hook{On: true, Event: proto.HookToolBefore, Paths: []string{"docs/**"}}, false},
		{"all of them", proto.Hook{On: true, Event: proto.HookToolBefore, Agents: []string{"codex"}, Tool: "push", Paths: []string{"ui/**"}}, true},
	}
	for _, c := range cases {
		if got := Matches(c.hook, ev); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if Check(proto.Hook{Event: proto.HookToolBefore, Action: proto.HookActionRun}) == nil || Check(proto.Hook{Event: "x", Action: proto.HookActionBlock}) == nil ||
		Check(proto.Hook{Event: proto.HookToolBefore, Action: proto.HookActionBlock, Tool: "("}) == nil {
		t.Fatal("a bad hook passed Check")
	}
}
