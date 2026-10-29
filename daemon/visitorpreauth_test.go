package daemon

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/proto"
)

// closedSoon says the server closed nc within d.
func closedSoon(nc net.Conn, d time.Duration) bool {
	nc.SetReadDeadline(time.Now().Add(d))
	for {
		_, err := nc.Read(make([]byte, 512))
		if err == nil {
			continue
		}
		var ne net.Error
		return !(errors.As(err, &ne) && ne.Timeout())
	}
}

// Before its ticket, a connection on the team listener gets no room for a
// big frame: the header claiming one ends it.
func TestVisitorPreAuthRefusesBigFrames(t *testing.T) {
	w := newVisitWorld(t, api.RoleViewer, false)
	nc, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	var hdr [9]byte
	binary.BigEndian.PutUint32(hdr[0:4], uint32(proto.MaxPayload+5))
	hdr[4] = byte(proto.TypeHello)
	nc.Write(hdr[:])
	nc.Write([]byte{'{'})
	if !closedSoon(nc, 2*time.Second) {
		t.Fatal("a pre-auth frame header claiming 16 MiB was waited for")
	}
}

// A connection that says nothing, or never hands its ticket, is closed
// after visitorSignInWait.
func TestVisitorPreAuthHasADeadline(t *testing.T) {
	old := visitorSignInWait
	visitorSignInWait = 300 * time.Millisecond
	t.Cleanup(func() { visitorSignInWait = old })
	w := newVisitWorld(t, api.RoleViewer, false)
	silent, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()
	nc, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	greeted, err := client.New(nc, nil) // says hello, hands nothing
	if err != nil {
		t.Fatal(err)
	}
	defer greeted.Close()
	if !closedSoon(silent, 3*time.Second) {
		t.Fatal("a silent connection was held past its sign-in time")
	}
	select {
	case <-greeted.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("a connection that handed no ticket was held past its sign-in time")
	}
}

// The first request other than team.auth ends a connection not signed in.
func TestVisitorPreAuthRefusalCloses(t *testing.T) {
	w := newVisitWorld(t, api.RoleViewer, false)
	nc, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	v, err := client.New(nc, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if _, err := v.Notes(); err == nil {
		t.Fatal("served before the ticket")
	}
	if _, err := v.TeamAuth(w.ticket(t)); err == nil {
		t.Fatal("a refused connection stayed open for a ticket")
	}
}

// One address opens so many connections at once and hands so many
// tickets a minute.
func TestVisitorPreAuthLimitsAnAddress(t *testing.T) {
	w := newVisitWorld(t, api.RoleViewer, false)
	var held []net.Conn
	for range teamConnsPerIP {
		nc, err := net.Dial("tcp", w.addr)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, nc)
	}
	time.Sleep(100 * time.Millisecond)
	extra, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	if !closedSoon(extra, 2*time.Second) {
		t.Fatalf("a connection past %d from one address was taken", teamConnsPerIP)
	}
	extra.Close()
	for _, nc := range held {
		nc.Close()
	}

	waitFor(t, "the connections let go", func() bool {
		w.ada.srv.visitors.gate.mu.Lock()
		defer w.ada.srv.visitors.gate.mu.Unlock()
		return w.ada.srv.visitors.gate.total == 0
	})
	for range authsPerIP {
		nc, err := net.Dial("tcp", w.addr)
		if err != nil {
			t.Fatal(err)
		}
		v, err := client.New(nc, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v.TeamAuth("enxt_forged"); err == nil {
			t.Fatal("a forged ticket let a visitor in")
		}
		v.Close()
	}
	nc, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	v, err := client.New(nc, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if _, err := v.TeamAuth(w.ticket(t)); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("a ticket past %d a minute from one address: %v", authsPerIP, err)
	}
}

// team.auth is answered one at a time: of two tickets sent at once, the
// second finds the connection signed in, never overwriting the first.
func TestVisitorTeamAuthOnce(t *testing.T) {
	w := newVisitWorld(t, api.RoleViewer, false)
	nc, err := net.Dial("tcp", w.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	hello, _ := json.Marshal(proto.Hello{Protocol: proto.Version})
	proto.WriteFrame(nc, proto.Frame{Type: proto.TypeHello, Payload: hello})
	if _, err := proto.ReadFrame(nc); err != nil {
		t.Fatal(err)
	}
	for i, tk := range []string{w.ticket(t), w.ticket(t)} {
		params, _ := json.Marshal(proto.TeamAuthParams{Ticket: tk})
		req, _ := json.Marshal(proto.Request{ID: uint64(i + 1), Method: proto.MethodTeamAuth, Params: params})
		proto.WriteFrame(nc, proto.Frame{Type: proto.TypeRequest, Payload: req})
	}
	errs := map[uint64]string{}
	nc.SetReadDeadline(time.Now().Add(10 * time.Second))
	for len(errs) < 2 {
		f, err := proto.ReadFrame(nc)
		if err == io.EOF || err != nil {
			t.Fatalf("answers: %v (%v)", errs, err)
		}
		if f.Type != proto.TypeResponse {
			continue
		}
		var resp proto.Response
		json.Unmarshal(f.Payload, &resp)
		errs[resp.ID] = resp.Error
	}
	if errs[1] != "" || !strings.Contains(errs[2], "signed in already") {
		t.Fatalf("two tickets at once: %v", errs)
	}
}
