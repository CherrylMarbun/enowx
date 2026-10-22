package envsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/enowx/enowx/proto"
)

// Codex keeps MCP servers as [mcp_servers.<name>] tables of config.toml.
// They are copied as text, table by table, so the rest of the file and its
// comments stay as they are.

func scanMCPTOML(home string, s source) []proto.EnvItem {
	rel := s.location(home)
	text, err := os.ReadFile(joinHome(home, rel))
	if err != nil {
		return nil
	}
	servers, err := tomlServers(string(text))
	if err != nil {
		return nil
	}
	var items []proto.EnvItem
	for _, name := range sortedKeys(servers) {
		if !validName(name) {
			continue
		}
		raw, _ := json.Marshal(servers[name])
		it := mcpItem(home, s, name, raw)
		if len(tomlBlocks(string(text), name)) == 0 {
			it.Warnings = append(it.Warnings, "set inline in "+tilde(rel)+"; move it to a [mcp_servers."+name+"] table to sync it")
		}
		items = append(items, it)
	}
	return items
}

func tomlServers(text string) (map[string]map[string]any, error) {
	var cfg struct {
		MCPServers map[string]map[string]any `toml:"mcp_servers"`
	}
	if _, err := toml.Decode(text, &cfg); err != nil {
		return nil, err
	}
	return cfg.MCPServers, nil
}

func exportMCPTOML(home string, s source, name string) ([]byte, error) {
	rel := s.location(home)
	text, err := os.ReadFile(joinHome(home, rel))
	if err != nil {
		return nil, err
	}
	blocks := tomlBlocks(string(text), name)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("envsync: MCP server %s is set inline in %s; move it to a [mcp_servers.%s] table", name, tilde(rel), name)
	}
	var b strings.Builder
	for _, bl := range blocks {
		b.WriteString(strings.TrimRight(bl.text, "\n"))
		b.WriteString("\n\n")
	}
	return []byte(strings.TrimRight(b.String(), "\n") + "\n"), nil
}

func importMCPTOML(home string, s source, name string, p proto.EnvPayload) (proto.EnvImportResult, error) {
	var res proto.EnvImportResult
	value := rewriteHome(string(p.Value), p.Home, home)
	incoming, err := tomlServers(value)
	if err != nil || len(incoming) != 1 || incoming[name] == nil {
		return res, fmt.Errorf("envsync: MCP server %s: not a single [mcp_servers.%s] table", name, name)
	}
	rel := s.location(home)
	file := joinHome(home, rel)
	old, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return res, err
	}
	text := string(old)
	if servers, err := tomlServers(text); err != nil {
		return res, fmt.Errorf("envsync: %s: %w", tilde(rel), err)
	} else if servers[name] != nil && len(tomlBlocks(text, name)) == 0 {
		return res, fmt.Errorf("envsync: MCP server %s is set inline in the server's %s; remove it there first", name, tilde(rel))
	}
	text = removeTOMLBlocks(text, name)
	if text != "" && !strings.HasSuffix(text, "\n\n") {
		text = strings.TrimRight(text, "\n") + "\n\n"
	}
	text += value
	got, err := tomlServers(text)
	if err != nil || got[name] == nil {
		return res, fmt.Errorf("envsync: MCP server %s: the merged %s doesn't parse: %v", name, tilde(rel), err)
	}
	if err := backup(file); err != nil {
		return res, err
	}
	if err := writeFileAtomic(file, []byte(text), 0o600); err != nil {
		return res, err
	}
	program, _ := incoming[name]["command"].(string)
	res.Warnings = append(res.Warnings, programWarnings(name, program)...)
	return res, nil
}

// tomlBlock is a table: its header line and the lines up to the next one.
type tomlBlock struct {
	start, end int // line indexes, end exclusive
	text       string
}

// tomlBlocks returns the tables of the MCP server called name: its own and
// its subtables, such as [mcp_servers.name.env].
func tomlBlocks(text, name string) []tomlBlock {
	lines := strings.SplitAfter(text, "\n")
	var out []tomlBlock
	for i := 0; i < len(lines); i++ {
		keys, ok := tomlHeader(lines[i])
		if !ok || len(keys) < 2 || keys[0] != "mcp_servers" || keys[1] != name {
			continue
		}
		j := i + 1
		for j < len(lines) {
			if _, ok := tomlHeader(lines[j]); ok {
				break
			}
			j++
		}
		out = append(out, tomlBlock{start: i, end: j, text: strings.Join(lines[i:j], "")})
		i = j - 1
	}
	return out
}

func removeTOMLBlocks(text, name string) string {
	blocks := tomlBlocks(text, name)
	if len(blocks) == 0 {
		return text
	}
	lines := strings.SplitAfter(text, "\n")
	var b strings.Builder
	next := 0
	for _, bl := range blocks {
		b.WriteString(strings.Join(lines[next:bl.start], ""))
		next = bl.end
	}
	b.WriteString(strings.Join(lines[next:], ""))
	return b.String()
}

// tomlHeader parses a table header line, [a.b] or [[a.b]], into its keys.
func tomlHeader(line string) ([]string, bool) {
	s := strings.TrimSpace(line)
	if !strings.HasPrefix(s, "[") {
		return nil, false
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "["), "[")
	var keys []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\'':
			j := i + 1
			for j < len(s) && s[j] != c {
				if c == '"' && s[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(s) {
				return nil, false
			}
			cur.WriteString(s[i+1 : j])
			i = j
		case c == '.':
			keys = append(keys, strings.TrimSpace(cur.String()))
			cur.Reset()
		case c == ']':
			keys = append(keys, strings.TrimSpace(cur.String()))
			return keys, true
		case c == ' ' || c == '\t':
		default:
			cur.WriteByte(c)
		}
	}
	return nil, false
}
