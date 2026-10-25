package client

import "github.com/enowx/enowx/proto"

// AttachNote attaches a note to projects, besides those it has.
func (c *Client) AttachNote(id string, projects []proto.NoteProject) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteAttach, proto.NoteAttachParams{ID: id, Projects: projects}, &info)
	return info, err
}

// DetachNote detaches a note from projects refs, or from all.
func (c *Client) DetachNote(id string, refs []string, all bool) (proto.NoteInfo, error) {
	var info proto.NoteInfo
	err := c.Call(proto.MethodNoteDetach, proto.NoteDetachParams{ID: id, Refs: refs, All: all}, &info)
	return info, err
}

// ProjectContext is a project's context, here or on another host.
func (c *Client) ProjectContext(p proto.ProjectContextParams) (proto.ProjectContext, error) {
	var out proto.ProjectContext
	err := c.Call(proto.MethodProjectContext, p, &out)
	return out, err
}

// ReadProjectFile reads a project's file, read-only.
func (c *Client) ReadProjectFile(p proto.ProjectReadParams) (proto.ProjectFile, error) {
	var out proto.ProjectFile
	err := c.Call(proto.MethodProjectRead, p, &out)
	return out, err
}
