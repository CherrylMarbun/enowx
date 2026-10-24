package daemon

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/enowx/enowx/internal/testlisten"
	"github.com/enowx/enowx/proto"
)

func TestPortChangesSayWhatOpened(t *testing.T) {
	vite := proto.Port{Port: 5173, Addr: "127.0.0.1", PID: 10, Process: "node"}
	api := proto.Port{Port: 8080, Addr: "*", PID: 11, Process: "go"}
	before := map[string][]proto.Port{"a": {vite}, "b": {api}}
	now := map[string][]proto.Port{"a": {vite, api}, "c": {vite}}
	got := portChanges(before, now)
	if len(got) != 3 {
		t.Fatalf("changes = %+v", got)
	}
	a, b, c := got[0], got[1], got[2]
	if a.Session != "a" || len(a.Opened) != 1 || a.Opened[0].Port != 8080 || len(a.Ports) != 2 {
		t.Fatalf("a = %+v", a)
	}
	if b.Session != "b" || len(b.Ports) != 0 || len(b.Opened) != 0 {
		t.Fatalf("b closed its port: %+v", b)
	}
	if c.Session != "c" || len(c.Opened) != 1 {
		t.Fatalf("c = %+v", c)
	}
	if len(portChanges(now, now)) != 0 {
		t.Fatal("no change reported as one")
	}
}

func TestClientsHearOfAPortOpened(t *testing.T) {
	if !portsSupported {
		t.Skip("ports aren't read here")
	}
	every := portsEvery
	portsEvery = 200 * time.Millisecond
	t.Cleanup(func() { portsEvery = every })
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()

	srv := startServer(t)
	changes := make(chan proto.PortsChange, 16)
	c := connect(t, srv, func(ev proto.Event) {
		var ch proto.PortsChange
		if ev.Name == proto.EventPortsChanged && json.Unmarshal(ev.Data, &ch) == nil {
			changes <- ch
		}
	})
	time.Sleep(3 * portsEvery) // the watch's first look learns what is open
	info, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "sleep 1; " + testlisten.Shell(port)}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ch := <-changes:
			if ch.Session == info.ID && len(ch.Opened) == 1 && ch.Opened[0].Port == port {
				return
			}
		case <-deadline:
			t.Fatal("no event for the port opened")
		}
	}
}
