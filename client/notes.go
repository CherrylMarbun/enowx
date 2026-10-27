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

// NoteProposals lists the changes agents proposed, waiting for the user.
func (c *Client) NoteProposals() ([]proto.NoteProposal, error) {
	var list []proto.NoteProposal
	err := c.Call(proto.MethodNoteProposals, nil, &list)
	return list, err
}

// ProposeNote proposes n as a change (a new note when n.ID is ""), made by
// session.
func (c *Client) ProposeNote(n proto.Note, session string) (proto.NoteProposal, error) {
	var p proto.NoteProposal
	err := c.Call(proto.MethodNotePropose, proto.NoteProposal{Note: n, Session: session}, &p)
	return p, err
}

// AcceptNoteProposal makes a proposed change.
func (c *Client) AcceptNoteProposal(id string) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteAccept, proto.NoteRef{ID: id}, &info)
	return info, err
}

// RejectNoteProposal forgets a proposed change.
func (c *Client) RejectNoteProposal(id string) error {
	return c.Call(proto.MethodNoteReject, proto.NoteRef{ID: id}, nil)
}

// NoteGroups lists the groups of notes, in order.
func (c *Client) NoteGroups() ([]proto.NoteGroup, error) {
	var list []proto.NoteGroup
	err := c.Call(proto.MethodNoteGroups, nil, &list)
	return list, err
}

// SaveNoteGroup makes a group (from "") or changes group from.
func (c *Client) SaveNoteGroup(from string, g proto.NoteGroup) ([]proto.NoteGroup, error) {
	var list []proto.NoteGroup
	err := c.Call(proto.MethodNoteGroupSave, proto.NoteGroupSaveParams{From: from, Group: g}, &list)
	return list, err
}

// DeleteNoteGroup deletes a group; its notes stay, in none.
func (c *Client) DeleteNoteGroup(name string) ([]proto.NoteGroup, error) {
	var list []proto.NoteGroup
	err := c.Call(proto.MethodNoteGroupDelete, proto.NoteGroupRef{Name: name}, &list)
	return list, err
}

// MoveNoteGroup puts a group before another ("" for last).
func (c *Client) MoveNoteGroup(name, before string) ([]proto.NoteGroup, error) {
	var list []proto.NoteGroup
	err := c.Call(proto.MethodNoteGroupMove, proto.NoteGroupMoveParams{Name: name, Before: before}, &list)
	return list, err
}

// SetNoteGroup puts a note in a group ("" for none), at the top level.
func (c *Client) SetNoteGroup(id, group string) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteSetGroup, proto.NoteSetGroupParams{ID: id, Group: group}, &info)
	return info, err
}

// NoteHistory lists a note's earlier versions, the latest first.
func (c *Client) NoteHistory(id string) ([]proto.NoteVersion, error) {
	var list []proto.NoteVersion
	err := c.Call(proto.MethodNoteHistory, proto.NoteRef{ID: id}, &list)
	return list, err
}

// NoteVersion reads an earlier version of a note.
func (c *Client) NoteVersion(id, version string) (proto.Note, error) {
	var n proto.Note
	err := c.Call(proto.MethodNoteVersion, proto.NoteVersionRef{ID: id, Version: version}, &n)
	return n, err
}

// RestoreNote brings an earlier version of a note back.
func (c *Client) RestoreNote(id, version string) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteRestore, proto.NoteVersionRef{ID: id, Version: version}, &info)
	return info, err
}

// NoteIconPrefs is the icons used lately, the favorites and the libraries
// the icon picker shows.
func (c *Client) NoteIconPrefs() (proto.NoteIconPrefs, error) {
	var p proto.NoteIconPrefs
	err := c.Call(proto.MethodNoteIconPrefs, nil, &p)
	return p, err
}

// SaveNoteIconPrefs keeps the favorites and libraries of p.
func (c *Client) SaveNoteIconPrefs(p proto.NoteIconPrefs) (proto.NoteIconPrefs, error) {
	var out proto.NoteIconPrefs
	err := c.Call(proto.MethodNoteIconPrefsSave, p, &out)
	return out, err
}

// NoteChat asks a note's agent something, in its conversation running or
// a new one.
func (c *Client) NoteChat(p proto.NoteChatParams) (proto.NoteChatLog, error) {
	var log proto.NoteChatLog
	err := c.Call(proto.MethodNoteChat, p, &log)
	return log, err
}

// NoteChatLog reads a note's conversations with its agent, on any host.
func (c *Client) NoteChatLog(id string) (proto.NoteChatLog, error) {
	var log proto.NoteChatLog
	err := c.Call(proto.MethodNoteChatLog, proto.NoteRef{ID: id}, &log)
	return log, err
}

// SetNoteAgent picks the agent a note's page talks to.
func (c *Client) SetNoteAgent(id, agent string) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteSetAgent, proto.NoteSetAgentParams{ID: id, Agent: agent}, &info)
	return info, err
}

// NoteAsset reads a file a note shows, as its text names it.
func (c *Client) NoteAsset(ref string) ([]byte, error) {
	var a proto.NoteAsset
	err := c.Call(proto.MethodNoteAsset, proto.NoteAssetRef{Ref: ref}, &a)
	return a.Data, err
}

// AddNoteImage uploads an image and returns its Markdown reference.
func (c *Client) AddNoteImage(id, name string, data []byte) (string, error) {
	var out proto.NoteAssetRef
	err := c.Call(proto.MethodNoteImage, proto.NoteImage{ID: id, Name: name, Data: data}, &out)
	return out.Ref, err
}
