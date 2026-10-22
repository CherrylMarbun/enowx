// Package transport connects the app to a daemon: on this machine through
// a local socket, or on a server through SSH.
package transport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Dir returns enowX's per-user directory (~/.enowx), creating it.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".enowx")
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
	return filepath.Join(dir, "enowxd.sock"), nil
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
	return l, nil
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
// output going to ~/.enowx/enowxd.log.
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
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("unix", path, 200*time.Millisecond)
		if err != nil {
			return nil
		}
		c.Close()
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("transport: the old daemon did not stop within 5s")
}
