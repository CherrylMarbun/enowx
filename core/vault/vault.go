// Package vault keeps the user's logins to platforms (GitHub, cloud
// providers, databases, AI APIs, SSH keys…) encrypted, and says what a
// session may have of them: the variables a credential sets, the login
// git or docker gets, whether a host may use it without asking
// (docs/vault.md). It never writes a secret anywhere but the sealed file.
package vault

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// Credential types.
const (
	TypeAPIKey   = "api-key"
	TypeLogin    = "login" // username and password
	TypeToken    = "oauth" // an OAuth token, maybe with a refresh token
	TypeEnvFile  = "env-file"
	TypeSSHKey   = "ssh-key"
	TypeCert     = "certificate"
	TypeKeyPair  = "key-pair" // an ID and a secret, as AWS's
	TypeURL      = "url"      // a connection string
	TypeFileJSON = "json-file"
)

// Vault is the credentials, as sealed in a File.
type Vault struct {
	Credentials []Credential `json:"credentials"`
}

// Credential is one login. Fields hold its values by field key (the
// platform's, see Platform.Fields); Env overrides which variables they
// set.
type Credential struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Platform string            `json:"platform"`
	Account  string            `json:"account,omitempty"`
	Type     string            `json:"type"`
	Fields   map[string]string `json:"fields"`
	Env      map[string]string `json:"env,omitempty"` // variable → field key
	Notes    string            `json:"notes,omitempty"`

	Created time.Time `json:"created"`
	Rotated time.Time `json:"rotated,omitzero"`
	Expires time.Time `json:"expires,omitzero"`
	Revoked bool      `json:"revoked,omitempty"`
	Status  string    `json:"status,omitempty"`
	Checked time.Time `json:"checked,omitzero"`

	Hosts     []string `json:"hosts,omitempty"`
	HostsOnly bool     `json:"hostsOnly,omitempty"`
	Projects  []string `json:"projects,omitempty"`
	Auto      bool     `json:"auto,omitempty"`
	Default   bool     `json:"default,omitempty"`
}

// NewID returns a credential's ID.
func NewID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Find returns the credential named, or with ID, name; names compare
// without case.
func (v *Vault) Find(name string) (*Credential, bool) {
	for i := range v.Credentials {
		c := &v.Credentials[i]
		if c.ID == name || strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return nil, false
}

// Add keeps c as a new credential, given an ID and a name of its own.
func (v *Vault) Add(c Credential) (*Credential, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		c.Name = defaultName(c)
	}
	if _, taken := v.Find(c.Name); taken {
		return nil, fmt.Errorf("vault: a credential is already named %q", c.Name)
	}
	if strings.ContainsAny(c.Name, " \t\n") {
		return nil, fmt.Errorf("vault: %q: a name has no spaces, to be typed in commands", c.Name)
	}
	if len(c.Fields) == 0 {
		return nil, errors.New("vault: a credential needs a value")
	}
	if c.ID == "" {
		c.ID = NewID()
	}
	if c.Type == "" {
		c.Type = Lookup(c.Platform).Type
	}
	if c.Created.IsZero() {
		c.Created = time.Now().UTC()
	}
	if c.Status == "" {
		c.Status = proto.CredUnchecked
	}
	if !slices.ContainsFunc(v.Credentials, func(o Credential) bool { return o.Platform == c.Platform }) {
		c.Default = true // the platform's first
	}
	v.Credentials = append(v.Credentials, c)
	return &v.Credentials[len(v.Credentials)-1], nil
}

// defaultName is a credential's name when none is given: its platform,
// and its account.
func defaultName(c Credential) string {
	name := c.Platform
	if c.Account != "" {
		name += "-" + c.Account
	}
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '@' {
			return '-'
		}
		return r
	}, strings.ToLower(name))
}

// Remove forgets credential name.
func (v *Vault) Remove(name string) error {
	c, ok := v.Find(name)
	if !ok {
		return fmt.Errorf("vault: no credential %q", name)
	}
	id, platform, wasDefault := c.ID, c.Platform, c.Default
	v.Credentials = slices.DeleteFunc(v.Credentials, func(o Credential) bool { return o.ID == id })
	if wasDefault {
		for i := range v.Credentials {
			if v.Credentials[i].Platform == platform {
				v.Credentials[i].Default = true
				break
			}
		}
	}
	return nil
}

// SetDefault makes name its platform's default.
func (v *Vault) SetDefault(name string) error {
	c, ok := v.Find(name)
	if !ok {
		return fmt.Errorf("vault: no credential %q", name)
	}
	for i := range v.Credentials {
		if o := &v.Credentials[i]; o.Platform == c.Platform {
			o.Default = o.ID == c.ID
		}
	}
	return nil
}

// Rotate replaces a credential's values, as after making a new token.
func (c *Credential) Rotate(fields map[string]string) {
	c.Fields, c.Rotated, c.Status, c.Checked = fields, time.Now().UTC(), proto.CredUnchecked, time.Time{}
}

// State is the credential's status now: revoked and expired win over
// what its last check said.
func (c *Credential) State(now time.Time) string {
	switch {
	case c.Revoked:
		return proto.CredRevoked
	case !c.Expires.IsZero() && now.After(c.Expires):
		return proto.CredExpired
	case c.Status == "":
		return proto.CredUnchecked
	}
	return c.Status
}

// ExpiresSoon reports whether it expires within d of now, not yet
// expired.
func (c *Credential) ExpiresSoon(now time.Time, d time.Duration) bool {
	return !c.Revoked && !c.Expires.IsZero() && now.Before(c.Expires) && c.Expires.Sub(now) <= d
}

// Index is what the vault holds without its values, for hosts and agents.
func (v *Vault) Index(updated time.Time) proto.VaultIndex {
	out := proto.VaultIndex{Present: true, Updated: updated, Credentials: []proto.CredentialInfo{}}
	now := time.Now()
	for _, c := range v.Credentials {
		p := Lookup(c.Platform)
		info := proto.CredentialInfo{
			ID: c.ID, Name: c.Name, Platform: c.Platform, Account: c.Account, Type: c.Type,
			Hint: Hint(c.main()), Env: c.EnvNames(), Status: c.State(now), Checked: c.Checked,
			Expires: c.Expires, Revoked: c.Revoked, Hosts: c.Hosts, HostsOnly: c.HostsOnly,
			Projects: c.Projects, Auto: c.Auto, Default: c.Default,
			GitHosts: c.gitHosts(p), Registries: c.registries(p),
		}
		if c.Type == TypeSSHKey {
			info.PublicKey, info.Hint = strings.TrimSpace(c.Fields["public_key"]), ""
		}
		out.Credentials = append(out.Credentials, info)
	}
	return out
}

// main is the value a hint is made of: the platform's first secret field.
func (c *Credential) main() string {
	for _, f := range Lookup(c.Platform).Fields {
		if f.Secret && c.Fields[f.Key] != "" {
			return c.Fields[f.Key]
		}
	}
	for _, k := range sortedKeys(c.Fields) {
		return c.Fields[k]
	}
	return ""
}

// Hint is a value's last four characters, "…last4", or nothing for a
// value too short to give any away.
func Hint(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 12 {
		return ""
	}
	return "…" + s[len(s)-4:]
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
