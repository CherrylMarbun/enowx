package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/enowx/enowx/proto"
)

// Variables the user sets for a host (API keys, tokens) go into every
// session's environment, kept in <daemon dir>/session-env.json, readable
// by its user alone, so sessions continued after a restart have them too.
// They are never logged, and the app shows them masked.

var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// sessionVars is the variables, safe for concurrent use.
type sessionVars struct {
	mu   sync.Mutex
	path string
	vars map[string]string
}

func (v *sessionVars) load(path string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.path, v.vars = path, map[string]string{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &v.vars)
	}
}

// env is the variables as NAME=value entries, by name.
func (v *sessionVars) env() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []string
	for _, name := range sortedNames(v.vars) {
		out = append(out, name+"="+v.vars[name])
	}
	return out
}

// list is the variables, their values masked.
func (v *sessionVars) list() []proto.SessionEnvVar {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := []proto.SessionEnvVar{}
	for _, name := range sortedNames(v.vars) {
		out = append(out, proto.SessionEnvVar{Name: name, Value: mask(v.vars[name])})
	}
	return out
}

// set sets a variable, or removes it with an empty value.
func (v *sessionVars) set(p proto.SessionEnvVar) error {
	if !envName.MatchString(p.Name) {
		return fmt.Errorf("%q isn't a variable's name", p.Name)
	}
	if slices.Contains([]string{"PATH", "HOME", "USER", "SHELL", "ENOWX_SESSION", "ENOWX_BIN"}, p.Name) {
		return fmt.Errorf("%s is set for sessions already", p.Name)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if p.Value == "" {
		delete(v.vars, p.Name)
	} else {
		v.vars[p.Name] = p.Value
	}
	data, err := json.MarshalIndent(v.vars, "", "  ")
	if err != nil {
		return err
	}
	if v.path == "" {
		return errors.New("no place to keep them")
	}
	return writeFileAtomic(v.path, data)
}

// mask keeps a value's last 4 characters, enough to tell keys apart.
func mask(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("•", 6)
	}
	return "••••" + s[len(s)-4:]
}

func sortedNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func (s *Server) sessionEnvFile() string {
	return filepath.Join(filepath.Dir(s.WorktreeRoot), "session-env.json")
}
