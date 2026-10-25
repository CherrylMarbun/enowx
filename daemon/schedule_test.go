package daemon

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

// fakeAgent puts a "claude" on PATH that keeps working: it prints until
// it ends.
func fakeAgent(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\nwhile :; do echo working; sleep 0.1; done\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("never: " + what)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestAgentsPastTheLimitWaitTheirTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	fakeAgent(t)
	srv := startServer(t)
	c := connect(t, srv, nil)
	if _, err := c.SetSettings(proto.DaemonSettings{MaxAgents: 1}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	first, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: dir, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: dir, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	if first.Queued || !second.Queued {
		t.Fatalf("first queued %v, second queued %v", first.Queued, second.Queued)
	}
	info := func(id string) proto.SessionInfo {
		sess, err := srv.sessions.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		return sess.Info()
	}
	time.Sleep(3 * time.Second)
	if !info(second.ID).Queued {
		t.Fatal("started while the first agent works")
	}
	// The first one ends: the second starts, in the same session.
	if err := c.KillSession(first.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the queued agent starts", func() bool { return !info(second.ID).Queued })
	waitFor(t, "it works", func() bool { return info(second.ID).Active })
	if list, _ := c.Sessions(); len(list) != 1 || list[0].ID != second.ID {
		t.Fatalf("sessions = %+v", list)
	}
	if got, _ := c.Settings(); got.MaxAgents != 1 {
		t.Fatalf("settings = %+v", got)
	}
}

func TestBudgetInterruptsTheAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	// A worker that prints while it works, and stops when interrupted.
	info, err := c.CreateSession(proto.SessionCreateParams{
		Command: []string{"sh", "-c", "trap 'echo interrupted; sleep 30' INT; while :; do echo working; sleep 0.1; done"},
		Budget:  1, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := srv.sessions.Get(info.ID)
	waitFor(t, "the budget runs out", func() bool { return sess.Info().OverBudget })
	waitFor(t, "the worker is interrupted", func() bool { return strings.Contains(sess.Terminal().Text(), "interrupted") })
	if got := sess.Info(); got.Worked < 1 || got.Budget != 1 {
		t.Fatalf("info = worked %d, budget %d", got.Worked, got.Budget)
	}
	// A new budget lifts it.
	if err := c.SetBudget(info.ID, time.Hour); err != nil {
		t.Fatal(err)
	}
	if sess.Info().OverBudget {
		t.Fatal("still over a budget of an hour")
	}
}

// An agent done and idle past the setting is stopped, its session kept
// as hibernated; continuing brings it back.
func TestIdleAgentsHibernateAndWake(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	fakeAgent(t)
	srv := startServer(t)
	c := connect(t, srv, nil)
	info, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Call(proto.MethodSessionState, proto.SessionStateParams{ID: info.ID, State: proto.StateDone}, nil); err != nil {
		t.Fatal(err)
	}
	sess, _ := srv.sessions.Get(info.ID)
	if sess.DoneFor(time.Now()) > time.Second {
		t.Fatal("done longer than since it said so")
	}
	if _, err := c.SetSettings(proto.DaemonSettings{HibernateAfter: 1}); err != nil {
		t.Fatal(err)
	}
	// What hibernating does, without waiting a minute of quiet.
	if err := srv.hibernateSession(sess, false); err != nil {
		t.Fatal(err)
	}
	list, _ := c.Sessions()
	if len(list) != 1 || !list[0].Hibernated || !list[0].Exited {
		t.Fatalf("after hibernating: %+v", list)
	}
	woke, err := c.ContinueSession(info.ID)
	if err != nil || woke.Hibernated || woke.Exited {
		t.Fatalf("woken: %+v, %v", woke, err)
	}
	st, _ := c.Settings()
	if st.HibernateAfter != 1 {
		t.Fatalf("settings %+v", st)
	}
}

// The scheduler picks agents done and quiet past the window, and leaves
// the others.
func TestHibernationPicksQuietDoneAgents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\necho ready\nsleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	hibernateUnit = 50 * time.Millisecond
	defer func() { hibernateUnit = time.Minute }()
	srv := startServer(t)
	c := connect(t, srv, nil)
	quiet, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	busy, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second) // both said ready
	if err := c.Call(proto.MethodSessionState, proto.SessionStateParams{ID: quiet.ID, State: proto.StateDone}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetSettings(proto.DaemonSettings{HibernateAfter: 2}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the quiet agent to hibernate", func() bool {
		srv.hibernate()
		list, _ := c.Sessions()
		for _, s := range list {
			if s.ID == quiet.ID {
				return s.Hibernated
			}
		}
		return false
	})
	list, _ := c.Sessions()
	for _, s := range list {
		if s.ID == busy.ID && (s.Hibernated || s.Exited) {
			t.Fatalf("an agent not done hibernated: %+v", s)
		}
	}
}

// Stopping the host's agents stops each running one, working or not,
// keeping its session to continue; a plain terminal keeps running.
func TestStopAgentsStopsEveryRunningAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	fakeAgent(t)
	srv := startServer(t)
	c := connect(t, srv, nil)
	var agents []string
	for range 2 {
		info, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: t.TempDir(), Cols: 80, Rows: 10})
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, info.ID)
	}
	shell, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh"}, Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	n, err := c.StopAgents()
	if err != nil || n != 2 {
		t.Fatalf("stopped %d, %v", n, err)
	}
	list, _ := c.Sessions()
	for _, s := range list {
		stopped := s.Exited && s.Hibernated
		if (s.ID == shell.ID) == stopped {
			t.Fatalf("after stopping agents: %+v", s)
		}
	}
	if woke, err := c.ContinueSession(agents[0]); err != nil || woke.Exited {
		t.Fatalf("continued: %+v, %v", woke, err)
	}
}

// An agent done and idle that still serves a port (a dev server it
// started) isn't hibernated: the server would stop with it.
func TestHibernationSparesAgentsServingAPort(t *testing.T) {
	if runtime.GOOS == "windows" || !portsSupported {
		t.Skip("uses sh and the host's ports")
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	bin := t.TempDir()
	script := "#!/bin/sh\necho ready\nnc -l 127.0.0.1 " + strconv.Itoa(port) + "\nsleep 60\n"
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	hibernateUnit = 50 * time.Millisecond
	defer func() { hibernateUnit = time.Minute }()
	srv := startServer(t)
	c := connect(t, srv, nil)
	info, err := c.CreateSession(proto.SessionCreateParams{Agent: "claude", Dir: t.TempDir(), Cols: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the port open", func() bool {
		ports, _ := srv.allPorts()
		return len(ports[info.ID]) > 0
	})
	if err := c.Call(proto.MethodSessionState, proto.SessionStateParams{ID: info.ID, State: proto.StateDone}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SetSettings(proto.DaemonSettings{HibernateAfter: 1}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // past the window
	srv.hibernate()
	list, _ := c.Sessions()
	if len(list) != 1 || list[0].Hibernated || list[0].Exited {
		t.Fatalf("a session serving a port hibernated: %+v", list)
	}
}

// Agents hibernate after 30 idle minutes until the user picks otherwise;
// "Never" picked stays never.
func TestHibernationDefaultsTo30Minutes(t *testing.T) {
	dir := t.TempDir()
	var sc scheduler
	if err := sc.open(settingsFile(dir)); err != nil || sc.get().HibernateAfter != 30 {
		t.Fatalf("no settings yet: %+v, %v", sc.get(), err)
	}
	// Saved by a daemon before the default: other settings, no window.
	if err := os.WriteFile(settingsFile(dir), []byte(`{"maxAgents":3}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := sc.open(settingsFile(dir)); err != nil || sc.get().HibernateAfter != 30 || sc.get().MaxAgents != 3 {
		t.Fatalf("settings of an older daemon: %+v, %v", sc.get(), err)
	}
	if _, err := sc.set(proto.DaemonSettings{MaxAgents: 3}); err != nil {
		t.Fatal(err)
	}
	var again scheduler
	if err := again.open(settingsFile(dir)); err != nil || again.get().HibernateAfter != 0 {
		t.Fatalf("Never picked: %+v, %v", again.get(), err)
	}
}
