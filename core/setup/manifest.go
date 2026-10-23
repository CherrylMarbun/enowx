package setup

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/BurntSushi/toml"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

// Manifest is what to bring to a host, as enowx.toml: made from a scan of
// this device, edited by the user, kept and used for other hosts.
type Manifest struct {
	// Tools are the toolchains, as mise names them, at their version:
	// node = "22.11.0".
	Tools map[string]string `toml:"tools"`
	// Git is git's global user.
	Git struct {
		Name  string `toml:"name"`
		Email string `toml:"email"`
	} `toml:"git"`
	// GitHub installs gh and signs it in (a code to enter in a browser):
	// no token leaves this device.
	GitHub bool `toml:"github"`
	// Agents are the agent CLIs to install, by preset ID; SignIn those to
	// sign in there too.
	Agents []string `toml:"agents"`
	SignIn []string `toml:"sign_in"`
	// Display installs what sessions need to run desktop apps (Xpra).
	Display bool `toml:"display"`
}

// miseNames maps the programs a scan finds to the tools mise installs.
var miseNames = []struct{ program, tool string }{
	{"node", "node"}, {"bun", "bun"}, {"deno", "deno"}, {"pnpm", "pnpm"},
	{"python3", "python"}, {"go", "go"}, {"rustc", "rust"}, {"java", "java"},
}

// FromScan is the manifest that brings a host to what s found: its
// toolchains at their versions, its git user, gh when it is signed in,
// its agents.
func FromScan(s proto.SetupScan) Manifest {
	m := Manifest{Tools: map[string]string{}, GitHub: s.GitHub}
	for _, n := range miseNames {
		if v, ok := s.Tools[n.program]; ok && v != "" {
			m.Tools[n.tool] = v
		}
	}
	m.Git.Name, m.Git.Email = s.GitName, s.GitEmail
	for _, p := range agent.Presets {
		if _, ok := s.Tools[p.Command]; ok {
			m.Agents = append(m.Agents, p.ID)
		}
	}
	_, m.Display = s.Tools["xpra"]
	return m
}

// Encode writes m as enowx.toml.
func (m Manifest) Encode() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("# What enowX brings to a host: see docs/setup.md.\n")
	if err := toml.NewEncoder(&b).Encode(m); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Decode reads an enowx.toml.
func Decode(data []byte) (Manifest, error) {
	var m Manifest
	if _, err := toml.Decode(string(data), &m); err != nil {
		return Manifest{}, fmt.Errorf("setup: enowx.toml: %w", err)
	}
	if m.Tools == nil {
		m.Tools = map[string]string{}
	}
	for _, id := range append(slices.Clone(m.Agents), m.SignIn...) {
		if _, ok := agent.Find(id); !ok {
			return Manifest{}, fmt.Errorf("setup: enowx.toml: no agent %q", id)
		}
	}
	return m, nil
}
