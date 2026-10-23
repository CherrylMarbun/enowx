package transport

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestForwardCarriesConnections(t *testing.T) {
	// The "server's" dev server: answers each line with it, upper-cased.
	srv, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go func() {
		for {
			c, err := srv.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				line, _ := bufio.NewReader(c).ReadString('\n')
				c.Write([]byte("GOT " + line))
			}()
		}
	}()
	port := srv.Addr().(*net.TCPAddr).Port

	// The port is taken (by the dev server itself): the forward picks another.
	l, err := Forward(port, func() (net.Conn, error) { return net.Dial("tcp", srv.Addr().String()) })
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if l.Addr().(*net.TCPAddr).Port == port {
		t.Fatal("listened on a taken port")
	}
	for range 2 {
		c, err := net.Dial("tcp", l.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		c.Write([]byte("hello\n"))
		got, _ := bufio.NewReader(c).ReadString('\n')
		c.Close()
		if got != "GOT hello\n" {
			t.Fatalf("got %q", got)
		}
	}
}

func TestForwardCarriesHTTPAndWebSockets(t *testing.T) {
	// A dev server: a page, and an upgrade to a connection that echoes,
	// as HMR's WebSocket does.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "<h1>app</h1>") })
	mux.HandleFunc("/hmr", func(w http.ResponseWriter, r *http.Request) {
		conn, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close()
		rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		rw.Flush()
		io.Copy(conn, rw) // echo until the client closes
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	l, err := Forward(0, func() (net.Conn, error) { return net.Dial("tcp", srv.Listener.Addr().String()) })
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	resp, err := http.Get("http://" + l.Addr().String() + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(page) != "<h1>app</h1>" {
		t.Fatalf("page %q", page)
	}

	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	fmt.Fprintf(c, "GET /hmr HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	r := bufio.NewReader(c)
	status, _ := r.ReadString('\n')
	if !strings.Contains(status, "101") {
		t.Fatalf("upgrade: %q", status)
	}
	for line, _ := r.ReadString('\n'); line != "\r\n" && line != ""; line, _ = r.ReadString('\n') {
	}
	for _, msg := range []string{"update a.tsx\n", "update b.tsx\n"} {
		c.Write([]byte(msg))
		got, err := r.ReadString('\n')
		if err != nil || got != msg {
			t.Fatalf("echo %q, %v", got, err)
		}
	}
}
