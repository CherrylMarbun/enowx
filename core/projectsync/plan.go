// Package projectsync brings two copies of a project on two hosts to the
// same state, both ways: it compares what each has now with what both had
// at the last sync (their base), to tell which side changed what, and
// plans what each side sends the other. See docs/transfer.md.
package projectsync

import (
	"cmp"
	"slices"
	"time"

	"github.com/enowx/enowx/proto"
)

// Side is one of the two copies: Here is the connected host's.
type Side int

const (
	Here Side = iota
	There
)

// Other is the other side.
func (s Side) Other() Side { return 1 - s }

// Key is a session's key: what it is on both hosts, as a transfer keeps
// when it was made and its agent.
func Key(created time.Time, agent string) string {
	return created.UTC().Format(time.RFC3339Nano) + "|" + agent
}

// File is a file to bring to one side: From's version wins, which is no
// file at all when Delete. Root is "" for the project's folder, else the
// key of the worktree session whose folder holds it.
type File struct {
	Root, Path string
	From       Side
	Delete     bool
	// Conflict is a file changed on both sides since the last sync; From
	// is then the newer, until the user picks.
	Conflict bool
}

// Session is a session to bring to one side: all of it when New, else
// its conversation, From's winning.
type Session struct {
	Key, Name string
	From      Side
	New       bool
	Conflict  bool // its conversation went on on both sides
	// Restarts is the session on the side taking it, running: its agent
	// starts again on the conversation brought.
	Restarts bool
}

// Branch is a branch to bring to one side, at From's commit: the side
// taking it moves its own forward when it can, else keeps it beside.
type Branch struct {
	Name   string
	From   Side
	Commit string
}

// Plan is what a sync brings each way.
type Plan struct {
	Files    []File
	Sessions []Session
	Branches []Branch
}

// Empty reports whether both sides are already the same.
func (p Plan) Empty() bool {
	return len(p.Files) == 0 && len(p.Sessions) == 0 && len(p.Branches) == 0
}

// Conflicts counts what changed on both sides.
func (p Plan) Conflicts() int {
	n := 0
	for _, f := range p.Files {
		if f.Conflict {
			n++
		}
	}
	for _, s := range p.Sessions {
		if s.Conflict {
			n++
		}
	}
	return n
}

// Compare plans a sync between here and there, base being what both had
// at the last sync (nil: never synced, so nothing is known deleted).
func Compare(base *Base, here, there proto.ProjectSyncState) Plan {
	if base == nil {
		base = &Base{}
	}
	var p Plan
	p.Files = compareFiles(base.Files[""], "", here.Files, there.Files)
	bykey := [2]map[string]proto.SyncSession{keyed(here.Sessions), keyed(there.Sessions)}
	keys := unionKeys(bykey[Here], bykey[There])
	for _, k := range keys {
		h, inHere := bykey[Here][k]
		t, inThere := bykey[There][k]
		_, synced := base.Sessions[k]
		switch {
		case !inHere || !inThere:
			if synced {
				continue // deleted on one side since: the other keeps it
			}
			from, s := Here, h
			if !inHere {
				from, s = There, t
			}
			p.Sessions = append(p.Sessions, Session{Key: k, Name: s.Name, From: from, New: true})
			continue
		}
		if h.Worktree && t.Worktree {
			p.Files = append(p.Files, compareFiles(base.Files[k], k, h.Files, t.Files)...)
		}
		if h.Talk == t.Talk || h.Conversation == "" || t.Conversation == "" {
			continue
		}
		s := Session{Key: k, Name: h.Name}
		switch b := base.Sessions[k]; {
		case synced && h.Talk == b:
			s.From = There
		case synced && t.Talk == b:
			s.From = Here
		default:
			s.Conflict = true
			if t.Said > h.Said {
				s.From = There
			}
		}
		p.Sessions = append(p.Sessions, s)
	}
	for i := range p.Sessions {
		s := &p.Sessions[i]
		if !s.New {
			taker := bykey[s.From.Other()][s.Key]
			s.Restarts = taker.Running
		}
	}
	for _, n := range unionKeys(here.Branches, there.Branches) {
		h, t, b := here.Branches[n], there.Branches[n], base.Branches[n]
		switch {
		case h == t:
		case h == "" || t == "":
			if b != "" {
				continue // deleted on one side since the last sync
			}
			if h == "" {
				p.Branches = append(p.Branches, Branch{Name: n, From: There, Commit: t})
			} else {
				p.Branches = append(p.Branches, Branch{Name: n, From: Here, Commit: h})
			}
		case h == b:
			p.Branches = append(p.Branches, Branch{Name: n, From: There, Commit: t})
		case t == b:
			p.Branches = append(p.Branches, Branch{Name: n, From: Here, Commit: h})
		default:
			// Committed to on both sides: the connected host takes the
			// other's commits, beside its own when they can't follow on.
			p.Branches = append(p.Branches, Branch{Name: n, From: There, Commit: t})
		}
	}
	return p
}

// compareFiles plans the files of one folder, root, base being their
// hashes at the last sync.
func compareFiles(base map[string]string, root string, here, there []proto.SyncFile) []File {
	h, t := filesByPath(here), filesByPath(there)
	var out []File
	for _, path := range unionKeys(h, t) {
		fh, inHere := h[path]
		ft, inThere := t[path]
		if inHere && inThere && fh.Hash == ft.Hash {
			continue
		}
		b, known := base[path]
		f := File{Root: root, Path: path}
		switch {
		case !inHere && !inThere:
			continue
		case known && inHere && fh.Hash == b:
			f.From, f.Delete = There, !inThere
		case known && inThere && ft.Hash == b:
			f.From, f.Delete = Here, !inHere
		case !inHere && !known:
			f.From = There
		case !inThere && !known:
			f.From = Here
		default:
			// Changed on both sides, or deleted on one and changed on
			// the other: the newer wins unless the user says.
			f.Conflict = true
			switch {
			case !inHere:
				f.From = There
			case !inThere:
				f.From = Here
			case ft.Modified.After(fh.Modified):
				f.From = There
			}
		}
		out = append(out, f)
	}
	return out
}

func filesByPath(files []proto.SyncFile) map[string]proto.SyncFile {
	m := make(map[string]proto.SyncFile, len(files))
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

func keyed(sessions []proto.SyncSession) map[string]proto.SyncSession {
	m := make(map[string]proto.SyncSession, len(sessions))
	for _, s := range sessions {
		m[s.Key] = s
	}
	return m
}

func unionKeys[V any](a, b map[string]V) []string {
	out := make([]string, 0, len(a)+len(b))
	for k := range a {
		out = append(out, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			out = append(out, k)
		}
	}
	slices.SortFunc(out, cmp.Compare[string])
	return out
}
