package client

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

// ListFiles lists the folder path of session id's folder ("" for itself).
func (c *Client) ListFiles(id, path string) (proto.FileListing, error) {
	var l proto.FileListing
	err := c.Call(proto.MethodFileList, proto.FileRef{Session: id, Path: path}, &l)
	return l, err
}

// ReadFile reads the file path of session id's folder, or at an absolute
// path ("/tmp/shot.png") anywhere on the host; when it last changed at
// unless (set), the answer only says Unchanged.
func (c *Client) ReadFile(id, path string, unless time.Time) (proto.FileContent, error) {
	var f proto.FileContent
	ref := proto.FileRef{Session: id, Path: path, Unless: unless, Abs: strings.HasPrefix(path, "/")}
	err := c.Call(proto.MethodFileRead, ref, &f)
	return f, err
}

// ReadBytes reads the whole file path of session id's folder (an absolute
// path on the host when abs), as it is, up to limit bytes. Daemons before
// file.bytes answer with what file.read sends: text up to a megabyte, and
// images.
func (c *Client) ReadBytes(id, path string, abs bool, limit int64) ([]byte, error) {
	var data []byte
	for {
		var part proto.FileBytes
		err := c.Call(proto.MethodFileBytes, proto.FileBytesParams{Session: id, Path: path, Abs: abs, Offset: int64(len(data))}, &part)
		if IsUnknownMethod(err) {
			f, err := c.ReadFile(id, path, time.Time{})
			if f.Data != nil {
				return f.Data, err
			}
			return []byte(f.Text), err
		}
		if err != nil {
			return nil, err
		}
		if part.Size > limit {
			return nil, fmt.Errorf("client: %s is larger than %d bytes", path, limit)
		}
		data = append(data, part.Data...)
		if len(part.Data) == 0 || int64(len(data)) >= part.Size {
			return data, nil
		}
	}
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
