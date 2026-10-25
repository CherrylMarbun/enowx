package projectlinks

import (
	"errors"
	"slices"
	"time"

	"github.com/enowx/enowx/core/autopush"
	"github.com/enowx/enowx/proto"
)

// DefaultPushInterval is how often a project's work is pushed unless the
// user says.
const DefaultPushInterval = 30 * time.Minute

// MinPushInterval is the shortest interval between auto pushes.
const MinPushInterval = time.Minute

// Interval is how often a's work is pushed.
func Interval(a proto.AutopushSettings) time.Duration {
	if a.Interval <= 0 {
		return DefaultPushInterval
	}
	return max(time.Duration(a.Interval)*time.Second, MinPushInterval)
}

// Autopushes lists the projects' auto push settings.
func (s *Store) Autopushes() ([]proto.AutopushSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var list []proto.AutopushSettings
	if err := s.read(AutopushFile, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// Autopush is project's auto push settings, ok false when it has none.
func (s *Store) Autopush(project string) (proto.AutopushSettings, bool, error) {
	list, err := s.Autopushes()
	if err != nil {
		return proto.AutopushSettings{}, false, err
	}
	i := slices.IndexFunc(list, func(a proto.AutopushSettings) bool { return a.Project == project })
	if i < 0 {
		return proto.AutopushSettings{}, false, nil
	}
	return list[i], true, nil
}

// SetAutopush records a's settings for its project.
func (s *Store) SetAutopush(a proto.AutopushSettings) error {
	if !ValidID(a.Project) {
		return errors.New("projectlinks: auto push needs the project's ID")
	}
	if a.Branch == "" {
		a.Branch = autopush.DefaultBranch
	}
	if a.Interval < 0 {
		return errors.New("projectlinks: the interval can't be negative")
	}
	a.Path = "" // each host knows its own folder
	s.mu.Lock()
	defer s.mu.Unlock()
	var list []proto.AutopushSettings
	if err := s.read(AutopushFile, &list); err != nil {
		return err
	}
	if i := slices.IndexFunc(list, func(o proto.AutopushSettings) bool { return o.Project == a.Project }); i >= 0 {
		list[i] = a
	} else {
		list = append(list, a)
	}
	return s.write(AutopushFile, list)
}

// PushState is how a project's auto pushes went on this host.
type PushState struct {
	LastRun    time.Time `json:"lastRun,omitempty"`
	LastPush   time.Time `json:"lastPush,omitempty"`
	Commit     string    `json:"commit,omitempty"` // the last snapshot
	Pushed     string    `json:"pushed,omitempty"` // the last pushed
	Files      int       `json:"files,omitempty"`
	Err        string    `json:"err,omitempty"`
	Visibility string    `json:"visibility,omitempty"`
	PRURL      string    `json:"prURL,omitempty"`
}

// PushStates are the projects' push states here, by project ID.
func (s *Store) PushStates() map[string]PushState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := map[string]PushState{}
	if s.read(pushStateFile, &st) != nil || st == nil {
		return map[string]PushState{}
	}
	return st
}

// SetPushState records project's push state here.
func (s *Store) SetPushState(project string, ps PushState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := map[string]PushState{}
	if s.read(pushStateFile, &st) != nil || st == nil {
		st = map[string]PushState{}
	}
	st[project] = ps
	return s.write(pushStateFile, st)
}
