package agentapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/envsync"
)

// Editing agents' settings files in place, keeping what enowX doesn't set.
// The files hold keys: they are written readable by the user only.

const fileMode = 0o600

func readFile(home, rel string) ([]byte, error) {
	return envsync.ReadFile(filepath.Join(home, rel))
}

func writeFile(home, rel string, data []byte) error {
	mode := os.FileMode(fileMode)
	if fi, err := os.Stat(filepath.Join(home, rel)); err == nil {
		mode = fi.Mode().Perm() & fileMode // never more open than the user's own, nor than fileMode
		if mode == 0 {
			mode = fileMode
		}
	}
	return envsync.WriteFileAtomic(filepath.Join(home, rel), data, mode)
}

// jsonEdit is a change of a JSON file: a value set at a path, or with nil
// removed.
type jsonEdit struct {
	path  []string
	value any
}

// editJSON makes edits to the JSON file rel under home.
func editJSON(home, rel string, edits ...jsonEdit) error {
	data, err := readFile(home, rel)
	if err != nil {
		return err
	}
	for _, e := range edits {
		var raw json.RawMessage
		if e.value != nil {
			if raw, err = json.Marshal(e.value); err != nil {
				return err
			}
		}
		if data, _, err = envsync.EditJSON(data, e.path, raw); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
	}
	return writeFile(home, rel, data)
}

// jsonString is the string at path in the JSON file rel, "" for none.
func jsonString(home, rel string, path ...string) (string, error) {
	data, err := readFile(home, rel)
	if err != nil {
		return "", err
	}
	s, _ := envsync.JSONValue(data, path...).(string)
	return s, nil
}

// jsonAny is the value at path in the JSON file rel.
func jsonAny(home, rel string, path ...string) (any, error) {
	data, err := readFile(home, rel)
	if err != nil {
		return nil, err
	}
	return envsync.JSONValue(data, path...), nil
}

// TOML, line by line: top-level keys go before the first table; a table
// enowX owns is replaced whole.

var tomlHeaderRe = regexp.MustCompile(`^\s*\[\[?\s*([^\]]+?)\s*\]\]?\s*(#.*)?$`)

func tomlQuote(s string) string { return strconv.Quote(s) }

// tomlTop is top-level key's string value in text, "" for none.
func tomlTop(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if tomlHeaderRe.MatchString(line) {
			break
		}
		if v, ok := tomlKeyValue(line, key); ok {
			return v
		}
	}
	return ""
}

// tomlKeyValue reads `key = "value"` from line.
func tomlKeyValue(line, key string) (string, bool) {
	k, v, ok := strings.Cut(line, "=")
	if !ok || strings.TrimSpace(k) != key {
		return "", false
	}
	v = strings.TrimSpace(v)
	if i := strings.Index(v, " #"); i >= 0 && !strings.HasPrefix(v, `"`) {
		v = strings.TrimSpace(v[:i])
	}
	if u, err := strconv.Unquote(v); err == nil {
		return u, true
	}
	if strings.HasPrefix(v, "'") && strings.HasSuffix(v, "'") && len(v) >= 2 {
		return v[1 : len(v)-1], true
	}
	return v, true
}

// tomlSetTop sets top-level keys (value "" removes one) in text.
func tomlSetTop(text string, values [][2]string) string {
	lines := strings.Split(text, "\n")
	end := len(lines)
	for i, l := range lines {
		if tomlHeaderRe.MatchString(l) {
			end = i
			break
		}
	}
	top, rest := lines[:end], lines[end:]
	for _, kv := range values {
		kept := top[:0:0]
		for _, l := range top {
			if _, ok := tomlKeyValue(l, kv[0]); !ok {
				kept = append(kept, l)
			}
		}
		top = kept
	}
	var add []string
	for _, kv := range values {
		if kv[1] != "" {
			add = append(add, kv[0]+" = "+tomlQuote(kv[1]))
		}
	}
	out := append(add, top...)
	if len(rest) > 0 {
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
		out = append(out, "")
		out = append(out, rest...)
	}
	return strings.Join(out, "\n")
}

// tomlTable is table name's keys' string values, nil when there is none.
func tomlTable(text, name string) map[string]string {
	var out map[string]string
	in := false
	for _, l := range strings.Split(text, "\n") {
		if m := tomlHeaderRe.FindStringSubmatch(l); m != nil {
			in = m[1] == name
			if in && out == nil {
				out = map[string]string{}
			}
			continue
		}
		if in {
			if k, _, ok := strings.Cut(l, "="); ok {
				key := strings.TrimSpace(k)
				if v, ok := tomlKeyValue(l, key); ok {
					out[key] = v
				}
			}
		}
	}
	return out
}

// tomlSetTable replaces table name (and its subtables) with body, the
// table's lines after its header; "" removes it.
func tomlSetTable(text, name, body string) string {
	lines := strings.Split(text, "\n")
	var out []string
	skip := false
	for _, l := range lines {
		if m := tomlHeaderRe.FindStringSubmatch(l); m != nil {
			skip = m[1] == name || strings.HasPrefix(m[1], name+".")
		}
		if !skip {
			out = append(out, l)
		}
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	if body != "" {
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, "["+name+"]", strings.TrimRight(body, "\n"))
	}
	return strings.Join(out, "\n") + "\n"
}

// YAML, top-level scalars only: enowX sets keys of its own at the top of
// the file and leaves the rest.

var yamlKeyRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+):\s*(.*)$`)

func yamlTop(text, key string) string {
	for _, l := range strings.Split(text, "\n") {
		if m := yamlKeyRe.FindStringSubmatch(l); m != nil && m[1] == key {
			v := strings.TrimSpace(m[2])
			if u, err := strconv.Unquote(v); err == nil {
				return u
			}
			return strings.Trim(v, "'")
		}
	}
	return ""
}

// yamlSetTop sets top-level scalar keys (value "" removes one).
func yamlSetTop(text string, values [][2]string) string {
	lines := strings.Split(text, "\n")
	for _, kv := range values {
		kept := lines[:0:0]
		for _, l := range lines {
			if m := yamlKeyRe.FindStringSubmatch(l); m != nil && m[1] == kv[0] {
				continue
			}
			kept = append(kept, l)
		}
		lines = kept
	}
	var add []string
	for _, kv := range values {
		if kv[1] != "" {
			add = append(add, kv[0]+": "+strconv.Quote(kv[1]))
		}
	}
	out := strings.TrimLeft(strings.Join(lines, "\n"), "\n")
	if len(add) == 0 {
		return out
	}
	return strings.Join(add, "\n") + "\n" + out
}

// .env files: KEY=VALUE lines.

func envValue(text, key string) string {
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "export "))
		if k, v, ok := strings.Cut(l, "="); ok && strings.TrimSpace(k) == key {
			v = strings.TrimSpace(v)
			if u, err := strconv.Unquote(v); err == nil {
				return u
			}
			return strings.Trim(v, "'")
		}
	}
	return ""
}

func envSet(text string, values [][2]string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for _, kv := range values {
		kept := lines[:0:0]
		for _, l := range lines {
			t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "export "))
			if k, _, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == kv[0] {
				continue
			}
			kept = append(kept, l)
		}
		lines = kept
	}
	for _, kv := range values {
		if kv[1] != "" {
			lines = append(lines, kv[0]+"="+strconv.Quote(kv[1]))
		}
	}
	out := strings.TrimLeft(strings.Join(lines, "\n"), "\n")
	if out == "" {
		return ""
	}
	return out + "\n"
}

// Arrays of tables ([[name]]): an item is found by one of its keys.

// tomlArrayItems splits text into its [[name]] items' line ranges.
func tomlArrayItems(lines []string, name string) [][2]int {
	var out [][2]int
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "[["+name+"]]" {
			continue
		}
		j := i + 1
		for j < len(lines) && !tomlHeaderRe.MatchString(lines[j]) {
			j++
		}
		out = append(out, [2]int{i, j})
		i = j - 1
	}
	return out
}

// tomlArrayItem is the [[name]] item whose key is value, nil for none.
func tomlArrayItem(text, name, key, value string) map[string]string {
	lines := strings.Split(text, "\n")
	for _, r := range tomlArrayItems(lines, name) {
		item := map[string]string{}
		for _, l := range lines[r[0]+1 : r[1]] {
			if k, _, ok := strings.Cut(l, "="); ok {
				if v, ok := tomlKeyValue(l, strings.TrimSpace(k)); ok {
					item[strings.TrimSpace(k)] = v
				}
			}
		}
		if item[key] == value {
			return item
		}
	}
	return nil
}

// tomlRemoveArrayItem removes the [[name]] items whose key is value.
func tomlRemoveArrayItem(text, name, key, value string) string {
	lines := strings.Split(text, "\n")
	ranges := tomlArrayItems(lines, name)
	for i := len(ranges) - 1; i >= 0; i-- {
		r := ranges[i]
		for _, l := range lines[r[0]+1 : r[1]] {
			if v, ok := tomlKeyValue(l, key); ok && v == value {
				lines = append(lines[:r[0]], lines[r[1]:]...)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}
