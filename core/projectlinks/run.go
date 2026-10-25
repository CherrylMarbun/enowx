package projectlinks

import (
	"fmt"
	"strings"
	"time"

	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/proto"
)

// Daemon is what a link's sync asks of each copy's daemon.
type Daemon interface {
	projectsync.Daemon
	ScanProjectSyncWith(proto.ProjectSyncScanParams) (proto.ProjectSyncState, error)
}

// Sync is one sync of a link, on its own.
type Sync struct {
	Link    proto.ProjectLink
	Daemons [2]Daemon // the source's, the target's
	// Base is what both had at the last sync, nil for none.
	Base *projectsync.Base
	// Carry carries archive id, of size bytes, from one side's daemon to
	// the other's.
	Carry func(from projectsync.Side, id string, size int64) error
	Now   time.Time
}

// Result is what a sync did.
type Result struct {
	Sent, Received int // files, source → target and back
	Conflicts      []proto.ProjectSyncConflict
	Base           projectsync.Base // for the next sync
	Notes          []string
}

// Summary says what r did, in a few words.
func (r Result) Summary(l proto.ProjectLink) string {
	var parts []string
	if r.Sent > 0 {
		parts = append(parts, fmt.Sprintf("%s to %s", files(r.Sent), l.Target.Host))
	}
	if r.Received > 0 {
		parts = append(parts, fmt.Sprintf("%s to %s", files(r.Received), l.Source.Host))
	}
	if len(parts) == 0 {
		parts = append(parts, "already the same")
	}
	if n := len(r.Conflicts); n > 0 {
		parts = append(parts, fmt.Sprintf("%d in conflict", n))
	}
	return strings.Join(parts, "; ")
}

func files(n int) string {
	if n == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", n)
}

// Scan is how a link scans a copy at dir.
func Scan(l proto.ProjectLink, dir string) proto.ProjectSyncScanParams {
	return proto.ProjectSyncScanParams{Project: dir, Env: l.Env, All: l.All, Include: l.Include, Exclude: l.Exclude, FilesOnly: true}
}

// Do runs the sync: both copies scanned by the link's rules, compared
// with the base, and what each lacks carried over, by AutoPlan.
func (s Sync) Do() (Result, error) {
	l := s.Link
	ends := [2]proto.ProjectEnd{l.Source, l.Target}
	var states [2]proto.ProjectSyncState
	for side, d := range s.Daemons {
		st, err := d.ScanProjectSyncWith(Scan(l, ends[side].Path))
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", ends[side].Host, err)
		}
		states[side] = st
	}
	if s.Now.IsZero() {
		s.Now = time.Now().UTC()
	}
	plan, conflicts := AutoPlan(l, projectsync.Compare(s.Base, states[0], states[1]), states, s.Now)
	res := Result{Conflicts: conflicts}
	if err := s.carryPlan(plan, &states, &res); err != nil {
		return res, err
	}
	res.Base = projectsync.BaseOf(states[0], states[1])
	return res, nil
}

// Bring carries the project's files named (each from its side, deleted
// where that side has none) to the other side: a conflict resolved.
func (s Sync) Bring(files []projectsync.File) error {
	ends := [2]proto.ProjectEnd{s.Link.Source, s.Link.Target}
	states := [2]proto.ProjectSyncState{{Path: ends[0].Path}, {Path: ends[1].Path}}
	var res Result
	return s.carryPlan(projectsync.Plan{Files: files}, &states, &res)
}

// carryPlan carries plan's files each way, states being both sides'
// (the side taking files is scanned again after).
func (s Sync) carryPlan(plan projectsync.Plan, states *[2]proto.ProjectSyncState, res *Result) error {
	ends := [2]proto.ProjectEnd{s.Link.Source, s.Link.Target}
	for _, from := range []projectsync.Side{Source, Target} {
		to := from.Other()
		pack, apply, ok := plan.Requests(from, *states, ends[from].Host)
		if !ok {
			continue
		}
		info, err := s.Daemons[from].PackProjectSync(pack)
		if err != nil {
			return fmt.Errorf("%s: %w", ends[from].Host, err)
		}
		if err := s.Carry(from, info.ID, info.Size); err != nil {
			_ = s.Daemons[from].DiscardTransfer(info.ID)
			_ = s.Daemons[to].DiscardTransfer(info.ID)
			return err
		}
		_ = s.Daemons[from].DiscardTransfer(info.ID)
		apply.ID = info.ID
		out, err := s.Daemons[to].ApplyProjectSync(apply)
		if err != nil {
			_ = s.Daemons[to].DiscardTransfer(info.ID)
			return fmt.Errorf("%s: %w", ends[to].Host, err)
		}
		res.Notes = append(res.Notes, out.Notes...)
		n := len(pack.Files) + len(apply.Delete)
		if from == Source {
			res.Sent = n
		} else {
			res.Received = n
		}
		// The side taking it scans again: the base is what both have now.
		if states[to], err = s.Daemons[to].ScanProjectSyncWith(Scan(s.Link, ends[to].Path)); err != nil {
			return fmt.Errorf("%s: %w", ends[to].Host, err)
		}
	}
	return nil
}
