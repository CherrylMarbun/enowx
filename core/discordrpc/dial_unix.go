//go:build !windows

package discordrpc

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// systemDirs are where else Discord may be: on macOS, the user's own
// temporary folder, which TMPDIR may not name when set otherwise (by a
// shell, a build). Tests keep to theirs.
var systemDirs = func() []string {
	if runtime.GOOS != "darwin" {
		return nil
	}
	mine, _ := filepath.Glob("/private/var/folders/*/*/T")
	return mine
}

// dial finds Discord's socket, discord-ipc-0 to 9, in the folders it is
// made in: the runtime or temporary folder, and those of its Flatpak and
// Snap packages.
func dial() (io.ReadWriteCloser, error) {
	var dirs []string
	for _, env := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if d := os.Getenv(env); d != "" {
			dirs = append(dirs, d)
		}
	}
	dirs = append(dirs, "/tmp")
	dirs = append(dirs, systemDirs()...)
	var all []string
	for _, d := range dirs {
		all = append(all, d,
			filepath.Join(d, "app", "com.discordapp.Discord"), filepath.Join(d, "app", "com.discordapp.DiscordCanary"),
			filepath.Join(d, "app", "com.discordapp.DiscordPTB"), filepath.Join(d, "snap.discord"),
			filepath.Join(d, ".flatpak", "dev.vencord.Vesktop", "xdg-run"))
	}
	for _, d := range all {
		for i := range 10 {
			if c, err := net.Dial("unix", filepath.Join(d, "discord-ipc-"+strconv.Itoa(i))); err == nil {
				return c, nil
			}
		}
	}
	return nil, ErrNotRunning
}
