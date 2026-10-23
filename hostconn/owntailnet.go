package hostconn

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/transport"
)

// own is enowX's own tailnet device, joined the first time a host
// asks for it and kept for the app's life, since every host shares it.
var own struct {
	mu sync.Mutex
	tn *transport.Tailnet
}

// ownTailnet joins enowX's tailnet device, or returns it if joined. Its
// state lives in ~/.enowx/tsnet, so it signs in once.
func ownTailnet(opts Options) (*transport.Tailnet, error) {
	own.mu.Lock()
	defer own.mu.Unlock()
	if own.tn != nil {
		return own.tn, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	name, _ := os.Hostname()
	name, _, _ = strings.Cut(name, ".")
	opts.progress("Joining enowX's tailnet")
	// Signing in waits for the user in a browser.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	tn, err := transport.JoinTailnet(ctx, transport.TailnetOptions{
		Dir: filepath.Join(home, ".enowx", "tsnet"), Hostname: "enowx-" + strings.ToLower(name), Login: opts.TailnetLogin,
	})
	if err != nil {
		return nil, err
	}
	own.tn = tn
	return tn, nil
}
