package client

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/enowx/enowx/proto"
)

// ListFiles lists the folder path of session id's folder ("" for itself).
func (c *Client) ListFiles(id, path string) (proto.FileListing, error) {
	var l proto.FileListing
	err := c.Call(proto.MethodFileList, proto.FileRef{Session: id, Path: path}, &l)
	return l, err
}

// ReadFile reads the file path of session id's folder; when it last
// changed at unless (set), the answer only says Unchanged.
func (c *Client) ReadFile(id, path string, unless time.Time) (proto.FileContent, error) {
	var f proto.FileContent
	err := c.Call(proto.MethodFileRead, proto.FileRef{Session: id, Path: path, Unless: unless}, &f)
	return f, err
}

// Ports lists the TCP ports session id's processes listen on.
func (c *Client) Ports(id string) (proto.SessionPorts, error) {
	var p proto.SessionPorts
	err := c.Call(proto.MethodSessionPorts, proto.SessionRef{ID: id}, &p)
	return p, err
}

// Upload puts the local file at path on session id's host, in parts, and
// returns where it is there.
func (c *Client) Upload(id, path string) (string, error) {
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
			var out proto.FileUploaded
			if cerr := c.Call(proto.MethodFileUpload, proto.FileUpload{Session: id, Name: filepath.Base(path), Path: remote, Data: buf[:n]}, &out); cerr != nil {
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
