package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/core/projectctx"
	"github.com/enowx/enowx/proto"
)

// Notes attached to projects, and the projects' context for the agents
// writing those notes: read here when the project is here, through the
// app from its host otherwise, and kept as a snapshot in the synced notes
// for when that host is out of reach.

var noteProjectHandlers = map[string]handler{
	proto.MethodNoteAttach: with(func(c *conn, p proto.NoteAttachParams) (any, error) {
		info, err := c.srv.notes.Attach(p.ID, p.Projects)
		if err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return info, nil
	}),
	proto.MethodNoteDetach: with(func(c *conn, p proto.NoteDetachParams) (any, error) {
		info, err := c.srv.notes.Detach(p.ID, p.Refs, p.All)
		if err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return info, nil
	}),
	proto.MethodProjectContext: with(func(c *conn, p proto.ProjectContextParams) (any, error) {
		return c.srv.projectContext(p)
	}),
	proto.MethodProjectRead: with(func(c *conn, p proto.ProjectReadParams) (any, error) {
		if p.Host == "" || p.Host == c.srv.daemonID {
			return projectctx.Read(homePath(p.Path), p.File)
		}
		raw, err := c.srv.askApp("project.read", p, time.Minute)
		if err != nil {
			return nil, err
		}
		var f proto.ProjectFile
		return f, json.Unmarshal(raw, &f)
	}),
}

// projectContext is the context of a project here or, through the app, on
// another host; the snapshot last kept when that host can't be reached.
func (s *Server) projectContext(p proto.ProjectContextParams) (proto.ProjectContext, error) {
	var c proto.ProjectContext
	var err error
	if p.Host == "" || p.Host == s.daemonID {
		c, err = projectctx.Of(homePath(p.Path))
		if err == nil {
			c.Host, c.HostName = s.daemonID, s.userdata.Name
			c.Ref = notes.ProjectRef(c.Remote, c.Host, c.Path)
		}
	} else {
		var raw json.RawMessage
		if raw, err = s.askApp("project.context", p, time.Minute); err == nil {
			err = json.Unmarshal(raw, &c)
		}
	}
	ref := p.Ref
	if ref == "" {
		ref = c.Ref
	}
	if err != nil {
		if old, ok := s.contextSnapshot(ref); ok {
			old.Cached = true
			return old, nil
		}
		return c, err
	}
	s.keepContext(ref, c)
	return c, nil
}

// contextFile is where the snapshot of project ref's context is kept, in
// the notes (synced to every host).
func (s *Server) contextFile(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return filepath.Join(s.dataDir, "notes", "context", hex.EncodeToString(sum[:8])+".json")
}

func (s *Server) keepContext(ref string, c proto.ProjectContext) {
	if ref == "" {
		return
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	file := s.contextFile(ref)
	if old, err := os.ReadFile(file); err == nil {
		var was proto.ProjectContext
		if json.Unmarshal(old, &was) == nil {
			was.Taken, was.Cached = c.Taken, false
			if same, _ := json.MarshalIndent(was, "", "  "); string(same) == string(data) {
				return // unchanged: no sync for nothing
			}
		}
	}
	_ = os.MkdirAll(filepath.Dir(file), 0o700)
	_ = os.WriteFile(file+".tmp", data, 0o600)
	_ = os.Rename(file+".tmp", file)
}

func (s *Server) contextSnapshot(ref string) (proto.ProjectContext, bool) {
	var c proto.ProjectContext
	if ref == "" {
		return c, false
	}
	data, err := os.ReadFile(s.contextFile(ref))
	if err != nil || json.Unmarshal(data, &c) != nil {
		return c, false
	}
	return c, true
}

// homePath is p with a leading ~ for the home folder.
func homePath(p string) string {
	if out, err := expandHome(p); err == nil {
		return out
	}
	return p
}
