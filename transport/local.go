// Package transport connects the app to a daemon: on this machine through
// a local socket, or on a server through SSH.
package transport

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// HomeName is enowX's per-user folder in the home folder, on this device
// and on servers. ~/.enowx was enowX's before, and is another app's.
const HomeName = ".enowx-app"

// Dir returns enowX's per-user directory, creating it: $ENOWX_HOME when
// set (so a development build keeps its daemon and state apart), else
// ~/.enowx-app. Daemons the app starts inherit the variable.
func Dir() (string, error) {
	dir := os.Getenv("ENOWX_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, HomeName)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// SocketPath returns the daemon's socket path. Unix sockets work on
// Windows 10 1803+ too, so one path scheme serves every platform.
func SocketPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return SocketPathIn(dir)
}

// SocketPathIn is the socket of the daemon whose folder is dir.
func SocketPathIn(dir string) (string, error) {
	path := filepath.Join(dir, "enowxd.sock")
	if len(path) <= maxSocketPath {
		return path, nil
	}
	// Socket paths are limited (104 bytes on macOS): a deep ENOWX_HOME
	// gets its socket in a private folder of the temporary one, named
	// after it so each home keeps its own daemon.
	sum := sha256.Sum256([]byte(dir))
	short := filepath.Join(shortTemp(), fmt.Sprintf("enowx-%d-%x", os.Getuid(), sum[:6]))
	if err := os.MkdirAll(short, 0o700); err != nil {
		return "", err
	}
	if !private(short) {
		return "", fmt.Errorf("transport: %s isn't private", short)
	}
	return filepath.Join(short, "enowxd.sock"), nil
}

// maxSocketPath is the longest socket path every platform takes.
const maxSocketPath = 100

// shortTemp is a temporary folder with a short path: macOS's own is deep.
func shortTemp() string {
	if fi, err := os.Stat("/tmp"); err == nil && fi.IsDir() {
		return "/tmp"
	}
	return os.TempDir()
}

// Listen listens on the daemon socket. It fails if another daemon is
// serving it, and removes a stale socket file left by one that died.
func Listen(path string) (net.Listener, error) {
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		c.Close()
		return nil, fmt.Errorf("transport: a daemon is already listening on %s", path)
	}
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("transport: listen %s: %w", path, err)
	}
	_ = os.Chmod(path, 0o600)
	// Closing removes the socket file only while it is this listener's: a
	// daemon left over (one whose home moved away under it) mustn't take
	// the path from the daemon serving it now.
	ul, ok := l.(*net.UnixListener)
	if !ok {
		return l, nil
	}
	ul.SetUnlinkOnClose(false)
	fi, err := os.Stat(path)
	if err != nil {
		return l, nil
	}
	return ownSocket{ul, path, fi}, nil
}

// ownSocket is a listener whose socket file goes with it, if still its own.
type ownSocket struct {
	*net.UnixListener
	path string
	file os.FileInfo
}

func (o ownSocket) Close() error {
	err := o.UnixListener.Close()
	if fi, statErr := os.Stat(o.path); statErr == nil && os.SameFile(fi, o.file) {
		os.Remove(o.path)
	}
	return err
}

// DialLocal connects to the daemon on path. If none is running and spawn
// is not nil, it runs spawn's command detached and waits for the socket.
func DialLocal(path string, spawn *exec.Cmd) (net.Conn, error) {
	if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
		return c, nil
	} else if spawn == nil {
		return nil, fmt.Errorf("transport: no daemon on %s: %w", path, err)
	}
	if err := startDetached(spawn); err != nil {
		return nil, fmt.Errorf("transport: start daemon: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("unix", path, time.Second); err == nil {
			return c, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, errors.New("transport: daemon did not start within 10s")
}

// startDetached starts cmd in its own session, outliving its parent, with
// output going to enowxd.log in Dir.
func startDetached(cmd *exec.Cmd) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	logf, err := os.OpenFile(filepath.Join(dir, "enowxd.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd.Stdin = nil
	cmd.Stdout, cmd.Stderr = logf, logf
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// StopDaemon stops the daemon with the given PID and waits until its
// socket at path stops answering.
func StopDaemon(pid int, path string) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := stopProcess(p); err != nil {
		return fmt.Errorf("transport: stop daemon %d: %w", pid, err)
	}
	// Gone, not only quiet: a daemon closes its socket first, then hangs
	// up its sessions, and one started meanwhile would run them twice.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if !running(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("transport: the old daemon did not stop within 15s")
}

// lockWait is how long a daemon waits for one stopping to let go of the
// lock.
const lockWait = 15 * time.Second

// LockDaemon makes this process the one daemon of the user, holding the
// lock file in dir for as long as it runs. A daemon stopping holds it until
// it has hung up its sessions, so a new one waits for that instead of
// running them twice; it fails when another daemon runs.
func LockDaemon(dir string) (*os.File, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "enowxd.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("transport: daemon lock: %w", err)
	}
	if !lockFile(f, lockWait) {
		f.Close()
		return nil, errors.New("transport: another daemon is running")
	}
	return f, nil
}
