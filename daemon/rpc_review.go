package daemon

import (
	"slices"
	"strings"

	"github.com/enowx/enowx/core/review"
	"github.com/enowx/enowx/proto"
)

// reviewHandlers run git and gh, which can take a while (a push, a large
// diff): they are slowMethods.
var reviewHandlers = map[string]handler{
	proto.MethodProjectConflicts: with(func(c *conn, p proto.ProjectRef) (any, error) {
		return c.srv.conflicts(p.Path)
	}),
	proto.MethodReviewStatus: with(func(c *conn, p proto.ReviewRef) (any, error) {
		w, base, err := c.srv.reviewOf(p.Session, p.Base)
		if err != nil {
			return nil, err
		}
		return review.Status(w, base)
	}),
	proto.MethodReviewDiff: with(func(c *conn, p proto.ReviewRef) (any, error) {
		w, base, err := c.srv.reviewOf(p.Session, p.Base)
		if err != nil {
			return nil, err
		}
		return review.Diff(w.Dir, base)
	}),
	proto.MethodReviewCommit: with(func(c *conn, p proto.CommitParams) (any, error) {
		w, base, err := c.srv.reviewOf(p.Session, "")
		if err != nil {
			return nil, err
		}
		if err := review.Commit(w.Dir, p.Message); err != nil {
			return nil, err
		}
		return review.Status(w, base)
	}),
	proto.MethodReviewPush: with(func(c *conn, p proto.ReviewRef) (any, error) {
		w, base, err := c.srv.reviewOf(p.Session, p.Base)
		if err != nil {
			return nil, err
		}
		if err := review.Push(w.Dir); err != nil {
			return nil, err
		}
		return review.Status(w, base)
	}),
	proto.MethodReviewPR: with(func(c *conn, p proto.PRParams) (any, error) {
		w, base, err := c.srv.reviewOf(p.Session, p.Base)
		if err != nil {
			return nil, err
		}
		url, err := review.PullRequest(w.Dir, base, p.Title, p.Body)
		if err != nil {
			return nil, err
		}
		return proto.PullRequest{URL: url}, nil
	}),
	proto.MethodReviewMerge: with(func(c *conn, p proto.ReviewRef) (any, error) {
		w, base, err := c.srv.reviewOf(p.Session, p.Base)
		if err != nil {
			return nil, err
		}
		if err := review.Merge(w, base); err != nil {
			return nil, err
		}
		c.srv.broadcast(proto.EventProjectsChanged, c.srv.projects.List())
		return review.Status(w, base)
	}),
	proto.MethodReviewDiscard: with(func(c *conn, p proto.DiscardParams) (any, error) {
		w, _, err := c.srv.reviewOf(p.Session, "")
		if err != nil {
			return nil, err
		}
		// The sessions in the worktree go with it.
		for _, sess := range c.srv.sessions.List() {
			if sess.Info().Dir == w.Dir {
				if err := c.srv.deleteSession(sess.ID()); err != nil {
					return nil, err
				}
				c.srv.broadcast(proto.EventSessionRemoved, proto.SessionRef{ID: sess.ID()})
			}
		}
		return ok, review.Discard(c.srv.WorktreeRoot, w, p.DeleteBranch)
	}),
}

// reviewOf is where session id runs, and the base to compare it with:
// base, else the session's default.
func (s *Server) reviewOf(id, base string) (review.Where, string, error) {
	sess, err := s.sessions.Get(id)
	if err != nil {
		return review.Where{}, "", err
	}
	info := sess.Info()
	w := review.Where{Dir: info.Dir, Project: info.Project, Worktree: info.Worktree}
	switch base {
	case "":
		base = review.DefaultBase(w)
	case "HEAD":
		base = "" // the last commit
	}
	return w, base, nil
}

// conflicts finds the files two running sessions of project both change,
// each against its own base. Sessions in one folder share their changes,
// so they don't conflict with each other.
func (s *Server) conflicts(project string) ([]proto.Conflict, error) {
	type changes struct {
		id, dir string
		touched map[string]review.Touch
	}
	var all []changes
	seen := map[string]bool{}
	for _, sess := range s.sessions.List() {
		info := sess.Info()
		if info.Project != project || info.Exited || info.Dir == "" || seen[info.Dir] {
			continue
		}
		seen[info.Dir] = true
		w := review.Where{Dir: info.Dir, Project: info.Project, Worktree: info.Worktree}
		touched, err := review.Touched(w.Dir, review.DefaultBase(w))
		if err != nil || len(touched) == 0 {
			continue
		}
		all = append(all, changes{info.ID, info.Dir, touched})
	}
	var out []proto.Conflict
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			a, b := all[i], all[j]
			for file, ta := range a.touched {
				if tb, ok := b.touched[file]; ok {
					out = append(out, proto.Conflict{File: file, Sessions: []string{a.id, b.id}, Lines: review.Overlap(ta, tb)})
				}
			}
		}
	}
	slices.SortFunc(out, func(x, y proto.Conflict) int {
		if x.File != y.File {
			return strings.Compare(x.File, y.File)
		}
		return strings.Compare(x.Sessions[0]+x.Sessions[1], y.Sessions[0]+y.Sessions[1])
	})
	return out, nil
}
