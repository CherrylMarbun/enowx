package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enowx/enowx/proto"
)

// What clones start from is kept in the synced settings/clone.json: each
// host's folder cloned into last (by its daemon's ID), the last on any
// host, and the repositories cloned lately.

const cloneURLsKept = 20

type clonePrefsFile struct {
	Dirs map[string]string `json:"dirs,omitempty"`
	Last string            `json:"last,omitempty"`
	URLs []string          `json:"urls,omitempty"`
}

func (s *Server) clonePrefsPath() string { return filepath.Join(s.dataDir, "settings", "clone.json") }

func (s *Server) readClonePrefs() clonePrefsFile {
	var f clonePrefsFile
	if data, err := os.ReadFile(s.clonePrefsPath()); err == nil {
		_ = json.Unmarshal(data, &f)
	}
	return f
}

// clonePrefs is what a clone here starts from.
func (s *Server) clonePrefs() (proto.ClonePrefs, error) {
	f := s.readClonePrefs()
	id, err := s.userdata.ID()
	if err != nil {
		return proto.ClonePrefs{}, err
	}
	out := proto.ClonePrefs{Dir: f.Dirs[id], URLs: f.URLs}
	if out.URLs == nil {
		out.URLs = []string{}
	}
	// Another host's folder is only a default when it means the same
	// here: one under the home.
	if strings.HasPrefix(f.Last, "~") {
		out.Default = f.Last
	}
	return out, nil
}

// keepClone records clone p: its folder here, its repository.
func (s *Server) keepClone(p proto.ProjectCloneParams) {
	s.clonePrefsMu.Lock()
	defer s.clonePrefsMu.Unlock()
	f := s.readClonePrefs()
	id, err := s.userdata.ID()
	if err != nil {
		return
	}
	if f.Dirs == nil {
		f.Dirs = map[string]string{}
	}
	dir := p.Dir
	if home, err := os.UserHomeDir(); err == nil {
		if rel, ok := strings.CutPrefix(filepath.Clean(dir), home); ok && (rel == "" || strings.HasPrefix(rel, "/")) {
			dir = "~" + rel
		}
	}
	f.Dirs[id], f.Last = dir, dir
	f.URLs = slices.DeleteFunc(f.URLs, func(u string) bool { return u == p.URL })
	f.URLs = append([]string{p.URL}, f.URLs...)
	if len(f.URLs) > cloneURLsKept {
		f.URLs = f.URLs[:cloneURLsKept]
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.clonePrefsPath()), 0o700)
	tmp := s.clonePrefsPath() + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, s.clonePrefsPath())
	}
}
