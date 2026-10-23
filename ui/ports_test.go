package ui

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/daemon"
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

func TestForwardGoesThroughTheDaemonWhenSSHRefuses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	if _, err := exec.LookPath("nc"); err != nil {
		t.Skip("needs nc")
	}
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	p1, p2 := net.Pipe()
	go srv.ServeConn(p1)
	cl, err := client.New(p2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	cl.SetTunnel(func(string) (net.Conn, error) { return nil, errors.New("administratively prohibited") })

	free, _ := net.Listen("tcp", "127.0.0.1:0")
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()
	serve := fmt.Sprintf(`while :; do printf 'HTTP/1.0 200 OK\r\nContent-Length: 2\r\n\r\nhi' | nc -l 127.0.0.1 %d; done`, port)
	info, err := cl.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", serve}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		got, _ := cl.Ports(info.ID)
		if len(got.Ports) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the dev server never listened")
		}
		time.Sleep(100 * time.Millisecond)
	}

	a := New(Options{})
	a.client = cl
	a.forwardPort(info.ID, proto.Port{Port: port, Addr: "127.0.0.1"})
	defer a.stopForwards("")
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
	if string(body) != "hi" || !a.sshRefused.Load() {
		t.Fatalf("body %q, through the daemon %v", body, a.sshRefused.Load())
	}
}
