package daemon

import (
	"strconv"
	"sync"

	"github.com/enowx/enowx/core/display"
	"github.com/enowx/enowx/proto"
)

// displays are the sessions' virtual displays (core/display): started
// with the session, its programs' windows open there, and the display's
// HTML5 client shows as a port of the session; stopped with it.
type displays struct {
	starting sync.Mutex // one start at a time, so a session gets one display
	mu       sync.Mutex
	byID     map[string]*display.Display
}

// start starts a display for session id.
func (d *displays) start(id string) (*display.Display, error) {
	d.starting.Lock()
	defer d.starting.Unlock()
	disp, err := display.Start()
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

// stop stops session id's display, if it has one.
func (d *displays) stop(id string) {
	d.mu.Lock()
	disp := d.byID[id]
	delete(d.byID, id)
	d.mu.Unlock()
	disp.Stop()
}

// stopAll stops every display, as the daemon stops.
func (d *displays) stopAll() {
	d.mu.Lock()
	all := d.byID
	d.byID = nil
	d.mu.Unlock()
	for _, disp := range all {
		disp.Stop()
	}
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
		s.sessions.SetDisplay(sess, disp.Port)
		s.persist.update(sess, nil)
	}
	return proto.DisplayInfo{Display: ":" + strconv.Itoa(disp.Number), Port: disp.Port}, nil
}
