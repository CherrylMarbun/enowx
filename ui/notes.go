package ui

import (
	"encoding/json"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/notesview"
)

// pageNotes is the rail's Notes page: the connected host's notes.
const pageNotes = "notes"

// hostNotes is the connected host's notes, as last listed.
type hostNotes struct {
	host      string
	list      []proto.NoteInfo
	proposals []proto.NoteProposal // agents' changes waiting for the user
	groups    []proto.NoteGroup
	loaded    bool
	err       string
}

// notesOf is the connected host's notes, listed anew when the host
// changed; the notes event keeps them current.
func (a *App) notesOf() *hostNotes {
	if a.noteList == nil || a.noteList.host != a.host.ID {
		a.noteList = &hostNotes{host: a.host.ID}
		a.notes = notesview.View{}
		hn, cl := a.noteList, a.client
		if cl != nil {
			go func() {
				list, err := cl.Notes()
				props, _ := cl.NoteProposals() // none from a daemon of before
				groups, _ := cl.NoteGroups()
				a.onMain(func() {
					hn.list, hn.proposals, hn.groups, hn.loaded = list, props, groups, err == nil
					if err != nil {
						hn.err = client.Explain(err)
					}
				})
			}()
		}
	}
	return a.noteList
}

// notesEvent takes the host's notes as they changed.
func (a *App) notesEvent(data json.RawMessage) func() {
	var list []proto.NoteInfo
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return func() {
		if hn := a.noteList; hn != nil && hn.host == a.host.ID {
			hn.list, hn.loaded, hn.err = list, true, ""
		}
	}
}

func (a *App) notesPage(c *ui.Context) {
	a.notes.Draw(c, a.notesOptions())
}

func (a *App) notesOptions() notesview.Options {
	hn, cl := a.notesOf(), a.client
	// call runs a call to the host off the main thread, its result back on
	// it.
	call := func(fn func() error, done func(error)) {
		go func() {
			err := fn()
			a.onMain(func() { done(err) })
		}()
	}
	return notesview.Options{
		Host:   a.host.Name,
		Notes:  hn.list,
		Loaded: hn.loaded,
		Err:    hn.err,
		Load: func(id string, done func(proto.Note, error)) {
			var n proto.Note
			call(func() (err error) { n, err = cl.Note(id); return err }, func(err error) { done(n, err) })
		},
		Save: func(n proto.Note, done func(proto.NoteInfo, error)) {
			var info proto.NoteInfo
			call(func() (err error) { info, err = cl.SaveNote(n); return err }, func(err error) { done(info, err) })
		},
		Delete: func(id string, done func(error)) { call(func() error { return cl.DeleteNote(id) }, done) },
		Move: func(id, parent, before string, done func(error)) {
			call(func() error { return cl.MoveNote(id, parent, before) }, done)
		},
		Proposals: a.notesProposals(),
		Accept: func(id string, done func(proto.NoteInfo, error)) {
			var info proto.NoteInfo
			call(func() (err error) { info, err = cl.AcceptNoteProposal(id); return err }, func(err error) { done(info, err) })
		},
		Reject:    func(id string, done func(error)) { call(func() error { return cl.RejectNoteProposal(id) }, done) },
		Assistant: a.notesAssistant,
		Groups:    hn.groups,
		SaveGroup: func(from string, g proto.NoteGroup, done func(error)) {
			call(func() error { _, err := cl.SaveNoteGroup(from, g); return err }, done)
		},
		DeleteGroup: func(name string, done func(error)) {
			call(func() error { _, err := cl.DeleteNoteGroup(name); return err }, done)
		},
		MoveGroup: func(name, before string, done func(error)) {
			call(func() error { _, err := cl.MoveNoteGroup(name, before); return err }, done)
		},
		SetGroup: func(id, group string, done func(error)) {
			call(func() error { _, err := cl.SetNoteGroup(id, group); return err }, done)
		},
	}
}

// groupsEvent takes the groups of notes as they changed.
func (a *App) groupsEvent(data json.RawMessage) func() {
	var list []proto.NoteGroup
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return func() {
		if hn := a.noteList; hn != nil && hn.host == a.host.ID {
			hn.groups = list
		}
	}
}
