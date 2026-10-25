package canvas

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/enowx/enowx/proto"
)

func (s *Store) commentsFile(id string) string { return filepath.Join(s.dir, id, "comments.json") }

// Comments is canvas id's comments, the oldest first.
func (s *Store) Comments(id string) ([]proto.CanvasComment, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	list := []proto.CanvasComment{}
	if err := readJSON(s.commentsFile(id), &list); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("canvas: comments: %w", err)
	}
	return list, nil
}

// Comment adds p.Comment to canvas p.ID, or resolves, reopens or deletes
// the comment with its ID, and returns the comments.
func (s *Store) Comment(p proto.CanvasCommentParams) ([]proto.CanvasComment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.Comments(p.ID)
	if err != nil {
		return nil, err
	}
	in := p.Comment
	if p.Op == proto.CanvasCommentAdd {
		in.Text = strings.TrimSpace(in.Text)
		if in.Text == "" {
			return nil, errors.New("canvas: a comment says something")
		}
		in.ID, in.Created, in.Resolved = "c"+newID(4), time.Now().UTC(), false
		list = append(list, in)
		return list, writeJSON(s.commentsFile(p.ID), list)
	}
	i := slices.IndexFunc(list, func(x proto.CanvasComment) bool { return x.ID == in.ID })
	if i < 0 {
		return nil, fmt.Errorf("canvas: no comment %q", in.ID)
	}
	switch p.Op {
	case proto.CanvasCommentResolve:
		list[i].Resolved = true
	case proto.CanvasCommentReopen:
		list[i].Resolved = false
	case proto.CanvasCommentDelete:
		list = slices.Delete(list, i, i+1)
	default:
		return nil, fmt.Errorf("canvas: no comment change %q", p.Op)
	}
	return list, writeJSON(s.commentsFile(p.ID), list)
}
