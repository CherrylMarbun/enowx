// Package vps creates servers at cloud providers, with the user's own API
// token, for enowX to set up: Hetzner Cloud, DigitalOcean and Vultr. Each
// gets an Ubuntu 24.04 server with the user's SSH key on it; enowX
// operates no service in between. See docs/vps.md.
package vps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider is a cloud provider enowX creates servers at.
type Provider struct {
	ID, Name string
	// TokenHelp says where the user makes an API token that can create
	// servers.
	TokenHelp string
	base      string
}

// Providers are the providers, in the order the app lists them.
var Providers = []Provider{
	{ID: "hetzner", Name: "Hetzner Cloud", TokenHelp: "Hetzner Console → your project → Security → API tokens: a Read & Write token", base: "https://api.hetzner.cloud/v1"},
	{ID: "digitalocean", Name: "DigitalOcean", TokenHelp: "DigitalOcean → API → Personal access tokens: a token with write access", base: "https://api.digitalocean.com/v2"},
	{ID: "vultr", Name: "Vultr", TokenHelp: "Vultr → Account → API: enable the API and copy its key", base: "https://api.vultr.com/v2"},
}

// Find returns the provider with the given ID.
func Find(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// Option is a region or a size to pick, with what to show for it.
type Option struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	// Monthly is a size's price a month, in US dollars (Hetzner: euros),
	// 0 when the provider doesn't say.
	Monthly float64 `json:"monthly,omitempty"`
}

// Server is a server as the provider reports it.
type Server struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // the provider's own word
	IPv4   string `json:"ipv4,omitempty"`
	// Ready says it runs, with an address to connect to.
	Ready bool `json:"ready"`
}

// CreateParams are a server to create.
type CreateParams struct {
	Name   string
	Region string
	Size   string
	// PublicKey is the user's SSH public key, put on the server for root.
	PublicKey string
}

// Client talks to one provider with one token.
type Client interface {
	Regions(ctx context.Context) ([]Option, error)
	// Sizes are the sizes a server can have in region.
	Sizes(ctx context.Context, region string) ([]Option, error)
	Create(ctx context.Context, p CreateParams) (Server, error)
	Server(ctx context.Context, id string) (Server, error)
	Delete(ctx context.Context, id string) error
}

// New returns a client of provider id with the user's token. base, when
// set, replaces the provider's API address (for tests).
func New(id, token, base string) (Client, error) {
	p, ok := Find(id)
	if !ok {
		return nil, fmt.Errorf("vps: unknown provider %q", id)
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("vps: the API token is empty")
	}
	if base == "" {
		base = p.base
	}
	api := &api{base: strings.TrimSuffix(base, "/"), token: strings.TrimSpace(token), name: p.Name, http: &http.Client{Timeout: 30 * time.Second}}
	switch id {
	case "hetzner":
		return hetzner{api}, nil
	case "digitalocean":
		return digitalOcean{api}, nil
	}
	return vultr{api}, nil
}

// api is a provider's JSON API, with the token as a bearer.
type api struct {
	base, token, name string
	http              *http.Client
}

// do calls method path with in as the JSON body (nil for none) and
// decodes the answer into out (nil to drop it).
func (a *api) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("vps: %s: %w", a.name, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("vps: %s: %w", a.name, err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("vps: %s refused the token (%s): check that it can create servers", a.name, resp.Status)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("vps: %s: %s %s: %s", a.name, method, path, apiError(resp.Status, data))
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("vps: %s: %s %s: %w", a.name, method, path, err)
	}
	return nil
}

// apiError is the message of a provider's error answer, else its status.
func apiError(status string, data []byte) string {
	var e struct {
		Error any    `json:"error"`   // Hetzner: {"message"}; Vultr: a string
		Text  string `json:"message"` // DigitalOcean
	}
	if json.Unmarshal(data, &e) == nil {
		switch v := e.Error.(type) {
		case string:
			return v
		case map[string]any:
			if m, ok := v["message"].(string); ok {
				return m
			}
		}
		if e.Text != "" {
			return e.Text
		}
	}
	return status
}

// keyBody is a public key's type and data, without its comment, for
// comparing keys a provider holds with the user's.
func keyBody(key string) string {
	f := strings.Fields(key)
	if len(f) < 2 {
		return strings.TrimSpace(key)
	}
	return f[0] + " " + f[1]
}

// keyName names the user's key at a provider.
const keyName = "enowX"

var timeNow = time.Now

// WaitReady polls server id until it runs with an address, every poll,
// telling each status to progress (nil for none).
func WaitReady(ctx context.Context, c Client, id string, poll time.Duration, progress func(Server)) (Server, error) {
	for {
		s, err := c.Server(ctx, id)
		if err != nil {
			return s, err
		}
		if progress != nil {
			progress(s)
		}
		if s.Ready {
			return s, nil
		}
		select {
		case <-ctx.Done():
			return s, fmt.Errorf("vps: server %s isn't up yet (%s): %w", id, s.Status, ctx.Err())
		case <-time.After(poll):
		}
	}
}
