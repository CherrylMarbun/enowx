package ui

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

func TestForwardsEndWithTheirSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "hello") }))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port

	cl := &client.Client{}
	cl.SetTunnel(func(addr string) (net.Conn, error) { return net.Dial("tcp", addr) }) // a "server" that is this machine
	a := New(Options{})
	a.client = cl
	a.sessions = []proto.SessionInfo{{ID: "s1"}, {ID: "s2"}}
	a.forwardPort("s1", proto.Port{Port: port, Addr: "127.0.0.1"})
	local, ok := a.forwardAddr(port)
	if !ok {
		t.Fatal("not forwarded")
	}
	resp, err := http.Get("http://" + local)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "hello" {
		t.Fatalf("got %q", body)
	}
	a.remove("s2")
	if _, ok := a.forwardAddr(port); !ok {
		t.Fatal("another session's end stopped the forward")
	}
	a.remove("s1")
	if _, ok := a.forwardAddr(port); ok {
		t.Fatal("the forward outlived its session")
	}
	if _, err := net.Dial("tcp", local); err == nil {
		t.Fatal("still listening")
	}
}
