package daemon

import (
	"context"
	"sync"
	"time"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

// Asking an agent for its commands takes seconds, so each list is kept and
// asked for again in the background once it is older than commandsFor.
const (
	commandsFor     = 10 * time.Minute
	commandsTimeout = 45 * time.Second
)

// commandCache keeps each agent's commands by folder. Its map is guarded
// by mu; a list is read on a goroutine of its own, one per folder at a
// time.
type commandCache struct {
	mu   sync.Mutex
	byID map[string]*commandList
}

type commandList struct {
	list    []proto.AgentCommand
	read    time.Time
	reading bool
}

// commands returns session id's agent's commands as last read, and reads
// them again when there are none yet or they are old.
func (s *Server) commands(p proto.SessionRef) (proto.AgentCommands, error) {
	sess, err := s.sessions.Get(p.ID)
	if err != nil {
		return proto.AgentCommands{}, err
	}
	info := sess.Info()
	key := info.Agent + "\x00" + info.Dir
	c := &s.commandCache
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		c.byID = map[string]*commandList{}
	}
	l := c.byID[key]
	if l == nil {
		l = &commandList{}
		c.byID[key] = l
	}
	if !l.reading && time.Since(l.read) > commandsFor {
		l.reading = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), commandsTimeout)
			defer cancel()
			list, err := agent.Commands(ctx, info.Agent, info.Dir)
			if err != nil {
				s.logger().Warn("could not list the agent's commands", "agent", info.Agent, "err", err)
			}
			c.mu.Lock()
			defer c.mu.Unlock()
			if list != nil || l.list == nil {
				l.list = list
			}
			l.read, l.reading = time.Now(), false
		}()
	}
	return proto.AgentCommands{Commands: l.list, Loading: l.reading && l.list == nil}, nil
}
