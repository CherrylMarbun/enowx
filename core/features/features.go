// Package features keeps which of enowX's features the user has on
// (core/agent.Features): for every host (settings/features.json, synced),
// per project in it, per host (features-host.json, this host's alone),
// and per session; and resolves them for a session.
package features

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/enowx/enowx/core/agent"
)

// Scopes a switch is set in, from the widest; a narrower one wins.
const (
	ScopeGlobal  = "global"
	ScopeHost    = "host"
	ScopeProject = "project"
	ScopeSession = "session"
)

// Settings are the user's switches kept for every host.
type Settings struct {
	Global map[string]bool `json:"global,omitempty"`
	// Projects are switches per project, by Key.
	Projects map[string]map[string]bool `json:"projects,omitempty"`
	// Lazy gives long features' guides as a line pointing at their help;
	// nil is on.
	Lazy *bool `json:"lazy,omitempty"`
}

// LazyOn reports whether guides are lazy.
func (s Settings) LazyOn() bool { return s.Lazy == nil || *s.Lazy }

// Store is a host's switches: the synced settings and its own.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open is the switches kept in daemon folder dir.
func Open(dir string) *Store { return &Store{dir: dir} }

func (s *Store) settingsFile() string { return filepath.Join(s.dir, "settings", "features.json") }
func (s *Store) hostFile() string     { return filepath.Join(s.dir, "features-host.json") }

// Settings reads the synced switches.
func (s *Store) Settings() (Settings, error) {
	var out Settings
	err := readJSON(s.settingsFile(), &out)
	return out, err
}

// Host reads this host's own switches.
func (s *Store) Host() (map[string]bool, error) {
	out := map[string]bool{}
	err := readJSON(s.hostFile(), &out)
	return out, err
}

// Set turns feature id on or off (nil: as the wider scope has it) in
// scope; project names the project for ScopeProject.
func (s *Store) Set(id string, on *bool, scope, project string) error {
	f, ok := agent.FindFeature(id)
	if !ok {
		return fmt.Errorf("features: no feature %q", id)
	}
	if f.Core && on != nil && !*on {
		return fmt.Errorf("features: %s can't be turned off", f.Name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	set := func(m map[string]bool) map[string]bool {
		if m == nil {
			m = map[string]bool{}
		}
		if on == nil {
			delete(m, id)
		} else {
			m[id] = *on
		}
		return m
	}
	switch scope {
	case ScopeGlobal, ScopeProject:
		st, err := s.Settings()
		if err != nil {
			return err
		}
		if scope == ScopeGlobal {
			st.Global = set(st.Global)
		} else {
			if project == "" {
				return errors.New("features: which project?")
			}
			if st.Projects == nil {
				st.Projects = map[string]map[string]bool{}
			}
			st.Projects[project] = set(st.Projects[project])
			if len(st.Projects[project]) == 0 {
				delete(st.Projects, project)
			}
		}
		return writeJSON(s.settingsFile(), st)
	case ScopeHost:
		h, err := s.Host()
		if err != nil {
			return err
		}
		return writeJSON(s.hostFile(), set(h))
	}
	return fmt.Errorf("features: no scope %q", scope)
}

// SetLazy turns lazy guides on or off.
func (s *Store) SetLazy(lazy bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.Settings()
	if err != nil {
		return err
	}
	st.Lazy = &lazy
	return writeJSON(s.settingsFile(), st)
}

// Resolved is a feature's state and the scope that decided it ("" for
// its default).
type Resolved struct {
	On   bool
	From string
}

// Resolve is every feature's state for a session of project (its Key, ""
// for none) with switches session of its own; noDisplay turns the
// display off by default, on a host without one.
func Resolve(st Settings, host map[string]bool, project string, session map[string]bool, noDisplay bool) map[string]Resolved {
	out := map[string]Resolved{}
	for _, f := range agent.Features {
		r := Resolved{On: f.Default || f.Core}
		if f.ID == "display" && noDisplay {
			r.On = false
		}
		for _, layer := range []struct {
			m    map[string]bool
			from string
		}{{st.Global, ScopeGlobal}, {host, ScopeHost}, {st.Projects[project], ScopeProject}, {session, ScopeSession}} {
			if v, ok := layer.m[f.ID]; ok {
				r = Resolved{On: v, From: layer.from}
			}
		}
		if f.Core {
			r.On = true
		}
		out[f.ID] = r
	}
	return out
}

// On is the features on among r.
func On(r map[string]Resolved) map[string]bool {
	out := map[string]bool{}
	for id, v := range r {
		out[id] = v.On
	}
	return out
}

// Presets are switches to set all at once, by name.
var Presets = map[string][]string{
	"Minimal": {"notes", "sync", "status"},
	"Web dev": {"notes", "sync", "status", "browser", "ports", "traffic", "agents", "clone", "autosync", "blocks", "icons", "groups", "notes-agent"},
	"Android": {"notes", "sync", "status", "ports", "android", "traffic", "agents", "clone", "blocks", "icons", "groups", "notes-agent"},
	"All":     nil, // every feature
}

// PresetSwitches are preset name's switches for every feature.
func PresetSwitches(name string) (map[string]bool, bool) {
	ids, ok := Presets[name]
	if !ok {
		return nil, false
	}
	out := map[string]bool{}
	for _, f := range agent.Features {
		on := ids == nil
		for _, id := range ids {
			if id == f.ID {
				on = true
			}
		}
		out[f.ID] = on || f.Core
	}
	return out, true
}

// Missing are the features feature id needs that on hasn't on.
func Missing(id string, on map[string]bool) []string {
	f, _ := agent.FindFeature(id)
	var out []string
	for _, r := range f.Requires {
		if !on[r] {
			out = append(out, r)
		}
	}
	return out
}

func readJSON(file string, v any) error {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("features: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("features: %s: %w", filepath.Base(file), err)
	}
	return nil
}

func writeJSON(file string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return fmt.Errorf("features: %w", err)
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("features: %w", err)
	}
	return os.Rename(tmp, file)
}
