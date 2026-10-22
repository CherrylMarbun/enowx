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
	out, err := exec.CommandContext(ctx, cli, "status", "--json").Output()
	if err != nil {
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

// findCLI returns the tailscale CLI: on the PATH, or inside the macOS app.
func findCLI() string {
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	if runtime.GOOS == "darwin" {
		const app = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"
		if _, err := os.Stat(app); err == nil {
			return app
		}
	}
	return ""
}
