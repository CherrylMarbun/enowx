package daemon

import (
	"sync"

	"github.com/enowx/enowx/core/display"
	"github.com/enowx/enowx/proto"
)

// displays are the sessions' virtual displays (core/display): started
// with the session, its programs' windows open there, and the display's
// HTML5 client shows as a port of the session; stopped with it.
type displays struct {
	mu   sync.Mutex
	byID map[string]*display.Display
}

// start starts a display for session id.
func (d *displays) start(id string) (*display.Display, error) {
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
