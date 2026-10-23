package daemon

import (
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/core/display"
	"github.com/enowx/enowx/proto"
)

func TestSessionWithADisplay(t *testing.T) {
	if ok, why := display.Available(); !ok {
		t.Skip(why)
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	if info, err := c.Info(); err != nil || !info.Display {
		t.Fatalf("info = %+v, %v", info, err)
	}
	info, err := c.CreateSession(proto.SessionCreateParams{
		Command: []string{"sh", "-c", "echo on $DISPLAY; xdpyinfo >/dev/null && echo reached; sleep 30"},
		Display: true, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Display == 0 {
		t.Fatal("no display port")
	}
	sess, _ := srv.sessions.Get(info.ID)
	waitFor(t, "the session reaches its display", func() bool { return strings.Contains(sess.Terminal().Text(), "reached") })
	if !strings.Contains(sess.Terminal().Text(), "on :") {
		t.Fatalf("screen %q", sess.Terminal().Text())
	}
	ports, err := c.Ports(info.ID)
	if err != nil || len(ports.Ports) == 0 || ports.Ports[0].Port != info.Display || ports.Ports[0].Process != "display" {
		t.Fatalf("ports = %+v, %v", ports, err)
	}
	if err := c.KillSession(info.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if conn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(info.Display)); err == nil {
		conn.Close()
		t.Fatal("the display outlived its session")
	}
}
