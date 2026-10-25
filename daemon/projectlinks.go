package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/core/project"
	"github.com/enowx/enowx/core/projectlinks"
	"github.com/enowx/enowx/proto"
)

// projectByID is the folder here of the project with ID id, "" when
// none of the projects here is it.
func (s *Server) projectByID(id string) string {
	if !projectlinks.ValidID(id) {
		return ""
	}
	for _, p := range s.projects.List() {
		if projectlinks.ReadID(p.Path) == id {
			return p.Path
		}
	}
	return ""
}

// projectID is the ID of the project at dir (~/ allowed), made when it
// has none.
func (s *Server) projectID(dir string) (proto.ProjectIdentity, error) {
	dir, err := expandHome(dir)
	if err != nil {
		return proto.ProjectIdentity{}, err
	}
	id, err := projectlinks.EnsureID(dir)
	return proto.ProjectIdentity{ID: id, Path: dir}, err
}

// findProject is the project here with ID p.ID: the folder p.Path when
// it still is (a link's last known), else the first project that is.
func (s *Server) findProject(p proto.ProjectIdentity) (proto.ProjectInfo, error) {
	if dir, err := expandHome(p.Path); p.Path != "" && err == nil && projectlinks.ReadID(dir) == p.ID {
		return project.Inspect(dir).ProjectInfo, nil
	}
	for _, info := range s.projects.List() {
		if projectlinks.ReadID(info.Path) == p.ID {
			return info, nil
		}
	}
	return proto.ProjectInfo{}, fmt.Errorf("no project here is %s", p.ID)
}

// saveLink records l, its project's copy here given the ID it links when
// it has none (a copy takes its source's).
func (s *Server) saveLink(l proto.ProjectLink) (proto.ProjectLink, error) {
	for _, end := range []proto.ProjectEnd{l.Source, l.Target} {
		if end.Daemon != s.daemonID || l.Project == "" {
			continue
		}
		if dir, err := expandHome(end.Path); err == nil && projectlinks.ReadID(dir) == "" {
			if err := projectlinks.WriteID(dir, l.Project); err != nil {
				return l, err
			}
		}
	}
	out, err := s.links.SaveLink(l)
	if err == nil {
		s.linksChanged()
	}
	return out, err
}

func (s *Server) linksChanged() {
	if list, err := s.links.States(""); err == nil {
		s.broadcast(proto.EventProjectLinksChanged, list)
	}
}

func (s *Server) reportLink(r proto.ProjectLinkReport) error {
	if err := s.links.Report(r); err != nil {
		return err
	}
	s.linksChanged()
	if r.Entry.Kind != "" {
		s.broadcast(proto.EventProjectHistory, r.Entry)
	}
	return nil
}

// copyProject has the app copy a project here to another host.
func (s *Server) copyProject(p proto.ProjectCopyParams) (proto.ProjectCopyResult, error) {
	dir, err := expandHome(p.Project)
	if err != nil {
		return proto.ProjectCopyResult{}, err
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return proto.ProjectCopyResult{}, fmt.Errorf("project: %s isn't a folder here", dir)
	}
	if strings.TrimSpace(p.To) == "" {
		return proto.ProjectCopyResult{}, errors.New("project: copy to which host? (--to HOST)")
	}
	p.Project = dir
	raw, err := s.askApp("project.copy", p, 2*time.Hour)
	if err != nil {
		return proto.ProjectCopyResult{}, err
	}
	var res proto.ProjectCopyResult
	return res, json.Unmarshal(raw, &res)
}

// syncLinkNow has the app sync link id now.
func (s *Server) syncLinkNow(id string) (proto.ProjectLinkState, error) {
	raw, err := s.askApp("project.sync", proto.ProjectLinkRef{ID: id}, 30*time.Minute)
	if err != nil {
		return proto.ProjectLinkState{}, err
	}
	var st proto.ProjectLinkState
	return st, json.Unmarshal(raw, &st)
}

// projectFileLimit bounds a file read to compare versions in conflict.
const projectFileLimit = 4 << 20

// projectFile reads file p.Path of project p.Project, to compare versions.
func (s *Server) projectFile(p proto.ProjectFileParams) ([]byte, error) {
	dir, err := expandHome(p.Project)
	if err != nil {
		return nil, err
	}
	target, err := insideOf(dir, p.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	if info.Size() > projectFileLimit {
		return nil, fmt.Errorf("project: %s is larger than %d MB", p.Path, projectFileLimit>>20)
	}
	return os.ReadFile(target)
}

// keepCopy copies file p.Path of project p.Project beside it, its name
// marked with p.Suffix, and returns the copy's path.
func (s *Server) keepCopy(p proto.ProjectKeepCopyParams) (string, error) {
	dir, err := expandHome(p.Project)
	if err != nil {
		return "", err
	}
	src, err := insideOf(dir, p.Path)
	if err != nil {
		return "", err
	}
	suffix := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ' ' || r == ':' {
			return '-'
		}
		return r
	}, p.Suffix)
	ext := path.Ext(p.Path)
	rel := strings.TrimSuffix(p.Path, ext) + ".conflict-" + suffix + ext
	dst, err := insideOf(dir, rel)
	if err != nil {
		return "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("project: %w", err)
	}
	defer in.Close()
	info, _ := in.Stat()
	mode := os.FileMode(0o644)
	if info != nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return "", fmt.Errorf("project: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return "", err
	}
	return rel, out.Close()
}
