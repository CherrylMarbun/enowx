package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/proto"
)

// projectsChanged tells every app the projects, and publishes them in the
// synced catalog for the other hosts.
func (s *Server) projectsChanged() {
	list := s.projects.List()
	s.broadcast(proto.EventProjectsChanged, list)
	s.publishSoon(list)
}

// publishSoon publishes list off the caller's goroutine; Close waits for
// it.
func (s *Server) publishSoon(list []proto.ProjectInfo) {
	s.catalogs.Add(1)
	go func() {
		defer s.catalogs.Done()
		s.publishCatalog(list)
	}()
}

// publishCatalog writes this host's projects to catalog/<its ID>.json,
// when they changed.
func (s *Server) publishCatalog(list []proto.ProjectInfo) {
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	id, err := s.userdata.ID()
	if err != nil {
		return
	}
	cat := proto.Catalog{Host: id, Name: s.userdata.Name, Projects: []proto.CatalogProject{}}
	for _, p := range list {
		remote, _ := git.Run(p.Path, "config", "--get", "remote.origin.url")
		cat.Projects = append(cat.Projects, proto.CatalogProject{Name: p.Name, Path: p.Path, Remote: strings.TrimSpace(remote), Group: p.Group})
	}
	file := filepath.Join(s.dataDir, "catalog", id+".json")
	var old proto.Catalog
	if data, err := os.ReadFile(file); err == nil && json.Unmarshal(data, &old) == nil &&
		old.Name == cat.Name && slices.Equal(old.Projects, cat.Projects) {
		return
	}
	cat.Updated = time.Now().UTC().Truncate(time.Second)
	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(file), 0o700)
	tmp := file + ".tmp"
	if os.WriteFile(tmp, data, 0o600) == nil {
		_ = os.Rename(tmp, file)
	}
}

// catalogs is every host's catalog this host has.
func (s *Server) catalogList() ([]proto.Catalog, error) {
	out := []proto.Catalog{}
	entries, err := os.ReadDir(filepath.Join(s.dataDir, "catalog"))
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dataDir, "catalog", e.Name()))
		if err != nil {
			continue
		}
		var c proto.Catalog
		if json.Unmarshal(data, &c) == nil && c.Host != "" {
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b proto.Catalog) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}
