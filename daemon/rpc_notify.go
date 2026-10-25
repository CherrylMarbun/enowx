package daemon

import (
	"fmt"
	"slices"

	"github.com/enowx/enowx/core/notify"
	"github.com/enowx/enowx/proto"
)

// Notifications: the settings the apps play sounds by, kept in the synced
// settings, and sounds agents ask the apps to play.

var notifyHandlers = map[string]handler{
	proto.MethodNotifyGet: bare(func(c *conn) (any, error) { return c.srv.notify.Get() }),
	proto.MethodNotifySet: with(func(c *conn, p proto.NotifySettings) (any, error) {
		st, err := c.srv.notify.Set(p)
		if err != nil {
			return nil, err
		}
		c.srv.broadcast(proto.EventNotifySettings, st)
		return st, nil
	}),
	proto.MethodNotifySound: with(func(c *conn, p proto.NotifySoundParams) (any, error) {
		if !slices.Contains(notify.Kinds, p.Kind) {
			return nil, fmt.Errorf("notify: no kind of sound %q: done, input or error", p.Kind)
		}
		c.srv.broadcast(proto.EventNotifySound, p)
		return ok, nil
	}),
	proto.MethodNotifySoundSave: with(func(c *conn, p proto.NotifySoundFile) (any, error) {
		return ok, c.srv.notify.SaveSound(p.Name, p.Data)
	}),
	proto.MethodNotifySoundRead: with(func(c *conn, p proto.NotifySoundFile) (any, error) {
		data, err := c.srv.notify.Sound(p.Name)
		if err != nil {
			return nil, err
		}
		return proto.NotifySoundFile{Name: p.Name, Data: data}, nil
	}),
}

// notifySynced tells the apps the settings when a sync brought new ones.
func (s *Server) notifySynced(paths []string) {
	if !slices.Contains(paths, "settings/notify.json") {
		return
	}
	if st, err := s.notify.Get(); err == nil {
		s.broadcast(proto.EventNotifySettings, st)
	}
}
