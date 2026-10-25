package proto

import "time"

// Notes: the user's pages on a host, nested like a notebook's, their
// bodies Markdown; agents read and write them too.
const (
	MethodNoteList   = "note.list"   // → []NoteInfo
	MethodNoteGet    = "note.get"    // NoteRef → Note
	MethodNoteSave   = "note.save"   // Note → NoteInfo (a new one without ID)
	MethodNoteDelete = "note.delete" // NoteRef: it and the pages under it
	MethodNoteMove   = "note.move"   // NoteMoveParams
)

// EventNotesChanged carries the full []NoteInfo after a change.
const EventNotesChanged = "notes.changed"

// NoteInfo is a page without its body. Pages under another name it as
// Parent; Order sorts a page's children.
type NoteInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Icon   string `json:"icon,omitempty"` // an emoji
	Parent string `json:"parent,omitempty"`
	// Group is the group a top-level note is in (NoteGroup.Name), ""
	// for none; pages under it are in its group.
	Group   string    `json:"group,omitempty"`
	Order   int       `json:"order"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

// Note is a page with its body.
type Note struct {
	NoteInfo
	Body string `json:"body"`
}

// NoteRef names a page.
type NoteRef struct {
	ID string `json:"id"`
}

// NoteMoveParams puts page ID under Parent ("" for the top), before page
// Before of it, or last with Before "".
type NoteMoveParams struct {
	ID     string `json:"id"`
	Parent string `json:"parent,omitempty"`
	Before string `json:"before,omitempty"`
}

// An agent asked to help with notes from the Notes page proposes its
// changes rather than making them: the user sees each against the note
// and accepts or rejects it.
const (
	MethodNoteProposals = "note.proposals" // → []NoteProposal
	MethodNotePropose   = "note.propose"   // NoteProposal (its Note and Session) → NoteProposal
	MethodNoteAccept    = "note.accept"    // NoteRef (a proposal's ID) → NoteInfo
	MethodNoteReject    = "note.reject"    // NoteRef (a proposal's ID)
)

// EventNoteProposalsChanged carries the full []NoteProposal after a change.
const EventNoteProposalsChanged = "notes.proposals"

// NoteProposal is a change an agent proposes: Note is the note as it would
// be (its ID "" for a new one, under Note.Parent); Before is the note's
// body when proposed, which the change is shown against.
type NoteProposal struct {
	ID      string    `json:"id"`
	Note    Note      `json:"note"`
	Before  string    `json:"before,omitempty"`
	Session string    `json:"session,omitempty"` // the agent's
	Created time.Time `json:"created"`
}

// EnvNotesPropose, set in a session's environment, makes its enowx-app
// notes propose changes rather than make them: the Notes page's agents.
const EnvNotesPropose = "ENOWX_NOTES_PROPOSE"

// Groups of notes, like folders: a top-level note is in one or none.
const (
	MethodNoteGroups      = "note.groups"       // → []NoteGroup
	MethodNoteGroupSave   = "note.group.save"   // NoteGroupSaveParams → []NoteGroup
	MethodNoteGroupDelete = "note.group.delete" // NoteGroupRef → []NoteGroup (its notes left without a group)
	MethodNoteGroupMove   = "note.group.move"   // NoteGroupMoveParams → []NoteGroup
	MethodNoteSetGroup    = "note.setGroup"     // NoteSetGroupParams → NoteInfo
)

// EventNoteGroupsChanged carries the full []NoteGroup after a change.
const EventNoteGroupsChanged = "notes.groups"

// NoteGroup is a group of notes, in the user's order.
type NoteGroup struct {
	Name   string `json:"name"`
	Icon   string `json:"icon,omitempty"`  // an emoji
	Color  string `json:"color,omitempty"` // #rrggbb
	Folded bool   `json:"folded,omitempty"`
}

// NoteGroupSaveParams makes a group, or changes group From ("" makes
// one): its name (its notes follow), icon, color and folding.
type NoteGroupSaveParams struct {
	From  string    `json:"from,omitempty"`
	Group NoteGroup `json:"group"`
}

// NoteGroupRef names a group.
type NoteGroupRef struct {
	Name string `json:"name"`
}

// NoteGroupMoveParams moves group Name before group Before, or last.
type NoteGroupMoveParams struct {
	Name   string `json:"name"`
	Before string `json:"before,omitempty"`
}

// NoteSetGroupParams puts note ID in Group ("" for none), at the top level.
type NoteSetGroupParams struct {
	ID    string `json:"id"`
	Group string `json:"group,omitempty"`
}

// A note's earlier versions, kept on each host, to see and bring back.
const (
	MethodNoteHistory = "note.history" // NoteRef → []NoteVersion, the latest first
	MethodNoteVersion = "note.version" // NoteVersionRef → Note
	MethodNoteRestore = "note.restore" // NoteVersionRef → NoteInfo: that version back
)

// NoteVersion is an earlier version of a note.
type NoteVersion struct {
	ID    string    `json:"id"`
	Time  time.Time `json:"time"`
	Title string    `json:"title"`
	Size  int       `json:"size"` // its body's, in bytes
}

// NoteVersionRef names version Version of note ID.
type NoteVersionRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}
