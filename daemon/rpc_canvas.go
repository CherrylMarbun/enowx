package daemon

import (
	"path/filepath"

	"github.com/enowx/enowx/proto"
)

var canvasHandlers = map[string]handler{
	proto.MethodCanvasList: bare(func(c *conn) (any, error) {
		list, err := c.srv.canvases.List()
		for i := range list {
			list[i].Paused = c.srv.canvasState.isPaused(list[i].ID)
		}
		return list, err
	}),
	proto.MethodCanvasGet: with(func(c *conn, p proto.CanvasRef) (any, error) {
		cv, err := c.srv.canvases.Get(p.ID)
		if err != nil {
			return nil, err
		}
		return c.srv.withState(cv), nil
	}),
	proto.MethodCanvasCreate: with(func(c *conn, p proto.CanvasCreateParams) (any, error) {
		cv, err := c.srv.canvases.Create(p)
		if err != nil {
			return nil, err
		}
		c.srv.canvasesChanged()
		return cv, nil
	}),
	proto.MethodCanvasDelete: with(func(c *conn, p proto.CanvasRef) (any, error) {
		if err := c.srv.canvases.Delete(p.ID); err != nil {
			return nil, err
		}
		c.srv.canvasState.setPaused(p.ID, false)
		c.srv.canvasesChanged()
		return ok, nil
	}),
	proto.MethodCanvasEdit: with(func(c *conn, p proto.CanvasEdit) (any, error) {
		return c.srv.canvasEdit(p)
	}),
	proto.MethodCanvasHistory: with(func(c *conn, p proto.CanvasRef) (any, error) {
		return c.srv.canvases.History(p.ID)
	}),
	proto.MethodCanvasVersion: with(func(c *conn, p proto.CanvasVersionRef) (any, error) {
		return c.srv.canvases.Version(p.ID, p.N)
	}),
	proto.MethodCanvasUndo: with(func(c *conn, p proto.CanvasRef) (any, error) {
		cv, err := c.srv.canvases.Undo(p.ID)
		return c.srv.canvasShown(cv, "undo", err)
	}),
	proto.MethodCanvasRedo: with(func(c *conn, p proto.CanvasRef) (any, error) {
		cv, err := c.srv.canvases.Redo(p.ID)
		return c.srv.canvasShown(cv, "redo", err)
	}),
	proto.MethodCanvasRestore: with(func(c *conn, p proto.CanvasVersionRef) (any, error) {
		cv, err := c.srv.canvases.Restore(p.ID, p.N)
		return c.srv.canvasShown(cv, "restore", err)
	}),
	proto.MethodCanvasComments: with(func(c *conn, p proto.CanvasRef) (any, error) {
		return c.srv.canvases.Comments(p.ID)
	}),
	proto.MethodCanvasComment: with(func(c *conn, p proto.CanvasCommentParams) (any, error) {
		list, err := c.srv.canvases.Comment(p)
		if err != nil {
			return nil, err
		}
		if cv, err := c.srv.canvases.Get(p.ID); err == nil {
			c.srv.broadcast(proto.EventCanvasChanged, proto.CanvasChange{Canvas: c.srv.withState(cv), Op: "comment", Element: p.Comment.Element})
		}
		return list, nil
	}),
	proto.MethodCanvasControl: with(func(c *conn, p proto.CanvasControl) (any, error) {
		return c.srv.canvasControl(p)
	}),
	proto.MethodCanvasAsset: with(func(c *conn, p proto.CanvasAsset) (any, error) {
		name, err := c.srv.canvases.AddAsset(p.ID, p.Name, p.Data)
		if err != nil {
			return nil, err
		}
		dir, _ := c.srv.canvases.Dir(p.ID)
		return proto.CanvasAsset{ID: p.ID, Name: name, Path: filepath.Join(dir, "assets", name)}, nil
	}),
	proto.MethodCanvasAssetGet: with(func(c *conn, p proto.CanvasAsset) (any, error) {
		data, err := c.srv.canvases.Asset(p.ID, p.Name)
		if err != nil {
			return nil, err
		}
		return proto.CanvasAsset{ID: p.ID, Name: p.Name, Data: data}, nil
	}),
	proto.MethodCanvasRender: with(func(c *conn, p proto.CanvasRenderParams) (any, error) {
		return c.srv.canvasRender(p)
	}),
	proto.MethodCanvasChat: with(func(c *conn, p proto.CanvasChatParams) (any, error) {
		return c.srv.canvasChatChanged(c.srv.canvases.Chat(p))
	}),
	proto.MethodCanvasChatLog: with(func(c *conn, p proto.CanvasRef) (any, error) {
		return c.srv.canvases.ChatLog(p.ID)
	}),
	proto.MethodCanvasChatMark: with(func(c *conn, p proto.CanvasChatMark) (any, error) {
		return c.srv.canvasChatChanged(c.srv.canvases.ChatMark(p))
	}),
	proto.MethodCanvasRenderReply: with(func(c *conn, p proto.CanvasRenderReply) (any, error) {
		c.srv.canvasState.reply(p)
		return ok, nil
	}),
}
