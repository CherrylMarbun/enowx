package projectlinks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/enowx/enowx/proto"
)

// Store keeps a daemon's links, auto pushes and what became of them, in
// its folder dir. The user's settings are read again at each call: a sync
// of the user's data may replace them. Safe for several goroutines.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open is the store in the daemon's folder dir.
func Open(dir string) *Store { return &Store{dir: dir} }

// Files of the store, under the daemon's folder: the user's settings
// (synced), then what this host saw.
const (
	LinksFile     = "settings/project-links.json"
	AutopushFile  = "settings/autopush.json"
	statusFile    = "project-link-status.json"
	historyFile   = "project-history.json"
	pushStateFile = "autopush-state.json"
)

// historyKept is how many entries the history keeps, the latest.
const historyKept = 300

func (s *Store) path(name string) string { return filepath.Join(s.dir, filepath.FromSlash(name)) }

func (s *Store) read(name string, v any) error {
	data, err := os.ReadFile(s.path(name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("projectlinks: %w", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("projectlinks: %s: %w", name, err)
	}
	return nil
}

func (s *Store) write(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(s.path(name), data)
}

// Links lists the links of the project with ID project, or all with "".
func (s *Store) Links(project string) ([]proto.ProjectLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []proto.ProjectLink
	if err := s.read(LinksFile, &all); err != nil {
		return nil, err
	}
	out := []proto.ProjectLink{}
	for _, l := range all {
		if project == "" || l.Project == project {
			out = append(out, l)
		}
	}
	return out, nil
}

// SaveLink records l: a new link without ID, else link l.ID's settings.
// The same two copies are linked once: saving them again updates theirs.
func (s *Store) SaveLink(l proto.ProjectLink) (proto.ProjectLink, error) {
	if !ValidID(l.Project) {
		return l, errors.New("projectlinks: a link needs its project's ID")
	}
	if l.Source.Daemon == "" || l.Target.Daemon == "" || l.Source.Path == "" || l.Target.Path == "" {
		return l, errors.New("projectlinks: a link needs both copies' hosts and folders")
	}
	if err := checkSettings(l); err != nil {
		return l, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []proto.ProjectLink
	if err := s.read(LinksFile, &all); err != nil {
		return l, err
	}
	i := slices.IndexFunc(all, func(o proto.ProjectLink) bool {
		return l.ID != "" && o.ID == l.ID || l.ID == "" && sameEnds(o, l)
	})
	switch {
	case i >= 0:
		l.ID, l.Created = all[i].ID, all[i].Created
		all[i] = l
	default:
		// A link made on another host keeps its ID here.
		if l.ID == "" {
			l.ID = newID()
		} else if _, err := hex.DecodeString(l.ID); err != nil || len(l.ID) != 12 {
			return l, fmt.Errorf("projectlinks: not a link's ID: %q", l.ID)
		}
		if l.Created.IsZero() {
			l.Created = time.Now().UTC().Truncate(time.Second)
		}
		all = append(all, l)
	}
	return l, s.write(LinksFile, all)
}

func sameEnds(a, b proto.ProjectLink) bool {
	same := func(x, y proto.ProjectEnd) bool { return x.Daemon == y.Daemon && x.Path == y.Path }
	return a.Project == b.Project && (same(a.Source, b.Source) && same(a.Target, b.Target) || same(a.Source, b.Target) && same(a.Target, b.Source))
}

func checkSettings(l proto.ProjectLink) error {
	switch {
	case !slices.Contains([]string{"", proto.SyncOneWay, proto.SyncTwoWay, proto.SyncReverse}, l.Direction):
		return fmt.Errorf("projectlinks: no direction %q: one-way, two-way or reverse", l.Direction)
	case !slices.Contains([]string{"", proto.DeletesMirror, proto.DeletesNever}, l.Deletes):
		return fmt.Errorf("projectlinks: deletes %q: mirror or never", l.Deletes)
	case !slices.Contains([]string{"", proto.LargeSkip, proto.LargeSource}, l.Large):
		return fmt.Errorf("projectlinks: large files %q: skip or source", l.Large)
	case l.Interval < 0 || l.Bandwidth < 0 || l.LargeMB < 0:
		return errors.New("projectlinks: interval, bandwidth and size can't be negative")
	case l.Auto && !l.Realtime && l.Interval > 0 && l.Interval < 5:
		return errors.New("projectlinks: sync at most every 5 seconds, or as files change")
	}
	for _, p := range append(slices.Clone(l.Include), l.Exclude...) {
		if _, err := filepath.Match(p, ""); err != nil {
			return fmt.Errorf("projectlinks: pattern %q: %w", p, err)
		}
	}
	return nil
}

// DeleteLink forgets link id; both copies stay.
func (s *Store) DeleteLink(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var all []proto.ProjectLink
	if err := s.read(LinksFile, &all); err != nil {
		return err
	}
	i := slices.IndexFunc(all, func(o proto.ProjectLink) bool { return o.ID == id })
	if i < 0 {
		return fmt.Errorf("projectlinks: no link %q", id)
	}
	if err := s.write(LinksFile, slices.Delete(all, i, i+1)); err != nil {
		return err
	}
	var st map[string]proto.ProjectLinkStatus
	if s.read(statusFile, &st) == nil && st != nil {
		delete(st, id)
		_ = s.write(statusFile, st)
	}
	_ = os.Remove(s.baseFile(id))
	return nil
}

// baseFile keeps what link id's copies had alike at its last sync.
func (s *Store) baseFile(id string) string {
	return filepath.Join(s.dir, "link-bases", filepath.Base(id)+".json")
}

// Base is what link id's copies had alike at its last sync, as reported
// here, nil for none.
func (s *Store) Base(id string) json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.baseFile(id))
	if err != nil || !json.Valid(data) {
		return nil
	}
	return data
}

// States lists the links of project (all with "") with their status here.
func (s *Store) States(project string) ([]proto.ProjectLinkState, error) {
	links, err := s.Links(project)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	var st map[string]proto.ProjectLinkStatus
	err = s.read(statusFile, &st)
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	out := make([]proto.ProjectLinkState, 0, len(links))
	for _, l := range links {
		out = append(out, proto.ProjectLinkState{ProjectLink: l, Status: st[l.ID]})
	}
	return out, nil
}

// Report records how a sync of a link went, here.
func (s *Store) Report(r proto.ProjectLinkReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := map[string]proto.ProjectLinkStatus{}
	if err := s.read(statusFile, &st); err != nil {
		return err
	}
	if st == nil {
		st = map[string]proto.ProjectLinkStatus{}
	}
	st[r.Link] = r.Status
	if err := s.write(statusFile, st); err != nil {
		return err
	}
	if len(r.Base) > 0 && json.Valid(r.Base) {
		if err := writeFile(s.baseFile(r.Link), r.Base); err != nil {
			return err
		}
	}
	if r.Entry.Kind == "" {
		return nil
	}
	return s.addHistory(r.Entry)
}

// AddHistory records a sync or a push.
func (s *Store) AddHistory(e proto.ProjectHistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addHistory(e)
}

func (s *Store) addHistory(e proto.ProjectHistoryEntry) error {
	var list []proto.ProjectHistoryEntry
	if err := s.read(historyFile, &list); err != nil {
		list = nil // a broken history starts again
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	list = append(list, e)
	if len(list) > historyKept {
		list = list[len(list)-historyKept:]
	}
	return s.write(historyFile, list)
}

// History lists the syncs and pushes of project (all with ""), the latest
// first.
func (s *Store) History(project string) ([]proto.ProjectHistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var list []proto.ProjectHistoryEntry
	if err := s.read(historyFile, &list); err != nil {
		return nil, err
	}
	out := []proto.ProjectHistoryEntry{}
	for i := len(list) - 1; i >= 0; i-- {
		if project == "" || list[i].Project == project {
			out = append(out, list[i])
		}
	}
	return out, nil
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
