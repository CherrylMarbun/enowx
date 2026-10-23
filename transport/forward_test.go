package transport

import (
	"bufio"
	"net"
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
