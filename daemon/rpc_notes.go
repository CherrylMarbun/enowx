package daemon

import (
	"strings"

	"github.com/enowx/enowx/core/notes"
	"github.com/enowx/enowx/proto"
)

var noteHandlers = map[string]handler{
	proto.MethodNoteList: bare(func(c *conn) (any, error) { return c.srv.allNotes() }),
	proto.MethodNoteGet: with(func(c *conn, p proto.NoteRef) (any, error) {
		n, err := c.srv.notesFor(p.ID).Get(p.ID)
		n.NoteInfo, err = c.srv.inTeam(n.NoteInfo, err)
		return n, err
	}),
	proto.MethodNoteSave: with(func(c *conn, p proto.Note) (any, error) {
		st, err := c.srv.notesToSave(p)
		if err != nil {
			return nil, err
		}
		info, err := c.srv.inTeam(st.Save(p))
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
		if err := c.srv.notesFor(p.ID).Delete(p.ID); err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return ok, nil
	}),
	proto.MethodNoteMove: with(func(c *conn, p proto.NoteMoveParams) (any, error) {
		st := c.srv.notesFor(p.ID)
		if p.Parent != "" && c.srv.notesFor(p.Parent) != st {
			return nil, errOtherSpace
		}
		if err := st.Move(p.ID, p.Parent, p.Before); err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return ok, nil
	}),
	proto.MethodNoteProposals: bare(func(c *conn) (any, error) { return c.srv.allProposals() }),
	proto.MethodNotePropose: with(func(c *conn, p proto.NoteProposal) (any, error) {
		st, err := c.srv.notesToSave(p.Note)
		if err != nil {
			return nil, err
		}
		out, err := st.Propose(p.Note, p.Session)
		if err != nil {
			return nil, err
		}
		c.srv.proposalsChanged()
		return out, nil
	}),
	proto.MethodNoteAccept: with(func(c *conn, p proto.NoteRef) (any, error) {
		info, err := c.srv.inTeam(c.srv.proposalStore(p.ID).Accept(p.ID))
		if err != nil {
			return nil, err
		}
		c.srv.proposalsChanged()
		c.srv.groupsChanged(nil)
		return info, nil
	}),
	proto.MethodNoteReject: with(func(c *conn, p proto.NoteRef) (any, error) {
		if err := c.srv.proposalStore(p.ID).Reject(p.ID); err != nil {
			return nil, err
		}
		c.srv.proposalsChanged()
		return ok, nil
	}),
	proto.MethodNoteHistory: with(func(c *conn, p proto.NoteRef) (any, error) { return c.srv.notesFor(p.ID).History(p.ID) }),
	proto.MethodNoteVersion: with(func(c *conn, p proto.NoteVersionRef) (any, error) {
		return c.srv.notesFor(p.ID).Version(p.ID, p.Version)
	}),
	proto.MethodNoteRestore: with(func(c *conn, p proto.NoteVersionRef) (any, error) {
		info, err := c.srv.inTeam(c.srv.notesFor(p.ID).RestoreVersion(p.ID, p.Version))
		if err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return info, nil
	}),
	proto.MethodNoteAsset: with(func(c *conn, p proto.NoteAssetRef) (any, error) {
		data, err := c.srv.notesFor(assetNote(p.Ref)).Asset(p.Ref)
		return proto.NoteAsset{Data: data}, err
	}),
	proto.MethodNoteImage: with(func(c *conn, p proto.NoteImage) (any, error) {
		ref, err := c.srv.notesFor(p.ID).AddImage(p.ID, p.Name, p.Data)
		return proto.NoteAssetRef{Ref: ref}, err
	}),
	proto.MethodNoteChat:    with(func(c *conn, p proto.NoteChatParams) (any, error) { return c.srv.noteChat(p) }),
	proto.MethodNoteChatLog: with(func(c *conn, p proto.NoteRef) (any, error) { return c.srv.noteChatLog(p.ID) }),
	proto.MethodNoteSetAgent: with(func(c *conn, p proto.NoteSetAgentParams) (any, error) {
		info, err := c.srv.inTeam(c.srv.notesFor(p.ID).SetAgent(p.ID, p.Agent))
		if err != nil {
			return nil, err
		}
		c.srv.notesChanged()
		return info, nil
	}),
	proto.MethodNoteMoveSpace: with(func(c *conn, p proto.SpaceMoveParams) (any, error) {
		return c.srv.moveNoteSpace(p)
	}),
	proto.MethodNoteGroups: bare(func(c *conn) (any, error) { return c.srv.allGroups() }),
	proto.MethodNoteGroupSave: with(func(c *conn, p proto.NoteGroupSaveParams) (any, error) {
		st, err := c.srv.notesIn(p.Group.Team)
		if err != nil {
			return nil, err
		}
		p.Group.Team = ""
		return c.srv.groupsChanged(st.SaveGroup(p.From, p.Group))
	}),
	proto.MethodNoteGroupDelete: with(func(c *conn, p proto.NoteGroupRef) (any, error) {
		st, err := c.srv.notesIn(p.Team)
		if err != nil {
			return nil, err
		}
		return c.srv.groupsChanged(st.DeleteGroup(p.Name))
	}),
	proto.MethodNoteGroupMove: with(func(c *conn, p proto.NoteGroupMoveParams) (any, error) {
		st, err := c.srv.notesIn(p.Team)
		if err != nil {
			return nil, err
		}
		return c.srv.groupsChanged(st.MoveGroup(p.Name, p.Before))
	}),
	proto.MethodNoteIconPrefs: bare(func(c *conn) (any, error) { return c.srv.notes.IconPrefs() }),
	proto.MethodNoteIconPrefsSave: with(func(c *conn, p proto.NoteIconPrefs) (any, error) {
		return c.srv.notes.SaveIconPrefs(p)
	}),
	proto.MethodNoteSetGroup: with(func(c *conn, p proto.NoteSetGroupParams) (any, error) {
		info, err := c.srv.inTeam(c.srv.notesFor(p.ID).SetGroup(p.ID, p.Group))
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
	list, err := s.allGroups()
	if err != nil {
		return nil, err
	}
	s.broadcast(proto.EventNoteGroupsChanged, list)
	s.notesChanged()
	return list, nil
}

// proposalsChanged tells every app the proposals waiting now.
func (s *Server) proposalsChanged() {
	if list, err := s.allProposals(); err == nil {
		s.broadcast(proto.EventNoteProposalsChanged, list)
	}
}

// notesChanged tells every app the notes as they are now.
func (s *Server) notesChanged() {
	if list, err := s.allNotes(); err == nil {
		s.broadcast(proto.EventNotesChanged, list)
	}
	s.teamSync.kick() // a team's may have changed
}

// notesToSave is the store note n goes to: its own when it has an ID, its
// parent's when it has one, else its Team's space.
func (s *Server) notesToSave(n proto.Note) (*notes.Store, error) {
	switch {
	case n.ID != "":
		return s.notesFor(n.ID), nil
	case n.Parent != "":
		return s.notesFor(n.Parent), nil
	}
	return s.notesIn(n.Team)
}

// assetNote is the note an asset's ref ("assets/<note ID>/<file>") is of.
func assetNote(ref string) string {
	parts := strings.Split(ref, "/")
	if len(parts) == 3 && parts[0] == "assets" {
		return parts[1]
	}
	return ""
}
