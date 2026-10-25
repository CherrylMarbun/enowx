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
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Icon    string    `json:"icon,omitempty"` // an emoji
	Parent  string    `json:"parent,omitempty"`
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
