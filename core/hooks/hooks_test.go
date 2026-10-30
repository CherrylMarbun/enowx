package hooks

import (
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

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

// Any runs on one condition met; Not turns one round; host and branch
// are conditions too.
func TestMatchesAnyNot(t *testing.T) {
	ev := proto.HookEvent{Event: proto.HookTurnDone, Agent: "codex", Host: "VPS-1", Branch: "feature/login"}
	any := proto.Hook{On: true, Event: proto.HookTurnDone, Any: true, Agents: []string{"claude"}, Branches: []string{"feature/*"}}
	not := proto.Hook{On: true, Event: proto.HookTurnDone, Branches: []string{"main"}, Not: []string{"branches"}, Hosts: []string{"vps-1"}}
	if !Matches(any, ev) || !Matches(not, ev) {
		t.Fatal("any or not didn't match")
	}
	ev.Branch = "main"
	if Matches(not, ev) {
		t.Fatal("not main ran on main")
	}
}

// Limits: once a session, at most N, a minimum interval, within hours.
func TestLimiter(t *testing.T) {
	var l Limiter
	at := time.Date(2026, 10, 31, 10, 0, 0, 0, time.Local)
	once := proto.Hook{ID: "a", Once: true}
	ev := proto.HookEvent{Session: "s1"}
	if !l.Take(once, ev, at) || l.Take(once, ev, at) || !l.Take(once, proto.HookEvent{Session: "s2"}, at) {
		t.Fatal("once")
	}
	every := proto.Hook{ID: "b", Every: 60}
	if !l.Take(every, ev, at) || l.Take(every, ev, at.Add(30*time.Second)) || !l.Take(every, ev, at.Add(61*time.Second)) {
		t.Fatal("every")
	}
	night := proto.Hook{ID: "c", Hours: "22:00-06:00"}
	if l.Take(night, ev, at) || !l.Take(night, ev, at.Add(13*time.Hour)) {
		t.Fatal("hours across midnight")
	}
}

// A hook changes a command, by find and replace or by what its command
// prints; a webhook posts the event; a message is filled in.
func TestRunActions(t *testing.T) {
	r := Runner{}
	ev := proto.HookEvent{Event: proto.HookToolBefore, Agent: "claude", Project: "/code/shop", Command: "npm install left-pad"}
	_, d := r.Run(proto.Hook{Action: proto.HookActionRewrite, Find: `^npm (install|i)\b`, Replace: "pnpm add"}, ev)
	if d.Command != "pnpm add left-pad" {
		t.Fatalf("rewrite: %q", d.Command)
	}
	if runtime.GOOS != "windows" {
		_, d = r.Run(proto.Hook{Action: proto.HookActionRun, Command: `echo '{"command":"ls -la"}'`}, ev)
		if d.Command != "ls -la" || d.Context != "" {
			t.Fatalf("printed rewrite: %+v", d)
		}
	}
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { got, _ = io.ReadAll(req.Body) }))
	defer srv.Close()
	run, _ := r.Run(proto.Hook{Action: proto.HookActionWebhook, URL: srv.URL + "/secret-token", Message: "{agent} in {project}: {command}"}, ev)
	if run.Error != "" || !strings.Contains(string(got), `"message":"claude in shop: npm install left-pad"`) || strings.Contains(run.Output, "secret") {
		t.Fatalf("webhook: %+v, posted %s", run, got)
	}
	var notified string
	r.Notify = func(kind, text string) { notified = kind + ": " + text }
	r.Run(proto.Hook{Action: proto.HookActionNotify, Kind: "input", Message: "{agent} waits"}, ev)
	if notified != "input: claude waits" {
		t.Fatalf("notify: %q", notified)
	}
}
