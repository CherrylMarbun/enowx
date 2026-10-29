//go:build !windows

package ui

import (
	"net"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
)

// The app reaches its other hosts for an agent asking of their sessions:
// it lists theirs, not the asking host's, and finds one by its host's
// name.
func TestAppReachesOtherHostsSessions(t *testing.T) {
	dial := func() (*client.Client, proto.DaemonInfo) {
		srv, err := daemon.NewServer(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(srv.Close)
		a, b := net.Pipe()
		go srv.ServeConn(a)
		cl, err := client.New(b, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cl.Close() })
		info, err := cl.Info()
		if err != nil {
			t.Fatal(err)
		}
		return cl, info
	}
	here, hereInfo := dial()
	vps, vpsInfo := dial()
	if _, err := vps.CreateSession(proto.SessionCreateParams{Name: "Login form", Dir: t.TempDir(), Cols: 80, Rows: 10, Command: []string{"sh", "-c", "sleep 30"}}); err != nil {
		t.Fatal(err)
	}
	app := New(Options{})
	app.client, app.daemonInfo = here, hereInfo
	app.host = hosts.Host{ID: hosts.LocalID, Name: "this Mac"}
	app.watches = map[string]*hostWatch{"vps": {host: hosts.Host{ID: "vps", Name: "datalix"}, cl: vps, info: vpsInfo}}

	type answer struct {
		v   any
		err error
	}
	got := make(chan answer, 1)
	reply := func(v any, err error) { got <- answer{v, err} }
	wait := func() answer {
		select {
		case a := <-got:
			return a
		case <-time.After(5 * time.Second):
			t.Fatal("no answer")
		}
		return answer{}
	}

	app.sessionsEverywhere(hereInfo.ID, reply)
	a := wait()
	list, _ := a.v.([]proto.HostSessions)
	if a.err != nil || len(list) != 1 || list[0].HostName != "datalix" || len(list[0].Sessions) != 1 {
		t.Fatalf("the other hosts' sessions: %+v %v", a.v, a.err)
	}
	app.peekSession(proto.SessionPeekParams{Host: "DATALIX", Session: "Login form"}, reply)
	a = wait()
	p, _ := a.v.(proto.SessionPeek)
	if a.err != nil || p.HostName != "datalix" || p.Session.Name != "Login form" || p.Reason == "" {
		t.Fatalf("a peek through the app: %+v %v", a.v, a.err)
	}
	app.peekSession(proto.SessionPeekParams{Host: "nowhere", Session: "x"}, reply)
	if a = wait(); a.err == nil {
		t.Fatal("peeked on a host the app doesn't reach")
	}
}
