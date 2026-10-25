package daemon

import "github.com/enowx/enowx/proto"

var noteIOHandlers = map[string]handler{
	proto.MethodNoteImport: with(func(c *conn, p proto.NoteImportParams) (any, error) {
		return c.srv.startNoteImport(p)
	}),
	proto.MethodNoteImportStatus: with(func(c *conn, p proto.NoteImportRef) (any, error) {
		j, err := c.srv.noteImport(p.ID)
		if err != nil {
			return nil, err
		}
		c.srv.noteIO.mu.Lock()
		defer c.srv.noteIO.mu.Unlock()
		return j.status, nil
	}),
	proto.MethodNoteImportCancel: with(func(c *conn, p proto.NoteImportRef) (any, error) {
		j, err := c.srv.noteImport(p.ID)
		if err != nil {
			return nil, err
		}
		j.cancel()
		return ok, nil
	}),
	proto.MethodNoteImports: bare(func(c *conn) (any, error) { return c.srv.noteImportList(), nil }),
	proto.MethodNoteStage: with(func(c *conn, p proto.NoteStage) (any, error) {
		return c.srv.stage(p)
	}),
	proto.MethodNoteExport: with(func(c *conn, p proto.NoteExportParams) (any, error) {
		return c.srv.exportNotes(p)
	}),
	proto.MethodNoteExportBytes: with(func(c *conn, p proto.NoteExportBytesParams) (any, error) {
		return c.srv.exportBytes(p)
	}),
}
