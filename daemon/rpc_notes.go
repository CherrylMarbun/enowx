package daemon

import "github.com/enowx/enowx/proto"

var noteHandlers = map[string]handler{
	proto.MethodNoteList: bare(func(c *conn) (any, error) { return c.srv.notes.List() }),
	proto.MethodNoteGet: with(func(c *conn, p proto.NoteRef) (any, error) {
		return c.srv.notes.Get(p.ID)
	}),
	proto.MethodNoteSave: with(func(c *conn, p proto.Note) (any, error) {
		info, err := c.srv.notes.Save(p)
		if err != nil {
			return nil, err
		}
		if p.ID == "" && info.Group != "" {
			c.srv.groupsChanged(nil) // made with a group it may have made
		} else {
			c.srv.notesChanged()
		}
		return info, nil
	}),
	proto.MethodNoteDelete: with(func(c *conn, p proto.NoteRef) (any, error) {
		if err := c.srv.notes.Delete(p.ID); err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return ok, nil
	}),
	proto.MethodNoteMove: with(func(c *conn, p proto.NoteMoveParams) (any, error) {
		if err := c.srv.notes.Move(p.ID, p.Parent, p.Before); err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return ok, nil
	}),
	proto.MethodNoteProposals: bare(func(c *conn) (any, error) { return c.srv.notes.Proposals() }),
	proto.MethodNotePropose: with(func(c *conn, p proto.NoteProposal) (any, error) {
		out, err := c.srv.notes.Propose(p.Note, p.Session)
		if err != nil {
			return nil, err
		}
		c.srv.proposalsChanged()
		return out, nil
	}),
	proto.MethodNoteAccept: with(func(c *conn, p proto.NoteRef) (any, error) {
		info, err := c.srv.notes.Accept(p.ID)
		if err != nil {
			return nil, err
		}
		c.srv.proposalsChanged()
		c.srv.groupsChanged(nil)
		return info, nil
	}),
	proto.MethodNoteReject: with(func(c *conn, p proto.NoteRef) (any, error) {
		if err := c.srv.notes.Reject(p.ID); err != nil {
			return nil, err
		}
		c.srv.proposalsChanged()
		return ok, nil
	}),
	proto.MethodNoteGroups: bare(func(c *conn) (any, error) { return c.srv.notes.Groups() }),
	proto.MethodNoteGroupSave: with(func(c *conn, p proto.NoteGroupSaveParams) (any, error) {
		return c.srv.groupsChanged(c.srv.notes.SaveGroup(p.From, p.Group))
	}),
	proto.MethodNoteGroupDelete: with(func(c *conn, p proto.NoteGroupRef) (any, error) {
		return c.srv.groupsChanged(c.srv.notes.DeleteGroup(p.Name))
	}),
	proto.MethodNoteGroupMove: with(func(c *conn, p proto.NoteGroupMoveParams) (any, error) {
		return c.srv.groupsChanged(c.srv.notes.MoveGroup(p.Name, p.Before))
	}),
	proto.MethodNoteIconPrefs: bare(func(c *conn) (any, error) { return c.srv.notes.IconPrefs() }),
	proto.MethodNoteIconPrefsSave: with(func(c *conn, p proto.NoteIconPrefs) (any, error) {
		return c.srv.notes.SaveIconPrefs(p)
	}),
	proto.MethodNoteSetGroup: with(func(c *conn, p proto.NoteSetGroupParams) (any, error) {
		info, err := c.srv.notes.SetGroup(p.ID, p.Group)
		if err != nil {
			return nil, err
		}
		c.srv.groupsChanged(nil)
		return info, nil
	}),
}

// groupsChanged tells every app the groups, and the notes they hold, after
// err-less changes to them; it answers with the groups.
func (s *Server) groupsChanged(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	list, err := s.notes.Groups()
	if err != nil {
		return nil, err
	}
	s.broadcast(proto.EventNoteGroupsChanged, list)
	s.notesChanged()
	return list, nil
}

// proposalsChanged tells every app the proposals waiting now.
func (s *Server) proposalsChanged() {
	if list, err := s.notes.Proposals(); err == nil {
		s.broadcast(proto.EventNoteProposalsChanged, list)
	}
}

// notesChanged tells every app the notes as they are now.
func (s *Server) notesChanged() {
	if list, err := s.notes.List(); err == nil {
		s.broadcast(proto.EventNotesChanged, list)
	}
}
