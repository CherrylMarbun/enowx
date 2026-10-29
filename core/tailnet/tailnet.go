// Package tailnet lists the other devices on the user's Tailscale network,
// through the Tailscale app's local API on macOS, else the tailscale CLI.
// Without Tailscale it finds nothing, and enowX works the same.
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

	"github.com/enowx/enowx/internal/quietcmd"
)

// ErrNotInstalled means no tailscale CLI was found.
var ErrNotInstalled = errors.New("tailnet: Tailscale is not installed")

// errAppUnreachable: the CLI inside Tailscale.app only works while the app
// runs, and enowX found no running app to ask directly.
var errAppUnreachable = errors.New("tailnet: couldn't reach the Tailscale app; make sure it is open and signed in")

// Peer is another device on the tailnet.
type Peer struct {
	HostName string // "my-vps"
	DNSName  string // "my-vps.tail1234.ts.net"
	IP       string // its Tailscale IPv4, else its first Tailscale IP
	OS       string // "linux", "macOS", …
	Online   bool
}

// Address is how SSH reaches the peer: its Tailscale IP, which works
// without MagicDNS and doesn't change, else its name.
func (p Peer) Address() string {
	if p.IP != "" {
		return p.IP
	}
	return p.DNSName
}

// Peers lists the other devices, online first, then by name.
func Peers(ctx context.Context) ([]Peer, error) {
	data, err := statusJSON(ctx)
	if err != nil {
		return nil, err
	}
	return parseStatus(data)
}

// SelfIP is this device's Tailscale IPv4, else its first Tailscale IP.
func SelfIP(ctx context.Context) (string, error) {
	data, err := statusJSON(ctx)
	if err != nil {
		return "", err
	}
	start := strings.IndexByte(string(data), '{')
	if start < 0 {
		return "", fmt.Errorf("tailnet: tailscale said %q", firstLine(strings.TrimSpace(string(data))))
	}
	var st struct {
		BackendState string
		Self         struct{ TailscaleIPs []string }
	}
	if err := json.Unmarshal(data[start:], &st); err != nil {
		return "", fmt.Errorf("tailnet: %w", err)
	}
	if st.BackendState != "" && st.BackendState != "Running" {
		return "", fmt.Errorf("tailnet: Tailscale is %s", strings.ToLower(st.BackendState))
	}
	return firstIPv4(st.Self.TailscaleIPs), nil
}

// statusJSON is `tailscale status --json`, from the app's local API or the
// CLI.
func statusJSON(ctx context.Context) ([]byte, error) {
	if data, err := localStatus(ctx); err == nil {
		return data, nil
	}
	cli := findCLI()
	if cli == "" {
		return nil, ErrNotInstalled
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := quietcmd.CommandContext(ctx, cli, "status", "--json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); strings.Contains(msg, "GUI failed to start") {
			return nil, errAppUnreachable
		} else if msg != "" {
			return nil, fmt.Errorf("tailnet: %s", firstLine(msg))
		}
		return nil, fmt.Errorf("tailnet: tailscale status: %w", err)
	}
	return out, nil
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
	// Some builds print a line (a version warning, a notice) before the JSON.
	start := strings.IndexByte(string(data), '{')
	if start < 0 && strings.Contains(string(data), "GUI failed to start") {
		return nil, errAppUnreachable
	}
	if start < 0 {
		return nil, fmt.Errorf("tailnet: tailscale said %q", firstLine(strings.TrimSpace(string(data))))
	}
	var st status
	if err := json.Unmarshal(data[start:], &st); err != nil {
		return nil, fmt.Errorf("tailnet: tailscale said %q", firstLine(strings.TrimSpace(string(data))))
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
		peer.IP = firstIPv4(p.TailscaleIPs)
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

// Resolve returns the Tailscale IP of the peer named name, a MagicDNS name
// such as "my-vps.tail1234.ts.net". Programs that resolve names themselves,
// as Go's resolver does on macOS, miss the system's tailnet DNS settings.
func Resolve(ctx context.Context, name string) (string, error) {
	peers, err := Peers(ctx)
	if err != nil {
		return "", err
	}
	name = strings.TrimSuffix(name, ".")
	for _, p := range peers {
		if p.IP != "" && strings.EqualFold(p.DNSName, name) {
			return p.IP, nil
		}
	}
	return "", fmt.Errorf("tailnet: no device named %s on your tailnet", name)
}

// IsMagicDNS reports whether host is a name on a Tailscale tailnet.
func IsMagicDNS(host string) bool {
	return strings.HasSuffix(strings.TrimSuffix(strings.ToLower(host), "."), ".ts.net")
}

// firstIPv4 is the first IPv4 of ips, else the first of them.
func firstIPv4(ips []string) string {
	for _, ip := range ips {
		if strings.Contains(ip, ".") {
			return ip
		}
	}
	if len(ips) > 0 {
		return ips[0]
	}
	return ""
}
