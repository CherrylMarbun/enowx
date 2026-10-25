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
		c.srv.notesChanged()
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
		c.srv.notesChanged()
		return info, nil
	}),
	proto.MethodNoteReject: with(func(c *conn, p proto.NoteRef) (any, error) {
		if err := c.srv.notes.Reject(p.ID); err != nil {
			return nil, err
		}
		c.srv.proposalsChanged()
		return ok, nil
	}),
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
