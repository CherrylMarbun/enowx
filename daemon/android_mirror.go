package daemon

import (
	"context"
	"fmt"
	"net"

	"github.com/enowx/enowx/core/android"
	"github.com/enowx/enowx/proto"
)

// A device's screen, shown in the app: scrcpy's server started on the
// device, its video carried to the app as a tunnel, input sent back by
// android.mirror.input. One screen a device: showing it again replaces
// the one shown.

// androidMirror starts device p.Serial's screen, carried by a tunnel of c
// that stops it once closed.
func (c *conn) androidMirror(p proto.AndroidMirrorParams) (proto.TunnelInfo, error) {
	sdk, err := c.srv.androidSDK()
	if err != nil {
		return proto.TunnelInfo{}, err
	}
	m, err := sdk.Mirror(context.Background(), p.Serial, android.MirrorOptions{MaxSize: p.MaxSize})
	if err != nil {
		return proto.TunnelInfo{}, err
	}
	a := &c.srv.android
	a.mu.Lock()
	old := a.mirrors[p.Serial]
	if a.mirrors == nil {
		a.mirrors = map[string]*android.Mirror{}
	}
	a.mirrors[p.Serial] = m
	a.mu.Unlock()
	if old != nil {
		old.Close()
	}
	id, err := c.attachTunnel(mirrorConn{m.Video, func() {
		a.mu.Lock()
		if a.mirrors[p.Serial] == m {
			delete(a.mirrors, p.Serial)
		}
		a.mu.Unlock()
		m.Close()
	}})
	return proto.TunnelInfo{Stream: id}, err
}

// mirrorConn is a screen's video, whose closing stops the screen.
type mirrorConn struct {
	net.Conn
	stop func()
}

func (m mirrorConn) Close() error {
	m.stop()
	return nil
}

func (s *Server) androidMirrorInput(p proto.AndroidMirrorInput) error {
	s.android.mu.Lock()
	m := s.android.mirrors[p.Serial]
	s.android.mu.Unlock()
	if m == nil {
		return fmt.Errorf("android: %s's screen isn't shown", p.Serial)
	}
	switch p.Kind {
	case "touch":
		return m.Touch(p.Action, p.X, p.Y, p.Width, p.Height)
	case "key":
		return m.Press(p.Key)
	case "text":
		return m.Text(p.Text)
	}
	return fmt.Errorf("android: no input %q", p.Kind)
}
