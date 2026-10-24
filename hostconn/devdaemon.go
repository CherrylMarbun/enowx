package hostconn

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// A development app (make dev, ENOWX_DEV) rebuilds itself on every change,
// but the Linux daemons it embeds only when it starts: servers would keep
// running code older than the app's. It builds them from the source it
// runs in instead, once per launch.

var devDaemons sync.Map // arch → path of the daemon built, "" if it couldn't

// devDaemon is the path of a daemon for linux/arch built from this
// source, or "" outside development or when it can't build one.
func devDaemon(arch string) string {
	if os.Getenv("ENOWX_DEV") == "" {
		return ""
	}
	if p, ok := devDaemons.Load(arch); ok {
		return p.(string)
	}
	p := ""
	if root := moduleRoot(); root != "" {
		out := filepath.Join(os.TempDir(), "enowx-dev-enowxd-linux-"+arch)
		cmd := exec.Command("go", "build", "-trimpath", "-o", out, "./cmd/enowxd")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
		if cmd.Run() == nil {
			p = out
		}
	}
	devDaemons.Store(arch, p)
	return p
}

// moduleRoot is the folder of enowX's go.mod above the working folder.
func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if f, err := os.Open(filepath.Join(dir, "go.mod")); err == nil {
			line, _ := bufio.NewReader(f).ReadString('\n')
			f.Close()
			if strings.TrimSpace(line) == "module github.com/enowx/enowx" {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
