package envsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/enowx/enowx/proto"
)

// The Settings page keeps the agents' items on a host: it deletes them,
// and adds MCP servers and skills, written as the agent keeps them.

// Delete removes the item id under home: its files, or its entry in the
// agent's config (which is backed up first, as imports do).
func Delete(home, id string) error { return DeleteIn(home, id, User) }

// DeleteIn is Delete of an item of scope under root.
func DeleteIn(home, id string, sc Scope) error {
	s, name, err := sc.sourceOf(id)
	if err != nil {
		return err
	}
	switch s.layout {
	case layoutEntries, layoutSkills:
		p := joinHome(home, itemPath(home, s, name))
		if !exists(p) {
			return fmt.Errorf("envsync: %s: not found", id)
		}
		return os.RemoveAll(p)
	case layoutFile:
		file := joinHome(home, s.location(home))
		if err := backup(file); err != nil {
			return err
		}
		return os.Remove(file)
	case layoutMCPJSON:
		return deleteMCPJSON(home, s, name)
	case layoutMCPTOML:
		return deleteMCPTOML(home, s, name)
	}
	return fmt.Errorf("envsync: %s can't be deleted here", id)
}

func deleteMCPJSON(home string, s source, name string) error {
	rel := s.location(home)
	file := joinHome(home, rel)
	data, _, err := readJSONC(file)
	if err != nil {
		return fmt.Errorf("envsync: %s: %w", tilde(rel), err)
	}
	top, err := objectMembers(data)
	if err != nil {
		return err
	}
	servers, err := mcpMembers(data, s.key)
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(servers, func(m member) bool { return m.key == name })
	if len(kept) == len(servers) {
		return fmt.Errorf("envsync: MCP server %s: not found", name)
	}
	top = setMember(top, s.key, encodeObject(kept))
	var out bytes.Buffer
	if err := json.Indent(&out, encodeObject(top), "", "  "); err != nil {
		return err
	}
	out.WriteByte('\n')
	if err := backup(file); err != nil {
		return err
	}
	return writeFileAtomic(file, out.Bytes(), 0o600)
}

func deleteMCPTOML(home string, s source, name string) error {
	rel := s.location(home)
	file := joinHome(home, rel)
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("envsync: %s: %w", tilde(rel), err)
	}
	text := string(data)
	if len(tomlBlocks(text, name)) == 0 {
		return fmt.Errorf("envsync: MCP server %s isn't a table of %s: remove it there", name, tilde(rel))
	}
	if err := backup(file); err != nil {
		return err
	}
	return writeFileAtomic(file, []byte(removeTOMLBlocks(text, name)), 0o600)
}

// MCPServer is an MCP server to add: a program it runs, or a URL it
// serves at.
type MCPServer struct {
	Command string
	Args    []string
	URL     string
	Env     map[string]string
}

// Agents is which agents keep items of kind (proto.Env*), in sources'
// order.
func Agents(kind string) []string { return AgentsIn(kind, User) }

// AgentsIn is Agents for scope.
func AgentsIn(kind string, sc Scope) []string {
	var out []string
	for _, s := range sc.sources() {
		if s.kind == kind && !slices.Contains(out, s.agent) {
			out = append(out, s.agent)
		}
	}
	return out
}

// NewMCP is the payload that adds srv as agent's MCP server called name,
// written as that agent keeps them; Import writes it under a home.
func NewMCP(agent, name string, srv MCPServer) (proto.EnvPayload, error) {
	return NewMCPIn(agent, name, srv, User)
}

// NewMCPIn is NewMCP for scope, whose config may be another file.
func NewMCPIn(agent, name string, srv MCPServer, sc Scope) (proto.EnvPayload, error) {
	if !validName(name) {
		return proto.EnvPayload{}, fmt.Errorf("envsync: %q isn't a name for an MCP server", name)
	}
	if (srv.Command == "") == (srv.URL == "") {
		return proto.EnvPayload{}, errors.New("envsync: an MCP server runs a command or serves at a URL")
	}
	id := agent + "/" + proto.EnvMCP + "/" + name
	s, _, err := sc.sourceOf(id)
	if err != nil {
		return proto.EnvPayload{}, err
	}
	var value []byte
	switch {
	case s.layout == layoutMCPTOML:
		value = []byte(mcpTOML(name, srv))
	case agent == "opencode":
		server := map[string]any{"enabled": true}
		if srv.URL != "" {
			server["type"], server["url"] = "remote", srv.URL
		} else {
			server["type"], server["command"] = "local", append([]string{srv.Command}, srv.Args...)
		}
		if len(srv.Env) > 0 {
			server["environment"] = srv.Env
		}
		value, err = json.Marshal(server)
	default:
		server := map[string]any{}
		if srv.URL != "" {
			server["type"], server["url"] = "http", srv.URL
		} else {
			server["command"], server["args"] = srv.Command, append([]string{}, srv.Args...)
		}
		if len(srv.Env) > 0 {
			server["env"] = srv.Env
		}
		value, err = json.Marshal(server)
	}
	if err != nil {
		return proto.EnvPayload{}, err
	}
	return proto.EnvPayload{Item: proto.EnvItem{ID: id, Agent: agent, Kind: proto.EnvMCP, Name: name}, Value: value}, nil
}

// mcpTOML writes srv as Codex's [mcp_servers.<name>] table.
func mcpTOML(name string, srv MCPServer) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[mcp_servers.%s]\n", name)
	if srv.URL != "" {
		fmt.Fprintf(&b, "url = %s\n", strconv.Quote(srv.URL))
	} else {
		fmt.Fprintf(&b, "command = %s\n", strconv.Quote(srv.Command))
		args := make([]string, len(srv.Args))
		for i, a := range srv.Args {
			args[i] = strconv.Quote(a)
		}
		fmt.Fprintf(&b, "args = [%s]\n", strings.Join(args, ", "))
	}
	if len(srv.Env) > 0 {
		fmt.Fprintf(&b, "\n[mcp_servers.%s.env]\n", name)
		for _, k := range sortedKeys(srv.Env) {
			fmt.Fprintf(&b, "%s = %s\n", k, strconv.Quote(srv.Env[k]))
		}
	}
	return b.String()
}

// NewSkill is the payload that adds a skill called name to agent, its files
// given by their paths in the skill's folder (SKILL.md first, say).
func NewSkill(agent, name string, files []proto.EnvFile) (proto.EnvPayload, error) {
	return NewSkillIn(agent, name, files, User)
}

// NewSkillIn is NewSkill for scope.
func NewSkillIn(agent, name string, files []proto.EnvFile, sc Scope) (proto.EnvPayload, error) {
	if !validName(name) {
		return proto.EnvPayload{}, fmt.Errorf("envsync: %q isn't a name for a skill", name)
	}
	id := agent + "/" + proto.EnvSkill + "/" + name
	s, _, err := sc.sourceOf(id)
	if err != nil {
		return proto.EnvPayload{}, err
	}
	if !slices.ContainsFunc(files, func(f proto.EnvFile) bool { return f.Path == "SKILL.md" }) {
		return proto.EnvPayload{}, errors.New("envsync: a skill's folder holds a SKILL.md")
	}
	root := path.Join(s.paths[0], name)
	out := make([]proto.EnvFile, len(files))
	for i, f := range files {
		clean := path.Clean(f.Path)
		if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return proto.EnvPayload{}, fmt.Errorf("envsync: %s leaves the skill", f.Path)
		}
		f.Path = path.Join(root, clean)
		out[i] = f
	}
	return proto.EnvPayload{Item: proto.EnvItem{ID: id, Agent: agent, Kind: proto.EnvSkill, Name: name}, Files: out}, nil
}
