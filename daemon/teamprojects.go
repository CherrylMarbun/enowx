package daemon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/core/git"
	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// A team's projects (docs/cloud.md): Git links its admins add, kept in
// projects.json of its space, which syncs with it. This host's copy of one
// is found by its Git remote, among the host's projects and the folders of
// its sessions running, so a member who has it already needn't clone it.

// teamProjectState caches the Git remotes of folders looked at, and the
// sessions' folders seen, so a new session's folder is looked at once. Its
// mutex guards both.
type teamProjectState struct {
	mu      sync.Mutex
	remotes map[string]cachedRemote
	seen    map[string]bool
}

type cachedRemote struct {
	ref string
	at  time.Time
}

const remoteFresh = 5 * time.Minute

type storedTeamProject struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Repo    string    `json:"repo"`
	Added   time.Time `json:"added"`
	AddedBy string    `json:"addedBy,omitempty"`
}

func (s *Server) teamProjectsFile(team string) string {
	return filepath.Join(s.teamsDir(), team, "projects.json")
}

func (s *Server) readTeamProjects(team string) []storedTeamProject {
	var list []storedTeamProject
	if data, err := os.ReadFile(s.teamProjectsFile(team)); err == nil {
		_ = json.Unmarshal(data, &list)
	}
	return list
}

func (s *Server) writeTeamProjects(team string, list []storedTeamProject) error {
	data, _ := json.MarshalIndent(list, "", "  ")
	return writeTeamFile(s.teamProjectsFile(team), data)
}

// teamProjects are every team's projects, each with its copies here.
func (s *Server) teamProjects() []proto.TeamProject {
	out := []proto.TeamProject{}
	spaces := s.spaces()
	if len(spaces) == 0 {
		return out
	}
	here := s.hostCopies()
	for _, sp := range spaces {
		for _, p := range s.readTeamProjects(sp.Team) {
			ref := notes.ProjectRef(p.Repo, "", "")
			out = append(out, proto.TeamProject{ID: p.ID, Team: sp.Team, Name: p.Name, Repo: p.Repo, Ref: ref, Added: p.Added, AddedBy: p.AddedBy, Here: here[ref]})
		}
	}
	return out
}

// hostCopies are this host's folders by the Git remote they hold: its
// projects', then its sessions' that are no project's.
func (s *Server) hostCopies() map[string][]proto.TeamProjectCopy {
	out := map[string][]proto.TeamProjectCopy{}
	projects := s.projects.List()
	for _, p := range projects {
		if ref := s.remoteRef(p.Path); ref != "" {
			out[ref] = append(out[ref], proto.TeamProjectCopy{Path: p.Path, Project: true})
		}
	}
	for _, sess := range s.sessions.List() {
		info := sess.Info()
		if info.Exited || info.Dir == "" || info.NotesFor != "" || info.CanvasFor != "" {
			continue
		}
		if slices.ContainsFunc(projects, func(p proto.ProjectInfo) bool { return info.Project == p.Path || within(info.Dir, p.Path) }) {
			continue
		}
		if ref := s.remoteRef(info.Dir); ref != "" && !slices.ContainsFunc(out[ref], func(c proto.TeamProjectCopy) bool { return c.Path == info.Dir }) {
			out[ref] = append(out[ref], proto.TeamProjectCopy{Path: info.Dir, Session: info.ID})
		}
	}
	return out
}

func within(dir, root string) bool {
	return dir == root || strings.HasPrefix(dir, strings.TrimSuffix(root, string(filepath.Separator))+string(filepath.Separator))
}

// remoteRef is the project ref of folder dir's Git remote, "" for none.
func (s *Server) remoteRef(dir string) string {
	st := &s.teamProjectsState
	st.mu.Lock()
	c, ok := st.remotes[dir]
	st.mu.Unlock()
	if ok && time.Since(c.at) < remoteFresh {
		return c.ref
	}
	ref := ""
	if remote, err := git.Run(dir, "config", "--get", "remote.origin.url"); err == nil && strings.TrimSpace(remote) != "" {
		ref = notes.ProjectRef(strings.TrimSpace(remote), "", "")
	}
	st.mu.Lock()
	if st.remotes == nil {
		st.remotes = map[string]cachedRemote{}
	}
	st.remotes[dir] = cachedRemote{ref: ref, at: time.Now()}
	st.mu.Unlock()
	return ref
}

// teamProjectsChanged tells every app the teams' projects.
func (s *Server) teamProjectsChanged() {
	if len(s.spaces()) == 0 {
		return
	}
	s.broadcast(proto.EventTeamProjectsChanged, s.teamProjects())
}

// sessionFolderSeen looks again for the teams' projects here once a
// session runs in a folder not seen before.
func (s *Server) sessionFolderSeen(info proto.SessionInfo) {
	if info.Dir == "" || info.Exited {
		return
	}
	st := &s.teamProjectsState
	st.mu.Lock()
	if st.seen == nil {
		st.seen = map[string]bool{}
	}
	fresh := !st.seen[info.Dir]
	st.seen[info.Dir] = true
	st.mu.Unlock()
	if fresh {
		go s.teamProjectsChanged()
	}
}

// cleanRepo is a Git link fit to share: a repository's address, with no
// credentials in it.
func cleanRepo(raw string) (string, error) {
	r := strings.TrimSpace(raw)
	if u, err := url.Parse(r); err == nil && u.Host != "" {
		_, password := u.User.Password()
		if password || u.User != nil && u.Scheme != "ssh" && u.User.Username() != "" {
			return "", errors.New("teams: the link has credentials in it: add it without them; each member clones with their own")
		}
	}
	ref := notes.NormalRemote(r)
	if ref == "" || !strings.Contains(ref, "/") || strings.ContainsAny(r, " \t\n") {
		return "", fmt.Errorf("teams: %q isn't a Git link (https://github.com/me/app, git@github.com:me/app.git)", raw)
	}
	return r, nil
}

// addTeamProject adds a Git link to a team, as one of its admins.
func (s *Server) addTeamProject(p proto.TeamProjectAddParams) (proto.TeamProject, error) {
	sp, err := s.adminOf(p.Team)
	if err != nil {
		return proto.TeamProject{}, err
	}
	repo, err := cleanRepo(p.Repo)
	if err != nil {
		return proto.TeamProject{}, err
	}
	ref := notes.ProjectRef(repo, "", "")
	list := s.readTeamProjects(sp.Team)
	if slices.ContainsFunc(list, func(x storedTeamProject) bool { return notes.ProjectRef(x.Repo, "", "") == ref }) {
		return proto.TeamProject{}, fmt.Errorf("teams: %s has %s already", sp.Name, strings.TrimPrefix(ref, "git:"))
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = path.Base(strings.TrimPrefix(ref, "git:"))
	}
	var b [6]byte
	rand.Read(b[:])
	st, _ := s.cloud.store.Get()
	added := storedTeamProject{ID: hex.EncodeToString(b[:]), Name: name, Repo: repo, Added: time.Now().UTC().Truncate(time.Second), AddedBy: st.Account}
	if err := s.writeTeamProjects(sp.Team, append(list, added)); err != nil {
		return proto.TeamProject{}, err
	}
	s.teamProjectsChanged()
	s.teamSync.kick()
	return proto.TeamProject{ID: added.ID, Team: sp.Team, Name: name, Repo: repo, Ref: ref, Added: added.Added, AddedBy: added.AddedBy, Here: s.hostCopies()[ref]}, nil
}

// removeTeamProject takes a project out of a team, as one of its admins;
// its copies on the members' hosts stay theirs.
func (s *Server) removeTeamProject(p proto.TeamProjectRef) error {
	sp, err := s.adminOf(p.Team)
	if err != nil {
		return err
	}
	list := s.readTeamProjects(sp.Team)
	kept := slices.DeleteFunc(slices.Clone(list), func(x storedTeamProject) bool { return x.ID == p.ID })
	if len(kept) == len(list) {
		return fmt.Errorf("teams: %s has no project %q", sp.Name, p.ID)
	}
	if err := s.writeTeamProjects(sp.Team, kept); err != nil {
		return err
	}
	s.teamProjectsChanged()
	s.teamSync.kick()
	return nil
}

// adminOf is team's space when the host's account is one of its admins.
func (s *Server) adminOf(team string) (proto.Space, error) {
	for _, sp := range s.spaces() {
		if sp.Team == team {
			if sp.Role != api.RoleOwner && sp.Role != api.RoleAdmin {
				return sp, fmt.Errorf("teams: only %s's admins change its projects", sp.Name)
			}
			return sp, nil
		}
	}
	return proto.Space{}, fmt.Errorf("spaces: team %s isn't on this host", team)
}
