package client

import "github.com/enowx/enowx/proto"

// Notes lists the host's notes, without their bodies.
func (c *Client) Notes() ([]proto.NoteInfo, error) {
	var list []proto.NoteInfo
	err := c.Call(proto.MethodNoteList, nil, &list)
	return list, err
}

// Note reads a note with its body.
func (c *Client) Note(id string) (proto.Note, error) {
	var n proto.Note
	err := c.Call(proto.MethodNoteGet, proto.NoteRef{ID: id}, &n)
	return n, err
}

// SaveNote writes a note: a new one when it has no ID.
func (c *Client) SaveNote(n proto.Note) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteSave, n, &info)
	return info, err
}

// DeleteNote deletes a note and the notes under it.
func (c *Client) DeleteNote(id string) error {
	return c.Call(proto.MethodNoteDelete, proto.NoteRef{ID: id}, nil)
}

// MoveNote puts a note under parent ("" for the top), before its child
// before ("" for last).
func (c *Client) MoveNote(id, parent, before string) error {
	return c.Call(proto.MethodNoteMove, proto.NoteMoveParams{ID: id, Parent: parent, Before: before}, nil)
}
