package daemon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/enowx/enowx/core/files"
	"github.com/enowx/enowx/proto"
)

// Files dropped on a session from the app's machine land in
// <daemon dir>/uploads/<session>/, out of the session's folder so its git
// status stays clean, and go with the session.

func (s *Server) uploadDir(id string) string {
	return filepath.Join(filepath.Dir(s.WorktreeRoot), "uploads", id)
}

// upload writes a part of a file sent for session p.Session: with its
// uploads, or into a folder of its own folder (p.Into).
func (s *Server) upload(p proto.FileUpload) (proto.FileUploaded, error) {
	if _, err := s.sessions.Get(p.Session); err != nil {
		return proto.FileUploaded{}, err
	}
	dir := s.uploadDir(p.Session)
	if p.Into != "" {
		root, err := s.sessionDir(p.Session)
		if err != nil {
			return proto.FileUploaded{}, err
		}
		if dir, err = files.Folder(root, p.Into); err != nil {
			return proto.FileUploaded{}, fmt.Errorf("upload: %w", err)
		}
	}
	path := p.Path
	flags := os.O_WRONLY | os.O_APPEND
	if path == "" {
		name := filepath.Base(strings.ReplaceAll(p.Name, "\\", "/"))
		if name == "." || name == "/" || name == "" {
			return proto.FileUploaded{}, errors.New("upload: the file has no name")
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return proto.FileUploaded{}, fmt.Errorf("upload: %w", err)
		}
		path = freeName(dir, name)
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	} else if filepath.Dir(path) != dir {
		return proto.FileUploaded{}, fmt.Errorf("upload: %s isn't where the file was going", path)
	}
	mode := os.FileMode(0o600) // the uploads are the user's alone
	if p.Into != "" {
		mode = 0o644 // a project's file, as others are
	}
	f, err := os.OpenFile(path, flags, mode)
	if err != nil {
		return proto.FileUploaded{}, fmt.Errorf("upload: %w", err)
	}
	if _, err := f.Write(p.Data); err != nil {
		f.Close()
		return proto.FileUploaded{}, fmt.Errorf("upload: %w", err)
	}
	if err := f.Close(); err != nil {
		return proto.FileUploaded{}, fmt.Errorf("upload: %w", err)
	}
	return proto.FileUploaded{Path: path, Placed: p.Into != ""}, nil
}

// freeName is name in dir, numbered when a file has it already.
func freeName(dir, name string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = stem + "-" + strconv.Itoa(i) + ext
		}
		p := filepath.Join(dir, candidate)
		if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
			return p
		}
	}
}
