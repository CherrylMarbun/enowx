// Package envsync copies the agents' environment between homes: skills,
// subagents, slash commands, memory files, settings and MCP servers, plus
// the git identity. Each item is scanned, exported and imported on its own,
// so the daemons of two hosts can sync them through the app.
//
// Agent logins and the keys agents store for themselves are never synced.
package envsync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// layout says how a source stores its items.
type layout int

const (
	layoutEntries layout = iota // each file or folder in a folder is an item
	layoutSkills                // each folder holding a SKILL.md is an item
	layoutFile                  // one file is the item
	layoutMCPJSON               // each server under a key of a JSON file
	layoutMCPTOML               // each [mcp_servers.<name>] table of a TOML file
	layoutGit                   // git's global user.name and user.email
)

// source is a place under the home folder where an agent keeps items. Where
// each one comes from is in docs/sync.md.
type source struct {
	agent, kind string
	layout      layout
	// paths are slash-separated under the home folder; the first one that
	// exists is used, and imports write to it, else to paths[0].
	paths []string
	key   string // the JSON key holding MCP servers
}

var sources = []source{
	{agent: "claude", kind: proto.EnvSkill, layout: layoutSkills, paths: []string{".claude/skills"}},
	{agent: "claude", kind: proto.EnvSubagent, layout: layoutEntries, paths: []string{".claude/agents"}},
	{agent: "claude", kind: proto.EnvCommand, layout: layoutEntries, paths: []string{".claude/commands"}},
	{agent: "claude", kind: proto.EnvMemory, layout: layoutFile, paths: []string{".claude/CLAUDE.md"}},
	{agent: "claude", kind: proto.EnvSettings, layout: layoutFile, paths: []string{".claude/settings.json"}},
	{agent: "claude", kind: proto.EnvMCP, layout: layoutMCPJSON, paths: []string{".claude.json"}, key: "mcpServers"},

	{agent: "codex", kind: proto.EnvMemory, layout: layoutFile, paths: []string{".codex/AGENTS.md"}},
	{agent: "codex", kind: proto.EnvSubagent, layout: layoutEntries, paths: []string{".codex/agents"}},
	{agent: "codex", kind: proto.EnvCommand, layout: layoutEntries, paths: []string{".codex/prompts"}},
	{agent: "codex", kind: proto.EnvMCP, layout: layoutMCPTOML, paths: []string{".codex/config.toml"}},

	// Read by Codex and OpenCode alike.
	{agent: "shared", kind: proto.EnvSkill, layout: layoutSkills, paths: []string{".agents/skills"}},

	{agent: "opencode", kind: proto.EnvMemory, layout: layoutFile, paths: []string{".config/opencode/AGENTS.md"}},
	{agent: "opencode", kind: proto.EnvSkill, layout: layoutSkills, paths: []string{".config/opencode/skills"}},
	{agent: "opencode", kind: proto.EnvSubagent, layout: layoutEntries, paths: []string{".config/opencode/agents"}},
	{agent: "opencode", kind: proto.EnvCommand, layout: layoutEntries, paths: []string{".config/opencode/commands"}},
	{agent: "opencode", kind: proto.EnvMCP, layout: layoutMCPJSON, paths: []string{".config/opencode/opencode.json", ".config/opencode/opencode.jsonc"}, key: "mcp"},

	{agent: "gemini", kind: proto.EnvMemory, layout: layoutFile, paths: []string{".gemini/GEMINI.md"}},
	{agent: "gemini", kind: proto.EnvCommand, layout: layoutEntries, paths: []string{".gemini/commands"}},
	{agent: "gemini", kind: proto.EnvMCP, layout: layoutMCPJSON, paths: []string{".gemini/settings.json"}, key: "mcpServers"},

	{agent: "shared", kind: proto.EnvGit, layout: layoutGit},
}

// Limits keep an item within one protocol frame.
const (
	maxFileBytes = 8 << 20
	maxItemBytes = 12 << 20
)

// prefix starts the IDs of the source's items.
func (s source) prefix() string { return s.agent + "/" + s.kind + "/" }

// Scan lists the items under home.
func Scan(home string) proto.EnvScan {
	out := proto.EnvScan{Home: home, Items: []proto.EnvItem{}}
	for _, s := range sources {
		out.Items = append(out.Items, s.scan(home)...)
	}
	return out
}

func (s source) scan(home string) []proto.EnvItem {
	switch s.layout {
	case layoutEntries, layoutSkills:
		return scanEntries(home, s)
	case layoutFile:
		return scanFile(home, s)
	case layoutMCPJSON:
		return scanMCPJSON(home, s)
	case layoutMCPTOML:
		return scanMCPTOML(home, s)
	case layoutGit:
		return scanGit(s)
	}
	return nil
}

// Export returns the content of the item id under home.
func Export(home, id string) (proto.EnvPayload, error) {
	s, name, err := sourceOf(id)
	if err != nil {
		return proto.EnvPayload{}, err
	}
	var item proto.EnvItem
	found := false
	for _, it := range s.scan(home) {
		if it.ID == id {
			item, found = it, true
			break
		}
	}
	if !found {
		return proto.EnvPayload{}, fmt.Errorf("envsync: %s: not found", id)
	}
	p := proto.EnvPayload{Item: item, Home: home}
	switch s.layout {
	case layoutEntries, layoutSkills, layoutFile:
		p.Files, err = exportFiles(home, s, name)
	case layoutMCPJSON:
		p.Value, err = exportMCPJSON(home, s, name)
	case layoutMCPTOML:
		p.Value, err = exportMCPTOML(home, s, name)
	case layoutGit:
		p.Value, err = exportGit()
	}
	return p, err
}

// Import writes p under home, replacing the item if it is there. Paths
// under the exporting home are moved to this one.
func Import(home string, p proto.EnvPayload) (proto.EnvImportResult, error) {
	s, name, err := sourceOf(p.Item.ID)
	if err != nil {
		return proto.EnvImportResult{}, err
	}
	switch s.layout {
	case layoutEntries, layoutSkills, layoutFile:
		return importFiles(home, s, name, p)
	case layoutMCPJSON:
		return importMCPJSON(home, s, name, p)
	case layoutMCPTOML:
		return importMCPTOML(home, s, name, p)
	case layoutGit:
		return importGit(p)
	}
	return proto.EnvImportResult{}, errors.New("envsync: unknown item")
}

// sourceOf finds the source of an item ID and the item's name in it.
func sourceOf(id string) (source, string, error) {
	for _, s := range sources {
		if name, ok := strings.CutPrefix(id, s.prefix()); ok && validName(name) {
			return s, name, nil
		}
	}
	return source{}, "", fmt.Errorf("envsync: unknown item %q", id)
}

// validName rejects names that would leave the source's folder.
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}

// location returns the source's path under home that exists, else the
// first one.
func (s source) location(home string) string {
	for _, p := range s.paths {
		if exists(joinHome(home, p)) {
			return p
		}
	}
	return s.paths[0]
}

func newItem(s source, name, detail string) proto.EnvItem {
	return proto.EnvItem{ID: s.prefix() + name, Agent: s.agent, Kind: s.kind, Name: name, Detail: detail}
}

// hashParts hashes the parts in order.
func hashParts(parts ...[]byte) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:", len(p))
		h.Write(p)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// homeToken stands for the home folder in hashes, so that an item hashes
// the same in homes at different paths.
const homeToken = "\x00HOME\x00"

// rewriteHome replaces the folder from with to in s where it is a whole
// path: "/Users/me/x" moves, "/Users/meg" doesn't.
func rewriteHome(s, from, to string) string {
	if from == "" || from == to {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(s, from)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(from)
		if end < len(s) && isNameByte(s[end]) {
			b.WriteString(s[:end])
			s = s[end:]
			continue
		}
		b.WriteString(s[:i])
		b.WriteString(to)
		s = s[end:]
	}
}

func isNameByte(c byte) bool {
	return c == '_' || c == '-' || c == '.' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// tilde writes a path under the home folder with ~.
func tilde(rel string) string { return path.Join("~", rel) }

func sortedKeys[M ~map[string]V, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
