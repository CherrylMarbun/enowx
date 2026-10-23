package procstat

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"testing"
)

func TestListeningFindsOwnPort(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("not read on " + runtime.GOOS)
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	got, err := Listening([]int{os.Getpid()})
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range got {
		if g.Port == port && g.PID == os.Getpid() && g.Loopback() {
			return
		}
	}
	t.Fatalf("listening = %+v, want port %d", got, port)
}

func TestParseListeners(t *testing.T) {
	got := parseLsof("p42\nf3\nn*:3000\nf4\nn[::1]:5173\np7\nn127.0.0.1:8080\n")
	want := []Listener{{42, 3000, "*"}, {42, 5173, "::1"}, {7, 8080, "127.0.0.1"}}
	if len(got) != len(want) {
		t.Fatalf("lsof = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lsof = %+v, want %+v", got, want)
		}
	}
	for in, want := range map[string]string{"0100007F:1F90": "127.0.0.1 8080", "00000000:0BB8": "* 3000"} {
		addr, port, ok := parseProcAddr(in)
		if !ok || addr+" "+itoa(port) != want {
			t.Errorf("%s = %s %d", in, addr, port)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
