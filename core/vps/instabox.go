package vps

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// InstaBox (api.instabox.sh): a Firecracker VM of Debian 13, reached
// through InstaBox's SSH gateway as the machine's own user, not root at an
// address of its own. The gateway takes certificates only, which its API
// signs for a public key for up to a day: enowX asks for one each time it
// connects (Certificate), with the user's API key.

// instaboxID is InstaBox's provider ID.
const instaboxID = "instabox"

type instabox struct{ *api }

// Regions is one: InstaBox doesn't let one pick.
func (b instabox) Regions(ctx context.Context) ([]Option, error) {
	if err := b.do(ctx, "GET", "/me", nil, nil); err != nil {
		return nil, err // the key, checked
	}
	return []Option{{ID: "default", Name: "InstaBox", Detail: "where InstaBox runs it"}}, nil
}

// Sizes are those the user's plan allows (GET /orgs, each org's
// allowedSpecs): the Free plan has one.
func (b instabox) Sizes(ctx context.Context, region string) ([]Option, error) {
	var orgs struct {
		Orgs []struct {
			Tier         string `json:"tier"`
			AllowedSpecs []struct {
				VCPUs  int `json:"vcpus"`
				MemMiB int `json:"memMiB"`
			} `json:"allowedSpecs"`
		} `json:"orgs"`
	}
	if err := b.do(ctx, "GET", "/orgs", nil, &orgs); err != nil {
		return nil, err
	}
	var out []Option
	seen := map[string]bool{}
	for _, o := range orgs.Orgs {
		for _, sp := range o.AllowedSpecs {
			id := fmt.Sprintf("%d/%d", sp.VCPUs, sp.MemMiB)
			if seen[id] || sp.VCPUs == 0 {
				continue
			}
			seen[id] = true
			detail := "billed by the second while it runs"
			if o.Tier == "free" {
				detail = "the Free plan's, not billed"
			}
			out = append(out, Option{ID: id, Name: instaboxSize(sp.VCPUs, sp.MemMiB), Detail: detail})
		}
	}
	return out, nil
}

func instaboxSize(vcpus, memMiB int) string {
	return fmt.Sprintf("%d vCPUs, %d GB", vcpus, memMiB/1024)
}

type instaboxMachine struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	State   string `json:"state"`
	Runtime struct {
		Ready bool `json:"ready"`
	} `json:"runtime"`
	Spec struct {
		VCPUs  int `json:"vcpus"`
		MemMiB int `json:"memMiB"`
	} `json:"spec"`
}

func (m instaboxMachine) server(ssh string) Server {
	s := Server{ID: m.ID, Name: m.Name, Status: m.State, Ready: m.State == "running" && m.Runtime.Ready, SSH: ssh}
	if m.Spec.VCPUs > 0 {
		s.Size = instaboxSize(m.Spec.VCPUs, m.Spec.MemMiB)
	}
	return s
}

// Servers are the user's machines, deleted ones aside.
func (b instabox) Servers(ctx context.Context) ([]Server, error) {
	var list struct {
		Machines []instaboxMachine `json:"machines"`
	}
	if err := b.do(ctx, "GET", "/machines", nil, &list); err != nil {
		return nil, err
	}
	out := []Server{}
	for _, m := range list.Machines {
		if m.State != "deleting" && m.State != "deleted" {
			out = append(out, m.server(""))
		}
	}
	return out, nil
}

// Create makes a machine of the default template (Docker, Node.js, Go,
// Python, a browser): p.PublicKey isn't needed, a certificate is asked for
// as enowX connects.
func (b instabox) Create(ctx context.Context, p CreateParams) (Server, error) {
	body := map[string]any{"name": instaboxName(p.Name)}
	if v, m, ok := strings.Cut(p.Size, "/"); ok {
		vcpus, err1 := strconv.Atoi(v)
		mem, err2 := strconv.Atoi(m)
		if err1 == nil && err2 == nil {
			body["vcpus"], body["memMiB"] = vcpus, mem
		}
	}
	var op struct {
		MachineID string `json:"machineId"`
	}
	if err := b.do(ctx, "POST", "/machines", body, &op); err != nil {
		return Server{}, err
	}
	if op.MachineID == "" {
		return Server{}, fmt.Errorf("vps: %s made no machine", b.name)
	}
	return Server{ID: op.MachineID, Name: p.Name, Status: "creating"}, nil
}

// instaboxName is a machine's name as InstaBox takes them: 1 to 63
// lowercase letters, digits and hyphens.
func instaboxName(name string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			sb.WriteRune(r)
		case r == ' ' || r == '_' || r == '.':
			sb.WriteRune('-')
		}
	}
	s := strings.Trim(sb.String(), "-")
	if len(s) > 63 {
		s = s[:63]
	}
	if s == "" {
		s = "enowx"
	}
	return s
}

func (b instabox) Server(ctx context.Context, id string) (Server, error) {
	var m instaboxMachine
	if err := b.do(ctx, "GET", "/machines/"+id, nil, &m); err != nil {
		return Server{}, err
	}
	s := m.server("")
	// Where its gateway takes it, known as soon as it is made, paused or
	// not.
	var info struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
	}
	if err := b.do(ctx, "GET", "/machines/"+id+"/connection-info", nil, &info); err == nil && info.Host != "" {
		s.SSH = info.Username + "@" + net.JoinHostPort(info.Host, strconv.Itoa(info.Port))
	} else if s.Ready {
		return Server{}, err
	}
	return s, nil
}

// wake resumes machine id when paused, starts it when stopped, and waits
// until it runs: InstaBox wakes a machine it paused itself as one signs
// in, but not one paused or stopped by hand.
func (b instabox) wake(ctx context.Context, id string) error {
	for {
		var m instaboxMachine
		if err := b.do(ctx, "GET", "/machines/"+id, nil, &m); err != nil {
			return err
		}
		switch m.State {
		case "running":
			if m.Runtime.Ready {
				return nil
			}
		case "suspended":
			if err := b.do(ctx, "POST", "/machines/"+id+"/resume", nil, nil); err != nil {
				return err
			}
		case "stopped":
			if err := b.do(ctx, "POST", "/machines/"+id+"/start", nil, nil); err != nil {
				return err
			}
		case "error", "deleting", "deleted":
			return fmt.Errorf("vps: %s's machine %s is %s", b.name, id, m.State)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("vps: %s's machine %s is still %s: %w", b.name, id, m.State, ctx.Err())
		case <-time.After(wakePoll):
		}
	}
}

// wakePoll is how often waking a machine looks again.
var wakePoll = 2 * time.Second

func (b instabox) Delete(ctx context.Context, id string) error {
	return b.do(ctx, "DELETE", "/machines/"+id, nil, nil)
}

// SSHCertificate is a certificate InstaBox signed for a public key, to
// sign in to a machine through its gateway, and the authority the
// gateway's host keys are signed by. Both are in authorized_keys form.
type SSHCertificate struct {
	Certificate   string
	HostAuthority string
}

// Certificate asks provider's API, with token, for a certificate signing
// publicKey in to machine id, for an hour; base replaces the API's
// address, for tests. Only InstaBox's machines need one.
func Certificate(ctx context.Context, provider, token, base, id, publicKey string) (SSHCertificate, error) {
	c, err := New(provider, token, base)
	if err != nil {
		return SSHCertificate{}, err
	}
	b, ok := c.(instabox)
	if !ok {
		return SSHCertificate{}, fmt.Errorf("vps: %s's servers take keys, not certificates", provider)
	}
	if err := b.wake(ctx, id); err != nil {
		return SSHCertificate{}, err
	}
	var cn struct {
		Certificate string `json:"certificate"`
		HostTrust   struct {
			PublicKey string `json:"publicKey"`
		} `json:"hostTrust"`
	}
	if err := b.do(ctx, "POST", "/machines/"+id+"/connections", map[string]any{"publicKey": strings.TrimSpace(publicKey), "ttlSeconds": 3600}, &cn); err != nil {
		return SSHCertificate{}, err
	}
	if cn.Certificate == "" || cn.HostTrust.PublicKey == "" {
		return SSHCertificate{}, fmt.Errorf("vps: %s gave no certificate", b.name)
	}
	return SSHCertificate{Certificate: cn.Certificate, HostAuthority: cn.HostTrust.PublicKey}, nil
}

// NeedsCertificate says provider's servers are signed in to with a
// certificate its API gives (Certificate), not a key they hold.
func NeedsCertificate(provider string) bool { return provider == instaboxID }

// idempotencyKey names one request to InstaBox, which makes POST and
// DELETE take one: a retry with it isn't done twice.
func idempotencyKey() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
