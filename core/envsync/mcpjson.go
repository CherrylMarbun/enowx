package envsync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/enowx/enowx/proto"
)

func scanMCPJSON(home string, s source) []proto.EnvItem {
	rel := s.location(home)
	data, _, err := readJSONC(joinHome(home, rel))
	if err != nil {
		return nil
	}
	servers, err := mcpMembers(data, s.key)
	if err != nil {
		return nil
	}
	var items []proto.EnvItem
	for _, m := range servers {
		if !validName(m.key) {
			continue
		}
		items = append(items, mcpItem(home, s, m.key, m.value))
	}
	return items
}

// mcpItem describes the MCP server called name, configured as raw.
func mcpItem(home string, s source, name string, raw []byte) proto.EnvItem {
	var server map[string]any
	_ = json.Unmarshal(raw, &server)
	it := newItem(s, name, mcpDetail(server))
	it.Secrets = mcpSecrets(server)
	canonical, _ := json.Marshal(server) // sorted keys
	it.Hash = hashParts([]byte(rewriteHome(string(canonical), home, homeToken)))
	return it
}

// mcpDetail is what the server runs, or where it is.
func mcpDetail(server map[string]any) string {
	if u, ok := server["url"].(string); ok {
		return u
	}
	var words []string
	switch c := server["command"].(type) {
	case string:
		words = append(words, c)
	case []any: // OpenCode: the command with its arguments
		for _, w := range c {
			if s, ok := w.(string); ok {
				words = append(words, s)
			}
		}
	}
	if args, ok := server["args"].([]any); ok {
		for _, a := range args {
			if s, ok := a.(string); ok {
				words = append(words, s)
			}
		}
	}
	return strings.Join(words, " ")
}

// mcpProgram is the program an MCP server runs, "" for a remote one.
func mcpProgram(server map[string]any) string {
	switch c := server["command"].(type) {
	case string:
		return c
	case []any:
		if len(c) > 0 {
			s, _ := c[0].(string)
			return s
		}
	}
	return ""
}

func exportMCPJSON(home string, s source, name string) ([]byte, error) {
	data, _, err := readJSONC(joinHome(home, s.location(home)))
	if err != nil {
		return nil, err
	}
	servers, err := mcpMembers(data, s.key)
	if err != nil {
		return nil, err
	}
	for _, m := range servers {
		if m.key == name {
			var buf bytes.Buffer
			if err := json.Compact(&buf, m.value); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("envsync: MCP server %s not found", name)
}

// importMCPJSON sets the server in the config file, keeping everything
// else in it as it was. The file before the change is kept beside it.
func importMCPJSON(home string, s source, name string, p proto.EnvPayload) (proto.EnvImportResult, error) {
	var res proto.EnvImportResult
	value := []byte(rewriteHome(string(p.Value), p.Home, home))
	var server map[string]any
	if err := json.Unmarshal(value, &server); err != nil || server == nil {
		return res, fmt.Errorf("envsync: MCP server %s: not a JSON object", name)
	}
	rel := s.location(home)
	file := joinHome(home, rel)
	data, hadComments, err := readJSONC(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		data = []byte("{}")
	case err != nil:
		return res, fmt.Errorf("envsync: %s: %w", tilde(rel), err)
	}
	top, err := objectMembers(data)
	if err != nil {
		return res, fmt.Errorf("envsync: %s: %w", tilde(rel), err)
	}
	servers, err := mcpMembers(data, s.key)
	if err != nil {
		return res, fmt.Errorf("envsync: %s: %w", tilde(rel), err)
	}
	servers = setMember(servers, name, value)
	top = setMember(top, s.key, encodeObject(servers))
	var out bytes.Buffer
	if err := json.Indent(&out, encodeObject(top), "", "  "); err != nil {
		return res, err
	}
	out.WriteByte('\n')
	if err := backup(file); err != nil {
		return res, err
	}
	if err := writeFileAtomic(file, out.Bytes(), 0o600); err != nil {
		return res, err
	}
	if hadComments {
		res.Warnings = append(res.Warnings, fmt.Sprintf("%s had comments, which were dropped; the old file is %s.enowx.bak", tilde(rel), tilde(rel)))
	}
	res.Warnings = append(res.Warnings, programWarnings(name, mcpProgram(server))...)
	return res, nil
}

// backup copies file to file.enowx.bak before enowX changes it.
func backup(file string) error {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(file+".enowx.bak", data, 0o600)
}

// member is a key and its raw value, in an object's order.
type member struct {
	key   string
	value json.RawMessage
}

// objectMembers parses a JSON object keeping its keys' order.
func objectMembers(data []byte) ([]member, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New("not a JSON object")
	}
	var out []member
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, member{key, v})
	}
	return out, nil
}

// mcpMembers returns the servers under key in a config, none when the key
// is missing.
func mcpMembers(data []byte, key string) ([]member, error) {
	top, err := objectMembers(data)
	if err != nil {
		return nil, err
	}
	for _, m := range top {
		if m.key == key {
			if bytes.Equal(bytes.TrimSpace(m.value), []byte("null")) {
				return nil, nil
			}
			return objectMembers(m.value)
		}
	}
	return nil, nil
}

func setMember(ms []member, key string, value []byte) []member {
	for i := range ms {
		if ms[i].key == key {
			ms[i].value = value
			return ms
		}
	}
	return append(ms, member{key, value})
}

func encodeObject(ms []member) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range ms {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(m.key)
		b.Write(k)
		b.WriteByte(':')
		b.Write(m.value)
	}
	b.WriteByte('}')
	return b.Bytes()
}

// readJSONC reads a JSON file that may have comments and trailing commas,
// as OpenCode's may, and returns it as plain JSON.
func readJSONC(file string) (data []byte, hadComments bool, err error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, false, err
	}
	data, hadComments = stripJSONC(raw)
	return data, hadComments, nil
}

func stripJSONC(in []byte) ([]byte, bool) {
	noComments, comments := stripComments(in)
	return stripTrailingCommas(noComments), comments
}

// skipString returns the index just past the string starting at in[i].
func skipString(in []byte, i int) int {
	for j := i + 1; j < len(in); j++ {
		switch in[j] {
		case '\\':
			j++
		case '"':
			return j + 1
		}
	}
	return len(in)
}

func stripComments(in []byte) ([]byte, bool) {
	out := make([]byte, 0, len(in))
	comments := false
	for i := 0; i < len(in); {
		switch {
		case in[i] == '"':
			end := skipString(in, i)
			out = append(out, in[i:end]...)
			i = end
		case in[i] == '/' && i+1 < len(in) && in[i+1] == '/':
			comments = true
			for i < len(in) && in[i] != '\n' {
				i++
			}
		case in[i] == '/' && i+1 < len(in) && in[i+1] == '*':
			comments = true
			end := bytes.Index(in[i+2:], []byte("*/"))
			if end < 0 {
				i = len(in)
			} else {
				i += end + 4
			}
			out = append(out, ' ')
		default:
			out = append(out, in[i])
			i++
		}
	}
	return out, comments
}

func stripTrailingCommas(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); {
		switch in[i] {
		case '"':
			end := skipString(in, i)
			out = append(out, in[i:end]...)
			i = end
			continue
		case ',':
			j := i + 1
			for j < len(in) && strings.ContainsRune(" \t\r\n", rune(in[j])) {
				j++
			}
			if j < len(in) && (in[j] == '}' || in[j] == ']') {
				i++
				continue
			}
		}
		out = append(out, in[i])
		i++
	}
	return out
}
