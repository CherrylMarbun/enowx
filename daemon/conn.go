package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/core/envsync"
	"github.com/enowx/enowx/core/folders"
	"github.com/enowx/enowx/core/project"
	"github.com/enowx/enowx/core/worktree"
	"github.com/enowx/enowx/proto"
)

// conn is one client connection. Its read loop runs on the goroutine that
// called serve; writes come from many goroutines and are serialized by wmu.
type conn struct {
	srv *Server
	nc  net.Conn

	wmu sync.Mutex

	mu         sync.Mutex
	streams    map[uint32]*stream
	nextStream uint32
	closed     bool
}

func newConn(s *Server, nc net.Conn) *conn {
	return &conn{srv: s, nc: nc, streams: map[uint32]*stream{}}
}

func (c *conn) serve() error {
	defer c.close()
	if err := c.handshake(); err != nil {
		return err
	}
	for {
		f, err := proto.ReadFrame(c.nc)
		if err != nil {
			return err
		}
		switch f.Type {
		case proto.TypeRequest:
			var req proto.Request
			if err := json.Unmarshal(f.Payload, &req); err != nil {
				return fmt.Errorf("bad request: %w", err)
			}
			// Requests run in order on this goroutine, so a resize sent
			// after an attach always finds its stream.
			c.reply(req.ID, c.handle(req))
		case proto.TypeInput:
			if st := c.stream(f.Channel); st != nil {
				st.input(f.Payload)
			}
		default:
			return fmt.Errorf("unexpected %s frame", f.Type)
		}
	}
}

func (c *conn) handshake() error {
	f, err := proto.ReadFrame(c.nc)
	if err != nil {
		return err
	}
	if f.Type != proto.TypeHello {
		return fmt.Errorf("want hello, got %s", f.Type)
	}
	var h proto.Hello
	if err := json.Unmarshal(f.Payload, &h); err != nil {
		return fmt.Errorf("bad hello: %w", err)
	}
	if h.Protocol != proto.Version {
		return fmt.Errorf("protocol %d not supported (daemon speaks %d)", h.Protocol, proto.Version)
	}
	payload, _ := json.Marshal(proto.Hello{Protocol: proto.Version, Agent: "enowxd/" + Version})
	c.send(proto.Frame{Type: proto.TypeHello, Payload: payload})
	return nil
}

type result struct {
	value any
	err   error
}

func (c *conn) handle(req proto.Request) result {
	s := c.srv
	switch req.Method {
	case proto.MethodDaemonInfo:
		return result{value: s.info()}

	case proto.MethodAgentList:
		refreshLoginPath(s.logger())
		return result{value: agent.List()}

	case proto.MethodProjectList:
		return result{value: s.projects.List()}

	case proto.MethodProjectAdd:
		var p proto.ProjectRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		info, err := s.projects.Add(p.Path)
		if err != nil {
			return result{err: err}
		}
		s.broadcast(proto.EventProjectsChanged, s.projects.List())
		return result{value: info}

	case proto.MethodProjectRemove:
		var p proto.ProjectRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		if err := s.projects.Remove(p.Path); err != nil {
			return result{err: err}
		}
		s.broadcast(proto.EventProjectsChanged, s.projects.List())
		return result{value: struct{}{}}

	case proto.MethodProjectInspect:
		var p proto.ProjectRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		return result{value: project.Inspect(p.Path)}

	case proto.MethodSessionCreate:
		var p proto.SessionCreateParams
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		sess, err := s.createSession(p)
		if err != nil {
			return result{err: err}
		}
		return result{value: sess.Info()}

	case proto.MethodSessionList:
		list := s.sessions.List()
		infos := make([]proto.SessionInfo, 0, len(list))
		for _, sess := range list {
			infos = append(infos, sess.Info())
		}
		return result{value: infos}

	case proto.MethodSessionAttach:
		var p proto.SessionRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		id, err := c.attach(p.ID)
		if err != nil {
			return result{err: err}
		}
		return result{value: proto.AttachResult{Stream: id}}

	case proto.MethodSessionDetach:
		var p proto.StreamRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		c.detach(p.Stream)
		return result{value: struct{}{}}

	case proto.MethodSessionResize:
		var p proto.ResizeParams
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		st := c.stream(p.Stream)
		if st == nil {
			return result{err: fmt.Errorf("unknown stream %d", p.Stream)}
		}
		st.resize(p)
		return result{value: struct{}{}}

	case proto.MethodSessionClear:
		var p proto.SessionRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		sess, err := s.sessions.Get(p.ID)
		if err != nil {
			return result{err: err}
		}
		sess.Terminal().ClearScreen()
		return result{value: struct{}{}}

	case proto.MethodSessionKill:
		var p proto.SessionRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		if err := s.deleteSession(p.ID); err != nil {
			return result{err: err}
		}
		s.broadcast(proto.EventSessionRemoved, p)
		return result{value: struct{}{}}

	case proto.MethodFolderList:
		var p proto.FolderListParams
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return result{err: err}
		}
		listing, err := folders.List(home, p)
		if err != nil {
			return result{err: err}
		}
		return result{value: listing}

	case proto.MethodFolderMake:
		var p proto.FolderRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return result{err: err}
		}
		dir, err := folders.Make(home, p.Path)
		if err != nil {
			return result{err: err}
		}
		return result{value: proto.FolderRef{Path: dir}}

	case proto.MethodEnvScan:
		home, err := os.UserHomeDir()
		if err != nil {
			return result{err: err}
		}
		return result{value: envsync.Scan(home)}

	case proto.MethodEnvExport:
		var p proto.EnvItemRef
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return result{err: err}
		}
		payload, err := envsync.Export(home, p.ID)
		if err != nil {
			return result{err: err}
		}
		return result{value: payload}

	case proto.MethodEnvImport:
		var p proto.EnvPayload
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return result{err: err}
		}
		res, err := envsync.Import(home, p)
		if err != nil {
			return result{err: err}
		}
		return result{value: res}

	case proto.MethodWorktreeCreate:
		var p proto.WorktreeCreateParams
		if err := decode(req.Params, &p); err != nil {
			return result{err: err}
		}
		info, err := worktree.Create(s.WorktreeRoot, p)
		if err != nil {
			return result{err: err}
		}
		return result{value: info}
	}
	return result{err: fmt.Errorf("unknown method %q", req.Method)}
}

func (c *conn) reply(id uint64, r result) {
	resp := proto.Response{ID: id}
	if r.err != nil {
		resp.Error = r.err.Error()
	} else if raw, err := json.Marshal(r.value); err != nil {
		resp.Error = err.Error()
	} else {
		resp.Result = raw
	}
	payload, _ := json.Marshal(resp)
	c.send(proto.Frame{Type: proto.TypeResponse, Payload: payload})
}

func (c *conn) attach(id string) (uint32, error) {
	sess, err := c.srv.sessions.Get(id)
	if err != nil {
		return 0, err
	}
	m, err := sess.Terminal().NewMirror()
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		m.Close()
		return 0, errors.New("connection closed")
	}
	c.nextStream++
	st := newStream(c, c.nextStream, sess, m)
	c.streams[st.id] = st
	c.mu.Unlock()
	go st.pump()
	return st.id, nil
}

func (c *conn) detach(id uint32) {
	c.mu.Lock()
	st := c.streams[id]
	delete(c.streams, id)
	c.mu.Unlock()
	if st != nil {
		st.stop()
	}
}

func (c *conn) stream(id uint32) *stream {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streams[id]
}

// send writes a frame; errors surface in the read loop as the socket closes.
func (c *conn) send(f proto.Frame) {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	if err := proto.WriteFrame(c.nc, f); err != nil {
		_ = c.nc.Close()
	}
}

func (c *conn) close() {
	c.mu.Lock()
	c.closed = true
	streams := c.streams
	c.streams = map[uint32]*stream{}
	c.mu.Unlock()
	for _, st := range streams {
		st.stop()
	}
	_ = c.nc.Close()
}

func decode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return errors.New("missing params")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("bad params: %w", err)
	}
	return nil
}
