package client

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/enowx/enowx/proto"
)

// ImportNotes starts importing notes from a file or folder on the host.
func (c *Client) ImportNotes(p proto.NoteImportParams) (proto.NoteImportStatus, error) {
	var st proto.NoteImportStatus
	err := c.Call(proto.MethodNoteImport, p, &st)
	return st, err
}

// NoteImport says how an import goes.
func (c *Client) NoteImport(id string) (proto.NoteImportStatus, error) {
	var st proto.NoteImportStatus
	err := c.Call(proto.MethodNoteImportStatus, proto.NoteImportRef{ID: id}, &st)
	return st, err
}

// CancelNoteImport stops an import, taking back the notes it made.
func (c *Client) CancelNoteImport(id string) error {
	return c.Call(proto.MethodNoteImportCancel, proto.NoteImportRef{ID: id}, nil)
}

// NoteImports lists the imports running and done lately.
func (c *Client) NoteImports() ([]proto.NoteImportStatus, error) {
	var list []proto.NoteImportStatus
	err := c.Call(proto.MethodNoteImports, nil, &list)
	return list, err
}

// StageNotes puts the local file at path on the host, in parts, for an
// import, and returns where it is there.
func (c *Client) StageNotes(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, proto.UploadPart)
	remote := ""
	for {
		n, err := io.ReadFull(f, buf)
		if n > 0 || remote == "" {
			var out proto.NoteStaged
			if cerr := c.Call(proto.MethodNoteStage, proto.NoteStage{Name: filepath.Base(path), Path: remote, Data: buf[:n]}, &out); cerr != nil {
				return "", cerr
			}
			remote = out.Path
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return remote, nil
		}
		if err != nil {
			return "", err
		}
	}
}

// ExportNotes writes notes as a file: at p.Out on the host, or, without
// it, where ReadExport reads it.
func (c *Client) ExportNotes(p proto.NoteExportParams) (proto.NoteExported, error) {
	var out proto.NoteExported
	err := c.Call(proto.MethodNoteExport, p, &out)
	return out, err
}

// ReadExport copies an export made without Out to w.
func (c *Client) ReadExport(path string, w io.Writer) error {
	var off int64
	for {
		var part proto.FileBytes
		if err := c.Call(proto.MethodNoteExportBytes, proto.NoteExportBytesParams{Path: path, Offset: off}, &part); err != nil {
			return err
		}
		if _, err := w.Write(part.Data); err != nil {
			return err
		}
		off += int64(len(part.Data))
		if len(part.Data) == 0 || off >= part.Size {
			return nil
		}
	}
}
