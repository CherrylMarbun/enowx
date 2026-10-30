package daemon

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/hooks"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/transport"
)

// hookOf runs `enowx hook args`, the agent's JSON in, and returns its exit
// code, stdout and stderr.
func hookOf(in string, args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := Hook(args, strings.NewReader(in), &out, &errOut)
	return code, out.String(), errOut.String()
}

// hookServer is a daemon on this test's socket, and a session of it this
// process belongs to, as an agent's hooks do.
func hookServer(t *testing.T) (*Server, *session.Session) {
	t.Helper()
	// A short folder: Unix socket paths are limited to about 100 bytes.
	home, err := os.MkdirTemp("", "enx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("ENOWX_HOME", home)
	path, err := transport.SocketPath()
	if err != nil {
		t.Fatal(err)
	}
	l, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := startServer(t)
	go srv.Serve(l)
	t.Cleanup(func() { l.Close() })

	sess, err := srv.sessions.Create(session.Spec{Command: []string{"sh", "-c", "cat"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(session.EnvSession, sess.ID())
	return srv, sess
}

func TestHookReportsTheAgentsState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv, sess := hookServer(t)
	Hook := func(args []string) { Hook(args, strings.NewReader(""), io.Discard, io.Discard) }
	for _, state := range []string{proto.StateWorking, proto.StateWaiting} {
		Hook([]string{state})
		if got := sess.Info().State; got != state {
			t.Fatalf("after hook %s: state %q", state, got)
		}
	}
	// Answering the agent ends its wait.
	srv.sessions.Input(sess, []byte("1"))
	if got := sess.Info().State; got != "" {
		t.Fatalf("after answering: state %q", got)
	}
	// After a turn, the next prompt ends done; typing it doesn't.
	Hook([]string{proto.StateDone})
	srv.sessions.Input(sess, []byte("next"))
	if got := sess.Info().State; got != proto.StateDone {
		t.Fatalf("while typing a prompt: state %q", got)
	}
	srv.sessions.Input(sess, []byte("\r"))
	if got := sess.Info().State; got != "" {
		t.Fatalf("after sending it: state %q", got)
	}
	// An unknown state is refused, and the session keeps its own.
	Hook([]string{"bogus"})
	if got := sess.Info().State; got != "" {
		t.Fatalf("after a bogus state: %q", got)
	}
}

// The user's hooks answer the agent's events: a dangerous command is
// refused with its reason, a prompt gets context, a turn's end runs a
// command in the background; a project's own hooks wait for the user.
func TestHooksAnswerTheAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv, sess := hookServer(t)
	done := filepath.Join(t.TempDir(), "done")
	_, err := srv.hooks.store.Save([]proto.Hook{
		{Name: "No rm -rf", On: true, Event: proto.HookToolBefore, Tool: `rm\s+-rf`, Action: proto.HookActionBlock, Reason: "Not here."},
		{Name: "Rules", On: true, Event: proto.HookPromptStart, Action: proto.HookActionRun, Command: `echo "Project rules: $(cat | grep -o '"prompt":"[^"]*"')"`},
		{Name: "Off", On: false, Event: proto.HookPromptStart, Action: proto.HookActionContext, Context: "never"},
		{Name: "Done", On: true, Event: proto.HookTurnDone, Action: proto.HookActionRun, Command: "cat > " + done},
	})
	if err != nil {
		t.Fatal(err)
	}
	code, _, reason := hookOf(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"rm -rf build"}}`, "--event", proto.HookToolBefore)
	if code != hooks.ExitBlock || strings.TrimSpace(reason) != "Not here." {
		t.Fatalf("rm -rf: exit %d, %q", code, reason)
	}
	if code, _, _ := hookOf(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`, "--event", proto.HookToolBefore); code != 0 {
		t.Fatalf("ls refused: %d", code)
	}
	code, ctx, _ := hookOf(`{"prompt":"fix the login"}`, "--event", proto.HookPromptStart, "--state", proto.StateWorking)
	if code != 0 || strings.TrimSpace(ctx) != `Project rules: "prompt":"fix the login"` {
		t.Fatalf("prompt: exit %d, context %q", code, ctx)
	}
	if got := sess.Info().State; got != proto.StateWorking {
		t.Fatalf("state %q", got)
	}
	// The end of a turn waits for nothing: its hook runs after.
	if code, out, _ := hookOf(`{}`, "--event", proto.HookTurnDone, "--state", proto.StateDone); code != 0 || out != "" {
		t.Fatalf("turn done: %d %q", code, out)
	}
	waitFor(t, "the turn's hook", func() bool {
		data, _ := os.ReadFile(done)
		return strings.Contains(string(data), `"event":"turn.done"`)
	})
	log := srv.hookLog()
	if len(log) < 3 || !slices.ContainsFunc(log, func(r proto.HookRun) bool { return r.Name == "No rm -rf" && r.Blocked }) {
		t.Fatalf("log: %+v", log)
	}

	// A project's hooks run only once its file is allowed, as it is.
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".enowx"), 0o755)
	os.WriteFile(filepath.Join(dir, hooks.ProjectFile), []byte(`[{"name":"Theirs","on":true,"event":"tool.before","action":"block","reason":"theirs"}]`), 0o644)
	info := sess.Info()
	info.Dir = dir
	ev := proto.HookEvent{Event: proto.HookToolBefore, Tool: "Bash", Command: "ls", Via: proto.HookInAgent}
	if d := srv.fireHooks(info, ev); d.Block {
		t.Fatal("a project's hooks ran before they were allowed")
	}
	hi := srv.hooksInfo("")
	if len(hi.Pending) != 1 || hi.Pending[0].Dir != dir {
		t.Fatalf("pending: %+v", hi.Pending)
	}
	if err := srv.approveHooks(proto.HooksApproval{Dir: dir, Hash: hi.Pending[0].Hash, Allow: true}); err != nil {
		t.Fatal(err)
	}
	if d := srv.fireHooks(info, ev); !d.Block || d.Reason != "theirs" {
		t.Fatalf("allowed: %+v", d)
	}
	os.WriteFile(filepath.Join(dir, hooks.ProjectFile), []byte(`[{"name":"Changed","on":true,"event":"tool.before","action":"block"}]`), 0o644)
	if d := srv.fireHooks(info, ev); d.Block {
		t.Fatal("a changed file ran without asking again")
	}
	if hi.Support["claude"][proto.HookToolBefore] != proto.HookInAgent || hi.Support["aider"][proto.HookTurnDone] != proto.HookOutside {
		t.Fatalf("support: %v", hi.Support)
	}
}

// What `enowx display` prints exports the display's environment, quoted
// for the shell.
func TestExportLine(t *testing.T) {
	got := exportLine([]string{"DISPLAY=:101", "GDK_SCALE=2", "ODD=it's"})
	want := `export DISPLAY=':101' GDK_SCALE='2' ODD='it'\''s'`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

// An agent without hooks of its own: the daemon sees its session start
// and the files its turn changed, from outside.
func TestHooksSeenFromOutside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	dir := t.TempDir()
	if _, err := git.Run(dir, "init", "-q"); err != nil {
		t.Skip("no git")
	}
	os.WriteFile(filepath.Join(dir, "old.txt"), []byte("changed before"), 0o644)
	got := filepath.Join(t.TempDir(), "got")
	if _, err := srv.hooks.store.Save([]proto.Hook{
		{On: true, Event: proto.HookSessionStart, Agents: []string{"aider"}, Action: proto.HookActionRun, Command: "echo start >> " + got},
		{On: true, Event: proto.HookFileChanged, Paths: []string{"src/**"}, Action: proto.HookActionRun, Command: "cat >> " + got},
	}); err != nil {
		t.Fatal(err)
	}
	sess, err := srv.sessions.Create(session.Spec{Command: []string{"sh", "-c", "cat"}, Dir: dir, Cols: 80, Rows: 24, Meta: session.Meta{Agent: "aider"}})
	if err != nil {
		t.Fatal(err)
	}
	srv.sessions.Input(sess, []byte("x")) // an update: the daemon looks
	waitFor(t, "the session's start", func() bool { data, _ := os.ReadFile(got); return strings.Contains(string(data), "start") })

	info := sess.Info()
	srv.turnStarted(info)
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)
	os.WriteFile(filepath.Join(dir, "src", "new.go"), []byte("package src"), 0o644)
	srv.turnEnded(info)
	waitFor(t, "the files changed", func() bool {
		data, _ := os.ReadFile(got)
		return strings.Contains(string(data), `"paths":["src/new.go"]`)
	})
}
