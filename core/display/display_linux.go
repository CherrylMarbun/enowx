package display

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// startTimeout bounds how long Xpra may take to serve its client.
const startTimeout = 20 * time.Second

// firstNumber is where display numbers start, clear of those a desktop
// or a VNC server takes.
const firstNumber = 100

// Available reports whether this host can run displays, and if not, what
// it needs.
func Available() (bool, string) {
	if _, err := exec.LookPath("xpra"); err != nil {
		return false, "needs Xpra: install it with your distribution's package (sudo apt install xpra) or from xpra.org"
	}
	return true, ""
}

// Start starts a display with Xpra, serving its HTML5 client on a free
// loopback port.
func Start() (*Display, error) {
	if ok, why := Available(); !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, why)
	}
	number := freeNumber()
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("xpra", "start", ":"+strconv.Itoa(number),
		"--bind-tcp=127.0.0.1:"+strconv.Itoa(port), "--html=on", "--daemon=no",
		"--mdns=no", "--pulseaudio=no", "--notifications=no", "--systemd-run=no", "--printing=no", "--webcam=no")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("display: start xpra: %w", err)
	}
	var once sync.Once
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	stop := func() {
		once.Do(func() {
			_ = exec.Command("xpra", "stop", ":"+strconv.Itoa(number)).Run()
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		})
	}
	alive := func() error {
		select {
		case err := <-exited:
			exited <- err
			msg := strings.TrimSpace(out.String())
			if i := strings.LastIndexByte(msg, '\n'); i >= 0 && len(msg) > 400 {
				msg = msg[i+1:]
			}
			return fmt.Errorf("display: xpra ended: %v: %s", err, msg)
		default:
			return nil
		}
	}
	if err := waitServing(port, alive); err != nil {
		stop()
		return nil, err
	}
	return &Display{Number: number, Port: port, stop: stop}, nil
}

// freeNumber is a display number no X server holds: neither its lock
// file nor its socket exists.
func freeNumber() int {
	for n := firstNumber; ; n++ {
		lock := "/tmp/.X" + strconv.Itoa(n) + "-lock"
		sock := "/tmp/.X11-unix/X" + strconv.Itoa(n)
		if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if _, err := os.Stat(sock); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		return n
	}
}

// freePort is a loopback TCP port nothing listens on now.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// waitServing waits for port to take connections, while alive says the
// server runs.
func waitServing(port int, alive func() error) error {
	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) {
		if err := alive(); err != nil {
			return err
		}
		if c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 200*time.Millisecond); err == nil {
			c.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("display: the display didn't serve :%d within %s", port, startTimeout)
}
