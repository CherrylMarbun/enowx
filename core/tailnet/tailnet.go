// Package tailnet lists the other devices on the user's Tailscale network,
// through the tailscale CLI installed on this machine. Without Tailscale it
// finds nothing, and enowX works the same.
package tailnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"
)

// ErrNotInstalled means no tailscale CLI was found.
var ErrNotInstalled = errors.New("tailnet: Tailscale is not installed")

// Peer is another device on the tailnet.
type Peer struct {
	HostName string // "my-vps"
	DNSName  string // "my-vps.tail1234.ts.net", what SSH connects to
	IP       string // first Tailscale IP
	OS       string // "linux", "macOS", …
	Online   bool
}

// Address is how SSH reaches the peer: its MagicDNS name, else its IP.
func (p Peer) Address() string {
	if p.DNSName != "" {
		return p.DNSName
	}
	return p.IP
}

// Peers lists the other devices, online first, then by name.
func Peers(ctx context.Context) ([]Peer, error) {
	cli := findCLI()
	if cli == "" {
		return nil, ErrNotInstalled
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, "status", "--json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("tailnet: %s", firstLine(msg))
		}
		return nil, fmt.Errorf("tailnet: tailscale status: %w", err)
	}
	return parseStatus(out)
}

// status is the part of `tailscale status --json` enowX reads.
type status struct {
	BackendState string
	Peer         map[string]struct {
		HostName     string
		DNSName      string
		OS           string
		TailscaleIPs []string
		Online       bool
	}
}

func parseStatus(data []byte) ([]Peer, error) {
	var st status
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("tailnet: read status: %w", err)
	}
	if st.BackendState != "" && st.BackendState != "Running" {
		return nil, fmt.Errorf("tailnet: Tailscale is %s", strings.ToLower(st.BackendState))
	}
	peers := make([]Peer, 0, len(st.Peer))
	for _, p := range st.Peer {
		peer := Peer{
			HostName: p.HostName,
			DNSName:  strings.TrimSuffix(p.DNSName, "."),
			OS:       p.OS,
			Online:   p.Online,
		}
		if len(p.TailscaleIPs) > 0 {
			peer.IP = p.TailscaleIPs[0]
		}
		if peer.HostName == "" {
			peer.HostName, _, _ = strings.Cut(peer.DNSName, ".")
		}
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Online != peers[j].Online {
			return peers[i].Online
		}
		return strings.ToLower(peers[i].HostName) < strings.ToLower(peers[j].HostName)
	})
	return peers, nil
}

// findCLI returns the tailscale CLI: on the PATH, else where installers put
// it. Apps opened from the Finder get a short PATH without Homebrew's or
// /usr/local's directories, so those are looked at by hand.
func findCLI() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Tailscale.app/Contents/MacOS/Tailscale", // App Store and standalone apps
			"/opt/homebrew/bin/tailscale",                          // Homebrew, Apple silicon
			"/usr/local/bin/tailscale",                             // Homebrew on Intel, the app's CLI link
		}
	case "windows":
		candidates = []string{`C:\Program Files\Tailscale\tailscale.exe`}
	default:
		candidates = []string{"/usr/bin/tailscale", "/usr/local/bin/tailscale", "/usr/sbin/tailscale"}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
