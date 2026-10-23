package daemon

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
)

// The test binary run with ENOWX_TEST_SERVE=<port> is a dev server for a
// session: a page, a large download, and an echo of what is posted.
func init() {
	port := os.Getenv("ENOWX_TEST_SERVE")
	if port == "" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "<h1>dev</h1>") })
	mux.HandleFunc("/big", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(50<<20))
		chunk := bytes.Repeat([]byte("0123456789abcdef"), 4096)
		for i := 0; i < (50<<20)/len(chunk); i++ {
			w.Write(chunk)
		}
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) { io.Copy(w, r.Body) })
	fmt.Println("serving")
	_ = http.ListenAndServe("127.0.0.1:"+port, mux)
	os.Exit(0)
}

// devServer starts a session serving on a free port, and returns both.
func devServer(t *testing.T, c *client.Client) (proto.SessionInfo, int) {
	t.Helper()
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()
	info, err := c.CreateSession(proto.SessionCreateParams{
		Command: []string{os.Args[0]}, Env: []string{"ENOWX_TEST_SERVE=" + strconv.Itoa(port)}, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the dev server listens", func() bool {
		got, _ := c.Ports(info.ID)
		for _, p := range got.Ports {
			if p.Port == port {
				return true
			}
		}
		return false
	})
	return info, port
}

// get does an HTTP request over conn and returns the body.
func get(t *testing.T, conn net.Conn, method, path string, body []byte) []byte {
	t.Helper()
	req, _ := http.NewRequest(method, "http://localhost"+path, bytes.NewReader(body))
	// Send while reading the answer, as an HTTP client does: a server
	// that answers as it reads would wait on a client that only writes.
	sent := make(chan error, 1)
	go func() { sent <- req.Write(conn) }()
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTunnelToASessionsPort(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ports aren't read on Windows")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	info, port := devServer(t, c)

	tun, err := c.OpenTunnel(info.ID, port)
	if err != nil {
		t.Fatal(err)
	}
	if page := get(t, tun, "GET", "/", nil); string(page) != "<h1>dev</h1>" {
		t.Fatalf("page %q", page)
	}
	// Bytes in are bytes out, both ways.
	upload := bytes.Repeat([]byte("enowx"), 1<<20)
	if back := get(t, tun, "POST", "/echo", upload); sha256.Sum256(back) != sha256.Sum256(upload) {
		t.Fatalf("echoed %d bytes, sent %d", len(back), len(upload))
	}
	tun.Close()

	// Open again after closing.
	tun, err = c.OpenTunnel(info.ID, port)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()
	if page := get(t, tun, "GET", "/", nil); string(page) != "<h1>dev</h1>" {
		t.Fatalf("reopened: %q", page)
	}

	// Only the session's ports.
	other, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := c.OpenTunnel(info.ID, other.Addr().(*net.TCPAddr).Port); err == nil || !strings.Contains(err.Error(), "doesn't listen") {
		t.Fatalf("a port not the session's: %v", err)
	}
}

func TestTunnelDownloadLeavesTerminalsResponsive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ports aren't read on Windows")
	}
	srv := startServer(t)
	c := connect(t, srv, nil)
	info, port := devServer(t, c)

	// A session drawing all along, attached on the same connection.
	ticker, err := c.CreateSession(proto.SessionCreateParams{Command: []string{"sh", "-c", "while :; do date; sleep 0.05; done"}, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	st, err := c.Attach(ticker.ID)
	if err != nil {
		t.Fatal(err)
	}
	var lastFrame atomic.Int64 // UnixNano of the stream's last frame
	go func() {
		buf := make([]byte, 64<<10)
		for {
			if _, err := st.Read(buf); err != nil {
				return
			}
			lastFrame.Store(time.Now().UnixNano())
		}
	}()
	time.Sleep(500 * time.Millisecond)

	// Downloads, one after the other, for a while.
	var downloaded atomic.Int64
	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		tun, err := c.OpenTunnel(info.ID, port)
		if err != nil {
			t.Error(err)
			return
		}
		defer tun.Close()
		for {
			select {
			case <-stop:
				return
			default:
			}
			body := get(t, tun, "GET", "/big", nil)
			if len(body) != 50<<20 {
				t.Errorf("downloaded %d bytes", len(body))
				return
			}
			downloaded.Add(int64(len(body)))
		}
	}()
	var worstCall, worstFrame time.Duration
	for end := time.Now().Add(1500 * time.Millisecond); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		start := time.Now()
		if _, err := c.Sessions(); err != nil {
			t.Fatal(err)
		}
		worstCall = max(worstCall, time.Since(start))
		worstFrame = max(worstFrame, time.Since(time.Unix(0, lastFrame.Load())))
	}
	close(stop)
	<-finished
	t.Logf("%d MB downloaded; longest call %s, longest without a terminal frame %s",
		downloaded.Load()>>20, worstCall.Round(time.Millisecond), worstFrame.Round(time.Millisecond))
	if downloaded.Load() < 50<<20 {
		t.Fatal("no download finished")
	}
	if worstCall > 500*time.Millisecond || worstFrame > time.Second {
		t.Fatalf("held up: call %s, terminal %s", worstCall, worstFrame)
	}
}
