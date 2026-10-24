package hostconn

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/transport"
	"github.com/enowx/enowx/transport/owntailnet"
)

// own is enowX's own tailnet device, joined the first time a host
// asks for it and kept for the app's life, since every host shares it.
var own struct {
	mu sync.Mutex
	tn *owntailnet.Tailnet
}

// ControlEnv names the coordination server enowX's tailnet device joins,
// for a self-hosted one (Headscale); unset is Tailscale's.
const ControlEnv = "ENOWX_TAILNET_CONTROL"

// ownTailnet joins enowX's tailnet device, or returns it if joined. Its
// state lives in tsnet under enowX's folder, so it signs in once.
func ownTailnet(opts Options) (*owntailnet.Tailnet, error) {
	own.mu.Lock()
	defer own.mu.Unlock()
	if own.tn != nil {
		return own.tn, nil
	}
	dir, err := transport.Dir()
	if err != nil {
		return nil, err
	}
	name, _ := os.Hostname()
	name, _, _ = strings.Cut(name, ".")
	opts.progress("Joining enowX's tailnet")
	// Signing in waits for the user in a browser.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	tn, err := owntailnet.Join(ctx, owntailnet.Options{
		Dir: filepath.Join(dir, "tsnet"), Hostname: "enowx-" + strings.ToLower(name), Login: opts.TailnetLogin,
		ControlURL: os.Getenv(ControlEnv),
	})
	if err != nil {
		return nil, err
	}
	own.tn = tn
	return tn, nil
}
