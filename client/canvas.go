package client

import "github.com/enowx/enowx/proto"

// Canvases lists the host's canvases, the latest changed first.
func (c *Client) Canvases() ([]proto.CanvasInfo, error) {
	var list []proto.CanvasInfo
	err := c.Call(proto.MethodCanvasList, nil, &list)
	return list, err
}

// Canvas reads a canvas with its frames.
func (c *Client) Canvas(id string) (proto.Canvas, error) {
	var cv proto.Canvas
	err := c.Call(proto.MethodCanvasGet, proto.CanvasRef{ID: id}, &cv)
	return cv, err
}

// CreateCanvas makes a canvas.
func (c *Client) CreateCanvas(p proto.CanvasCreateParams) (proto.Canvas, error) {
	var cv proto.Canvas
	err := c.Call(proto.MethodCanvasCreate, p, &cv)
	return cv, err
}

// DeleteCanvas deletes a canvas with its versions, comments and assets.
func (c *Client) DeleteCanvas(id string) error {
	return c.Call(proto.MethodCanvasDelete, proto.CanvasRef{ID: id}, nil)
}

// EditCanvas makes a change to a canvas, which every app sees at once.
func (c *Client) EditCanvas(e proto.CanvasEdit) (proto.Canvas, error) {
	var cv proto.Canvas
	err := c.Call(proto.MethodCanvasEdit, e, &cv)
	return cv, err
}

// CanvasHistory lists a canvas's versions.
func (c *Client) CanvasHistory(id string) (proto.CanvasHistory, error) {
	var h proto.CanvasHistory
	err := c.Call(proto.MethodCanvasHistory, proto.CanvasRef{ID: id}, &h)
	return h, err
}

// CanvasVersion reads a canvas as it was at version n.
func (c *Client) CanvasVersion(id string, n int) (proto.Canvas, error) {
	var cv proto.Canvas
	err := c.Call(proto.MethodCanvasVersion, proto.CanvasVersionRef{ID: id, N: n}, &cv)
	return cv, err
}

// UndoCanvas shows a canvas's version before the one shown.
func (c *Client) UndoCanvas(id string) (proto.Canvas, error) {
	var cv proto.Canvas
	err := c.Call(proto.MethodCanvasUndo, proto.CanvasRef{ID: id}, &cv)
	return cv, err
}

// RedoCanvas shows the version undone last.
func (c *Client) RedoCanvas(id string) (proto.Canvas, error) {
	var cv proto.Canvas
	err := c.Call(proto.MethodCanvasRedo, proto.CanvasRef{ID: id}, &cv)
	return cv, err
}

// RestoreCanvas makes a canvas as it was at version n its newest version.
func (c *Client) RestoreCanvas(id string, n int) (proto.Canvas, error) {
	var cv proto.Canvas
	err := c.Call(proto.MethodCanvasRestore, proto.CanvasVersionRef{ID: id, N: n}, &cv)
	return cv, err
}

// CanvasComments lists a canvas's comments.
func (c *Client) CanvasComments(id string) ([]proto.CanvasComment, error) {
	var list []proto.CanvasComment
	err := c.Call(proto.MethodCanvasComments, proto.CanvasRef{ID: id}, &list)
	return list, err
}

// CommentCanvas adds, resolves, reopens or deletes a comment.
func (c *Client) CommentCanvas(p proto.CanvasCommentParams) ([]proto.CanvasComment, error) {
	var list []proto.CanvasComment
	err := c.Call(proto.MethodCanvasComment, p, &list)
	return list, err
}

// ControlCanvas pauses, resumes or stops the agents designing a canvas.
func (c *Client) ControlCanvas(id, action string) (proto.CanvasInfo, error) {
	var info proto.CanvasInfo
	err := c.Call(proto.MethodCanvasControl, proto.CanvasControl{ID: id, Action: action}, &info)
	return info, err
}

// AddCanvasAsset keeps a file a canvas shows; it returns its name there.
func (c *Client) AddCanvasAsset(id, name string, data []byte) (string, error) {
	var out proto.CanvasAsset
	err := c.Call(proto.MethodCanvasAsset, proto.CanvasAsset{ID: id, Name: name, Data: data}, &out)
	return out.Name, err
}

// RenderCanvas renders a frame as a PNG or a PDF.
func (c *Client) RenderCanvas(p proto.CanvasRenderParams) (proto.CanvasRendered, error) {
	var out proto.CanvasRendered
	err := c.Call(proto.MethodCanvasRender, p, &out)
	return out, err
}

// CanvasRenderReply answers the daemon's request to render.
func (c *Client) CanvasRenderReply(r proto.CanvasRenderReply) error {
	return c.Call(proto.MethodCanvasRenderReply, r, nil)
}
