package userservice

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

// Start installs the daemon at exe as the user's service, or brings its
// unit up to date, and starts it. ok is false where systemd doesn't run
// user services; err says why one that should didn't start.
func Start(exe string) (ok bool, err error) {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false, nil // not booted with systemd
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false, nil
	}
	if err := userBus(); err != nil {
		return false, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	dir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	var env []string
	for _, k := range []string{"LANG", "LC_ALL", "ENOWX_HOME"} {
		if v := os.Getenv(k); v != "" {
			env = append(env, k+"="+v)
		}
	}
	unit := []byte(unitFile(exe, env))
	path := filepath.Join(dir, Unit)
	if old, err := os.ReadFile(path); err != nil || !bytes.Equal(old, unit) {
		if err := os.WriteFile(path, unit, 0o644); err != nil {
			return false, err
		}
		if err := systemctl("daemon-reload"); err != nil {
			return false, err
		}
	}
	if err := systemctl("enable", Unit); err != nil {
		return false, err
	}
	return true, systemctl("start", Unit)
}

// userBus makes sure the user's service manager runs and can be reached:
// lingering starts it, and keeps it, without a login session, which an SSH
// command run as another user (su) doesn't have.
func userBus() error {
	uid := strconv.Itoa(os.Getuid())
	run := "/run/user/" + uid
	if os.Getenv("XDG_RUNTIME_DIR") == "" {
		os.Setenv("XDG_RUNTIME_DIR", run)
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+run+"/bus")
	}
	if !lingers() {
		// Allowed for one's own user by default (polkit set-self-linger).
		if out, err := exec.Command("loginctl", "enable-linger").CombinedOutput(); err != nil {
			return fmt.Errorf("userservice: enable lingering: %v: %s", err, bytes.TrimSpace(out))
		}
	}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if _, err := os.Stat(run + "/bus"); err == nil {
			return nil
		}
	}
	return errors.New("userservice: the user's service manager didn't start")
}

func lingers() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	_, err = os.Stat("/var/lib/systemd/linger/" + u.Username)
	return err == nil
}

func systemctl(args ...string) error {
	out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("userservice: systemctl --user %v: %v: %s", args, err, bytes.TrimSpace(out))
	}
	return nil
}

// unitFile is the service's unit: exe serve, started again if it fails.
// A daemon stopped on purpose (an upgrade) exits cleanly and stays down
// until the next connection starts it.
func unitFile(exe string, env []string) string {
	s := "[Unit]\nDescription=enowX daemon\n\n[Service]\nExecStart=" + quote(exe) + " serve\nRestart=on-failure\nRestartSec=2\n"
	for _, e := range env {
		s += "Environment=" + quote(e) + "\n"
	}
	return s + "\n[Install]\nWantedBy=default.target\n"
}

// quote writes s as systemd reads a word, in double quotes.
func quote(s string) string {
	out := []byte{'"'}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\\':
			out = append(out, '\\', c)
		case '%':
			out = append(out, '%', '%')
		default:
			out = append(out, c)
		}
	}
	return string(append(out, '"'))
}
