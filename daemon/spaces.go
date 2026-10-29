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

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/core/canvas"
	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

// Spaces (docs/cloud.md, Personal and team): the user's own notes and
// canvases in notes/ and canvas/, each team's in teams/<id>/ beside them
// with its name in team.json. A note or canvas is found in the space that
// holds it by its ID, so every method naming one works on a team's as on
// the user's own; lists hold every space's, each item marked with its
// team. A team's space comes when the host learns of the team from the
// cloud, signed in as a device of a member.

// teamSpaces keeps one store per team's space, so a team's changes go one
// at a time as the user's own do. Its mutex guards the maps.
type teamSpaces struct {
	mu       sync.Mutex
	notes    map[string]*notes.Store
	canvases map[string]*canvas.Store
}

// teamPattern is a team's ID (cloud/store).
var teamPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

func (s *Server) teamsDir() string { return filepath.Join(s.dataDir, "teams") }

// teamFile is what team.json keeps of a team: its name, and the account
// the host learned it as a member of.
type teamFile struct {
	proto.Space
	Account string `json:"account"`
}

// spaces are the teams' spaces on this host, by name: those of the
// account the host is signed in as, none while it isn't. Another
// account's stay on disk, out of sight.
func (s *Server) spaces() []proto.Space {
	st, err := s.cloud.store.Get()
	if err != nil || !st.SignedIn() {
		return []proto.Space{}
	}
	entries, _ := os.ReadDir(s.teamsDir())
	out := []proto.Space{}
	for _, e := range entries {
		if !e.IsDir() || !teamPattern.MatchString(e.Name()) {
			continue
		}
		var tf teamFile
		data, err := os.ReadFile(filepath.Join(s.teamsDir(), e.Name(), "team.json"))
		if err != nil || json.Unmarshal(data, &tf) != nil || tf.Account != st.AccountID {
			continue
		}
		tf.Team = e.Name()
		if tf.Name == "" {
			tf.Name = "Team " + tf.Team[:6]
		}
		out = append(out, tf.Space)
	}
	slices.SortFunc(out, func(a, b proto.Space) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

// learnTeams keeps the teams the cloud says the host's account is in: a
// space for each, named as the team is.
func (s *Server) learnTeams(account string, teams []api.Team) {
	changed := false
	for _, t := range teams {
		if !teamPattern.MatchString(t.ID) {
			continue
		}
		dir := filepath.Join(s.teamsDir(), t.ID)
		data, _ := json.Marshal(teamFile{Space: proto.Space{Team: t.ID, Name: t.Name}, Account: account})
		if old, err := os.ReadFile(filepath.Join(dir, "team.json")); err == nil && string(old) == string(data) {
			continue
		}
		if os.MkdirAll(dir, 0o700) != nil || os.WriteFile(filepath.Join(dir, "team.json"), data, 0o600) != nil {
			continue
		}
		changed = true
	}
	if changed {
		s.spacesChanged()
	}
}

// spacesChanged tells every app the spaces, and the notes and canvases in
// them.
func (s *Server) spacesChanged() {
	s.broadcast(proto.EventSpacesChanged, s.spaces())
	s.groupsChanged(nil)
	s.canvasesChanged()
}

// checkTeam says whether team is a space here: "" (the user's own) is.
func (s *Server) checkTeam(team string) error {
	if team == "" {
		return nil
	}
	if !teamPattern.MatchString(team) {
		return fmt.Errorf("spaces: no team %q", team)
	}
	if !slices.ContainsFunc(s.spaces(), func(sp proto.Space) bool { return sp.Team == team }) {
		return fmt.Errorf("spaces: team %s isn't on this host: sign in to the cloud as a member of it", team)
	}
	return nil
}

// notesIn is the notes of team's space, the user's own for "".
func (s *Server) notesIn(team string) (*notes.Store, error) {
	if err := s.checkTeam(team); err != nil {
		return nil, err
	}
	if team == "" {
		return s.notes, nil
	}
	s.teams.mu.Lock()
	defer s.teams.mu.Unlock()
	if s.teams.notes == nil {
		s.teams.notes = map[string]*notes.Store{}
	}
	st := s.teams.notes[team]
	if st == nil {
		st = notes.Open(filepath.Join(s.teamsDir(), team, "notes"))
		s.teams.notes[team] = st
	}
	return st, nil
}

// canvasesIn is the canvases of team's space, the user's own for "".
func (s *Server) canvasesIn(team string) (*canvas.Store, error) {
	if err := s.checkTeam(team); err != nil {
		return nil, err
	}
	if team == "" {
		return s.canvases, nil
	}
	s.teams.mu.Lock()
	defer s.teams.mu.Unlock()
	if s.teams.canvases == nil {
		s.teams.canvases = map[string]*canvas.Store{}
	}
	st := s.teams.canvases[team]
	if st == nil {
		st = canvas.Open(filepath.Join(s.teamsDir(), team, "canvas"))
		s.teams.canvases[team] = st
	}
	return st, nil
}

// spaceOf is the team whose space holds what name names inside a store's
// folder (a note's file, a canvas's folder), "" when it's the user's own
// or nowhere.
func (s *Server) spaceOf(kind, name string) string {
	if _, err := os.Stat(filepath.Join(s.dataDir, kind, name)); err == nil {
		return ""
	}
	for _, sp := range s.spaces() {
		if _, err := os.Stat(filepath.Join(s.teamsDir(), sp.Team, kind, name)); err == nil {
			return sp.Team
		}
	}
	return ""
}

// notesFor is the store holding note id: the user's own when no team's
// does, which then says it has no such note.
func (s *Server) notesFor(id string) *notes.Store {
	if !validID(id) {
		return s.notes
	}
	st, err := s.notesIn(s.spaceOf("notes", id+".md"))
	if err != nil {
		return s.notes
	}
	return st
}

// canvasFor is the store holding canvas id, as notesFor.
func (s *Server) canvasFor(id string) *canvas.Store {
	if !validID(id) {
		return s.canvases
	}
	st, err := s.canvasesIn(s.spaceOf("canvas", id))
	if err != nil {
		return s.canvases
	}
	return st
}

// teamOfCanvas is the team whose space holds canvas id.
func (s *Server) teamOfCanvas(id string) string {
	if !validID(id) {
		return ""
	}
	return s.spaceOf("canvas", id)
}

// validID is a note's or canvas's ID, nothing that could walk out of a
// folder.
func validID(id string) bool {
	return id != "" && !strings.ContainsAny(id, `/\.`) && len(id) <= 64
}

// allNotes are the notes of every space, each marked with its team.
func (s *Server) allNotes() ([]proto.NoteInfo, error) {
	out, err := s.notes.List()
	if err != nil {
		return nil, err
	}
	for _, sp := range s.spaces() {
		st, err := s.notesIn(sp.Team)
		if err != nil {
			continue
		}
		list, err := st.List()
		if err != nil {
			continue
		}
		for i := range list {
			list[i].Team = sp.Team
		}
		out = append(out, list...)
	}
	return out, nil
}

// allGroups are the groups of every space, each marked with its team.
func (s *Server) allGroups() ([]proto.NoteGroup, error) {
	out, err := s.notes.Groups()
	if err != nil {
		return nil, err
	}
	for _, sp := range s.spaces() {
		st, err := s.notesIn(sp.Team)
		if err != nil {
			continue
		}
		list, err := st.Groups()
		if err != nil {
			continue
		}
		for i := range list {
			list[i].Team = sp.Team
		}
		out = append(out, list...)
	}
	return out, nil
}

// allCanvases are the canvases of every space, the latest changed first,
// each marked with its team.
func (s *Server) allCanvases() ([]proto.CanvasInfo, error) {
	out, err := s.canvases.List()
	if err != nil {
		return nil, err
	}
	for _, sp := range s.spaces() {
		st, err := s.canvasesIn(sp.Team)
		if err != nil {
			continue
		}
		list, err := st.List()
		if err != nil {
			continue
		}
		for i := range list {
			list[i].Team = sp.Team
		}
		out = append(out, list...)
	}
	slices.SortStableFunc(out, func(a, b proto.CanvasInfo) int { return b.Updated.Compare(a.Updated) })
	return out, nil
}

// allProposals are agents' changes waiting in every space.
func (s *Server) allProposals() ([]proto.NoteProposal, error) {
	out, err := s.notes.Proposals()
	if err != nil {
		return nil, err
	}
	for _, sp := range s.spaces() {
		if st, err := s.notesIn(sp.Team); err == nil {
			if list, err := st.Proposals(); err == nil {
				out = append(out, list...)
			}
		}
	}
	return out, nil
}

// proposalStore is the store proposal id waits in, the user's own when
// none holds it (which then says so).
func (s *Server) proposalStore(id string) *notes.Store {
	for _, sp := range s.spaces() {
		st, err := s.notesIn(sp.Team)
		if err != nil {
			continue
		}
		if list, err := st.Proposals(); err == nil && slices.ContainsFunc(list, func(p proto.NoteProposal) bool { return p.ID == id }) {
			return st
		}
	}
	return s.notes
}

// inTeam marks a note read from a store with the team whose space it is.
func (s *Server) inTeam(n proto.NoteInfo, err error) (proto.NoteInfo, error) {
	if err == nil {
		n.Team = s.spaceOf("notes", n.ID+".md")
	}
	return n, err
}

var errOtherSpace = errors.New("spaces: a page goes under a page of its own space")
