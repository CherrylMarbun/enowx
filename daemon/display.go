package daemon

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/enowx/enowx/core/display"
	"github.com/enowx/enowx/proto"
)

// displays are the sessions' virtual displays (core/display): started
// with the session, its programs' windows open there, and the display's
// HTML5 client shows as a port of the session; stopped with it.
type displays struct {
	scale    atomic.Int32 // what displays start at: the last client's screen
	starting sync.Mutex   // one start at a time, so a session gets one display
	mu       sync.Mutex
	byID     map[string]*display.Display
}

// start starts a display for session id.
func (d *displays) start(id string) (*display.Display, error) {
	d.starting.Lock()
	defer d.starting.Unlock()
	disp, err := display.Start(int(d.scale.Load()))
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	if d.byID == nil {
		d.byID = map[string]*display.Display{}
	}
	d.byID[id] = disp
	d.mu.Unlock()
	return disp, nil
}

// byPort is the session whose display serves port, among ids.
func (d *displays) byPort(port int, ids []string) (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, id := range ids {
		if disp := d.byID[id]; disp != nil && disp.Port == port {
			return id, true
		}
	}
	return "", false
}

// stopDisplay stops the display on port of session id or a tab of it, and
// everything on it but the session's own program (an agent, a shell): the
// app, its dev server, its D-Bus.
func (s *Server) stopDisplay(p proto.DisplayStopParams) error {
	ids := []string{p.ID}
	for _, sess := range s.sessions.List() {
		if sess.Meta().Parent == p.ID {
			ids = append(ids, sess.ID())
		}
	}
	id, ok := s.displays.byPort(p.Port, ids)
	if !ok {
		return fmt.Errorf("no display on port %d", p.Port)
	}
	keep := s.sessionProgram(id)
	s.displays.mu.Lock()
	disp := s.displays.byID[id]
	delete(s.displays.byID, id)
	s.displays.mu.Unlock()
	if disp != nil {
		disp.Stop()
		for _, t := range displayTags(disp) {
			reapBut(t, keep)
		}
	}
	if sess, err := s.sessions.Get(id); err == nil {
		s.sessions.SetDisplay(sess, 0, 0)
		s.persist.update(sess, nil)
	}
	return nil
}

// stop stops session id's display, if it has one.
func (d *displays) stop(id string) {
	d.mu.Lock()
	disp := d.byID[id]
	delete(d.byID, id)
	d.mu.Unlock()
	if disp != nil {
		disp.Stop()
		for _, t := range displayTags(disp) {
			reap(t)
		}
	}
}

// displayTags mark what goes with disp: what was run for its app
// (display.EnvApp), and what Xpra started there, as its D-Bus, which no
// session started. Not everything with its DISPLAY: a session run with
// the display has it too, its agent's helpers with it.
func displayTags(disp *display.Display) []tag {
	n := ":" + strconv.Itoa(disp.Number)
	return []tag{{key: display.EnvApp, value: n}, {key: "DISPLAY", value: n, sessionless: true}}
}

// stopAll stops every display, as the daemon stops, and returns the tags
// of what ran on them, for the daemon to reap.
func (d *displays) stopAll() []tag {
	d.mu.Lock()
	all := d.byID
	d.byID = nil
	d.mu.Unlock()
	var tags []tag
	for _, disp := range all {
		disp.Stop()
		tags = append(tags, displayTags(disp)...)
	}
	return tags
}

// ports are the displays' ports, by session, as session ports.
func (d *displays) ports() map[string]proto.Port {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]proto.Port{}
	for id, disp := range d.byID {
		out[id] = proto.Port{Port: disp.Port, Addr: "127.0.0.1", Process: "display"}
	}
	return out
}

// ensure returns session id's display, starting one when it has none;
// started tells which.
func (d *displays) ensure(id string) (disp *display.Display, started bool, err error) {
	d.starting.Lock()
	d.mu.Lock()
	disp = d.byID[id]
	d.mu.Unlock()
	d.starting.Unlock()
	if disp != nil {
		return disp, false, nil
	}
	disp, err = d.start(id)
	return disp, err == nil, err
}

// sessionDisplay gives session id a display, for a desktop app it runs
// after it started (enowx display).
func (s *Server) sessionDisplay(id string) (proto.DisplayInfo, error) {
	sess, err := s.sessions.Get(id)
	if err != nil {
		return proto.DisplayInfo{}, err
	}
	disp, started, err := s.displays.ensure(id)
	if err != nil {
		return proto.DisplayInfo{}, err
	}
	if started {
		s.sessions.SetDisplay(sess, disp.Port, disp.Scale)
		s.persist.update(sess, nil)
	}
	return proto.DisplayInfo{Display: ":" + strconv.Itoa(disp.Number), Port: disp.Port, Env: disp.AppEnv()}, nil
}
