package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/internal/quietcmd"
	"github.com/enowx/enowx/transport"
)

// enowX kept its state in ~/.enowx, a folder another app of the same name
// uses too. MoveOldHome carries enowX's own files from there to its folder
// now (transport.HomeName), once, and leaves the rest where it is.

// oldHomeName is the folder enowX used before.
const oldHomeName = ".enowx"

// ownedNames are what enowX kept in its folder: the rest is another app's.
var ownedNames = []string{
	"app.json", "hosts.json", "projects.json", "sessions.json", "settings.json", "session-env.json",
	"checks.json", "commands.json", "accounts.json", "pipelines.json", "automations.json", "enowxd.log",
	"history", "worktrees", "accounts", "tsnet", "uploads", "transfers", "screenshots", "setup",
	filepath.Join("bin", "enowxd"),
}

// rewritten are the files that name paths in the folder, written anew for
// the new one.
var rewritten = []string{"sessions.json", "pipelines.json", "automations.json"}

// MoveOldHome moves enowX's files from ~/.enowx to its folder, stopping
// the daemon that ran there first, unless the folder exists already or
// $ENOWX_HOME names another. Every enowX program calls it before using
// its folder; one moves, the others wait for it.
func MoveOldHome() error {
	if os.Getenv("ENOWX_HOME") != "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return moveHome(filepath.Join(home, oldHomeName), filepath.Join(home, transport.HomeName))
}

func moveHome(old, dir string) error {
	if hasState(dir) || !ownsAny(old) {
		return nil
	}
	// A server's folder may exist already, with only the daemon the app
	// just installed: what is there stays, and the rest comes in beside.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	moving := filepath.Join(dir, ".moving")
	if err := os.Mkdir(moving, 0o700); errors.Is(err, os.ErrExist) {
		// Another program moves it: wait for it to be done.
		for deadline := time.Now().Add(time.Minute); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
			if !exists(moving) {
				return nil
			}
		}
		return errors.New("daemon: the move of enowX's folder didn't finish: remove " + moving + " and start again")
	} else if err != nil {
		return err
	}
	defer os.Remove(moving)
	stopDaemonIn(old)
	for _, name := range ownedNames {
		from, to := filepath.Join(old, name), filepath.Join(dir, name)
		if !exists(from) || exists(to) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
			return err
		}
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	for _, name := range rewritten {
		f := filepath.Join(dir, name)
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		data = []byte(strings.ReplaceAll(string(data), old+string(filepath.Separator), dir+string(filepath.Separator)))
		if err := os.WriteFile(f, data, 0o600); err != nil {
			return err
		}
	}
	for _, name := range []string{"enowxd.sock", "enowxd.lock"} {
		_ = os.Remove(filepath.Join(old, name))
	}
	repairWorktrees(filepath.Join(dir, "worktrees"))
	return nil
}

// stopDaemonIn stops the daemon serving folder dir, if one does, so that
// its sessions are saved to come back with the next.
func stopDaemonIn(dir string) {
	sock, err := transport.SocketPathIn(dir)
	if err != nil {
		return
	}
	conn, err := transport.DialLocal(sock, nil)
	if err != nil {
		return
	}
	c, err := client.New(conn, nil)
	if err != nil {
		return
	}
	info, err := c.Info()
	c.Close()
	if err == nil {
		_ = transport.StopDaemon(info.PID, sock)
	}
}

// repairWorktrees tells git where the worktrees under root moved, from
// each of them, which fixes the links both ways.
func repairWorktrees(root string) {
	gits, _ := filepath.Glob(filepath.Join(root, "*", "*", ".git"))
	for _, g := range gits {
		_ = quietcmd.Command("git", "-C", filepath.Dir(g), "worktree", "repair").Run()
	}
}

// hasState reports whether dir holds enowX's state, not only a daemon.
func hasState(dir string) bool {
	for _, name := range []string{"sessions.json", "projects.json", "hosts.json", "app.json"} {
		if exists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func ownsAny(dir string) bool {
	for _, name := range []string{"sessions.json", "projects.json", "hosts.json", "app.json", filepath.Join("bin", "enowxd")} {
		if exists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
