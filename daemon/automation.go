package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/automation"
	"github.com/enowx/enowx/core/project"
	"github.com/enowx/enowx/core/session"
	"github.com/enowx/enowx/proto"
)

// automations are the host's automations (proto.Automation), kept in
// automations.json; mu guards them. runAutomations starts those due.
type automations struct {
	mu   sync.Mutex
	file string
	list []proto.Automation
}

// automationPoll is how often due automations are looked for.
const automationPoll = 20 * time.Second

func (au *automations) load(file string) {
	au.file = file
	if data, err := os.ReadFile(file); err == nil {
		_ = json.Unmarshal(data, &au.list)
	}
}

func (au *automations) saveLocked() error {
	data, err := json.MarshalIndent(au.list, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(au.file, data)
}

func (s *Server) listAutomations() []proto.Automation {
	s.autos.mu.Lock()
	defer s.autos.mu.Unlock()
	return slices.Clone(s.autos.list)
}

// saveAutomation checks a and adds it, or replaces the one of its ID,
// keeping what the daemon recorded of its runs.
func (s *Server) saveAutomation(a proto.Automation) (proto.Automation, error) {
	a.Name, a.Prompt = strings.TrimSpace(a.Name), strings.TrimSpace(a.Prompt)
	sc, err := automation.Parse(a.Schedule)
	switch {
	case err != nil:
		return proto.Automation{}, err
	case a.Name == "" || a.Prompt == "":
		return proto.Automation{}, errors.New("automation: give it a name and a prompt")
	}
	if _, ok := agent.Find(a.Agent); !ok {
		return proto.Automation{}, fmt.Errorf("automation: unknown agent %q", a.Agent)
	}
	proj := project.Inspect(a.Project)
	if _, err := os.Stat(proj.Path); err != nil {
		return proto.Automation{}, fmt.Errorf("automation: %w", err)
	}
	if a.Branch && !proj.IsGit {
		return proto.Automation{}, fmt.Errorf("automation: %s is not a git repository, so it has no branches", proj.Name)
	}
	a.Project, a.Schedule = proj.Path, sc.String()
	au := &s.autos
	au.mu.Lock()
	defer au.mu.Unlock()
	i := slices.IndexFunc(au.list, func(o proto.Automation) bool { return o.ID == a.ID })
	if a.ID == "" || i < 0 {
		a.ID = session.NewID()
		a.LastRun, a.LastSession, a.LastError = time.Time{}, "", ""
	} else {
		old := au.list[i]
		a.LastRun, a.LastSession, a.LastError = old.LastRun, old.LastSession, old.LastError
	}
	a.NextRun = time.Time{}
	if a.Enabled {
		a.NextRun = sc.Next(time.Now())
	}
	if i >= 0 && au.list[i].ID == a.ID {
		au.list[i] = a
	} else {
		au.list = append(au.list, a)
	}
	return a, au.saveLocked()
}

func (s *Server) removeAutomation(id string) error {
	au := &s.autos
	au.mu.Lock()
	defer au.mu.Unlock()
	n := len(au.list)
	au.list = slices.DeleteFunc(au.list, func(a proto.Automation) bool { return a.ID == id })
	if len(au.list) == n {
		return fmt.Errorf("no automation %s", id)
	}
	return au.saveLocked()
}

// runAutomations starts the automations due, until done closes. One the
// daemon was down for runs once when it is back.
func (s *Server) runAutomations(done <-chan struct{}) {
	t := time.NewTicker(automationPoll)
	defer t.Stop()
	for {
		s.startDueAutomations(time.Now())
		select {
		case <-done:
			return
		case <-t.C:
		}
	}
}

func (s *Server) startDueAutomations(now time.Time) {
	s.autos.mu.Lock()
	var due []string
	for _, a := range s.autos.list {
		if a.Enabled && !a.NextRun.IsZero() && !now.Before(a.NextRun) {
			due = append(due, a.ID)
		}
	}
	s.autos.mu.Unlock()
	for _, id := range due {
		if _, err := s.runAutomation(id, now); err != nil {
			s.logger().Warn("an automation didn't start", "id", id, "err", err)
		}
	}
}

// runAutomation starts automation id's agent now, records the run and
// when the next is.
func (s *Server) runAutomation(id string, now time.Time) (*session.Session, error) {
	s.autos.mu.Lock()
	i := slices.IndexFunc(s.autos.list, func(a proto.Automation) bool { return a.ID == id })
	if i < 0 {
		s.autos.mu.Unlock()
		return nil, fmt.Errorf("no automation %s", id)
	}
	a := s.autos.list[i]
	s.autos.mu.Unlock()

	p := proto.SessionCreateParams{
		Project: a.Project, Agent: a.Agent, Prompt: a.Prompt, Name: a.Name + " · " + now.Format("Jan 2 15:04"),
		SkipPermissions: a.SkipPermissions, BranchMode: proto.BranchNone,
	}
	if a.Branch {
		p.BranchMode, p.Branch = proto.BranchNew, "automation/"+slug(a.Name)+"-"+now.Format("20060102-1504")
	}
	sess, err := s.createSession(p)

	s.autos.mu.Lock()
	defer s.autos.mu.Unlock()
	if i = slices.IndexFunc(s.autos.list, func(a proto.Automation) bool { return a.ID == id }); i >= 0 {
		r := &s.autos.list[i]
		r.LastRun, r.LastError = now, ""
		if err != nil {
			r.LastError = err.Error()
		} else {
			r.LastSession = sess.ID()
		}
		if sc, perr := automation.Parse(r.Schedule); perr == nil && r.Enabled {
			r.NextRun = sc.Next(now)
		}
		_ = s.autos.saveLocked()
	}
	return sess, err
}

// slug makes a name fit a branch: lowercase words joined by dashes.
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func automationsFile(dir string) string { return filepath.Join(dir, "automations.json") }
