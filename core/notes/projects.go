package notes

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// A note's projects: attached by Attach, detached by Detach, kept in its
// header. A project with a Git remote is named by it, so its clones on
// other hosts are the same project; one without by its host and folder.

// ProjectRef is the name of the project at folder dir of daemon host,
// with Git remote remote ("" for none).
func ProjectRef(remote, host, dir string) string {
	if r := NormalRemote(remote); r != "" {
		return "git:" + r
	}
	return "path:" + host + ":" + dir
}

// NormalRemote is a Git remote's address the same however it's written:
// "git@github.com:me/app.git", "https://github.com/me/app" and
// "ssh://git@github.com/me/app.git" are all "github.com/me/app".
func NormalRemote(remote string) string {
	r := strings.TrimSpace(remote)
	if r == "" {
		return ""
	}
	if u, err := url.Parse(r); err == nil && u.Host != "" {
		r = u.Hostname() + "/" + strings.TrimPrefix(u.Path, "/")
	} else if at := strings.Index(r, "@"); at >= 0 && strings.Contains(r[at:], ":") {
		host, p, _ := strings.Cut(r[at+1:], ":")
		r = host + "/" + strings.TrimPrefix(p, "/")
	}
	r = strings.TrimSuffix(strings.TrimSuffix(r, "/"), ".git")
	if host, rest, ok := strings.Cut(r, "/"); ok {
		r = strings.ToLower(host) + "/" + rest
	}
	return r
}

// CatalogProject is a host's project as a note's project.
func CatalogProject(cat proto.Catalog, p proto.CatalogProject) proto.NoteProject {
	return proto.NoteProject{Ref: ProjectRef(p.Remote, cat.Host, p.Path), Name: p.Name, Host: cat.Host,
		HostName: cat.Name, Path: p.Path, Remote: p.Remote}
}

// IsProject reports whether a note's project np is project p of host.
func IsProject(np proto.NoteProject, host string, p proto.CatalogProject) bool {
	ref := np.Ref
	if ref == "" {
		ref = ProjectRef(np.Remote, np.Host, np.Path)
	}
	return ref == ProjectRef(p.Remote, host, p.Path) || np.Host == host && np.Path == p.Path
}

// Attach attaches note id to projects, besides those it has (the same
// project once).
func (s *Store) Attach(id string, projects []proto.NoteProject) (proto.NoteInfo, error) {
	return s.changeProjects(id, func(have []proto.NoteProject) ([]proto.NoteProject, error) {
		for _, p := range projects {
			if p.Ref == "" {
				p.Ref = ProjectRef(p.Remote, p.Host, p.Path)
			}
			if p.Ref == "path::" || p.Name == "" && p.Path == "" {
				return nil, fmt.Errorf("notes: a project to attach needs a folder or a Git remote")
			}
			if p.Name == "" {
				p.Name = path.Base(p.Path)
			}
			if i := slices.IndexFunc(have, func(o proto.NoteProject) bool { return o.Ref == p.Ref }); i >= 0 {
				have[i] = p // relinked: where it is now
				continue
			}
			have = append(have, p)
		}
		return have, nil
	})
}

// Detach detaches note id from the projects refs, or from all; the note
// stays.
func (s *Store) Detach(id string, refs []string, all bool) (proto.NoteInfo, error) {
	return s.changeProjects(id, func(have []proto.NoteProject) ([]proto.NoteProject, error) {
		if all {
			return nil, nil
		}
		kept := slices.DeleteFunc(have, func(p proto.NoteProject) bool { return slices.Contains(refs, p.Ref) })
		if len(kept) == len(have) && len(refs) > 0 {
			return nil, fmt.Errorf("notes: the note isn't attached to %s", strings.Join(refs, ", "))
		}
		return kept, nil
	})
}

func (s *Store) changeProjects(id string, change func([]proto.NoteProject) ([]proto.NoteProject, error)) (proto.NoteInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.read(id, true)
	if err != nil {
		return proto.NoteInfo{}, err
	}
	projects, err := change(slices.Clone(n.Projects))
	if err != nil {
		return proto.NoteInfo{}, err
	}
	n.Projects = projects
	n.Updated = time.Now().UTC().Truncate(time.Second)
	if err := s.write(n); err != nil {
		return proto.NoteInfo{}, err
	}
	return n.NoteInfo, nil
}
