package projectsync

import (
	"fmt"

	"github.com/enowx/enowx/proto"
)

// Daemon is what a sync asks of each side's daemon (a client of it).
type Daemon interface {
	ScanProjectSync(dir string, env bool) (proto.ProjectSyncState, error)
	PackProjectSync(proto.ProjectSyncPackParams) (proto.TransferInfo, error)
	ApplyProjectSync(proto.ProjectSyncApplyParams) (proto.ProjectSyncResult, error)
	DiscardTransfer(id string) error
}

// Run is a sync to run: Plan, compared from States, carried between
// Daemons, whose hosts are Names.
type Run struct {
	Daemons [2]Daemon
	Names   [2]string
	States  [2]proto.ProjectSyncState
	Plan    Plan
	Env     bool
	// Carry carries archive id, of size bytes, from one side's daemon to
	// the other's.
	Carry func(from Side, id string, size int64) error
	// Step learns what the sync is at.
	Step func(from Side, what string)
}

// Do runs r one way then the other, and returns what each side's daemon
// did, by side, and their base for the next sync, from both scanned
// again.
func (r Run) Do() ([2]proto.ProjectSyncResult, Base, error) {
	var res [2]proto.ProjectSyncResult
	step := func(from Side, what string) {
		if r.Step != nil {
			r.Step(from, what)
		}
	}
	for _, from := range []Side{Here, There} {
		to := from.Other()
		pack, apply, ok := r.Plan.Requests(from, r.States, r.Names[from])
		if !ok {
			continue
		}
		step(from, "packing")
		info, err := r.Daemons[from].PackProjectSync(pack)
		if err != nil {
			return res, Base{}, fmt.Errorf("%s: %w", r.Names[from], err)
		}
		step(from, "carrying")
		if err := r.Carry(from, info.ID, info.Size); err != nil {
			_ = r.Daemons[from].DiscardTransfer(info.ID)
			_ = r.Daemons[to].DiscardTransfer(info.ID)
			return res, Base{}, err
		}
		_ = r.Daemons[from].DiscardTransfer(info.ID)
		step(from, "applying")
		apply.ID = info.ID
		if res[to], err = r.Daemons[to].ApplyProjectSync(apply); err != nil {
			_ = r.Daemons[to].DiscardTransfer(info.ID)
			return res, Base{}, fmt.Errorf("%s: %w", r.Names[to], err)
		}
	}
	var after [2]proto.ProjectSyncState
	for side, d := range r.Daemons {
		st, err := d.ScanProjectSync(r.States[side].Path, r.Env)
		if err != nil {
			return res, Base{}, fmt.Errorf("%s: %w", r.Names[side], err)
		}
		after[side] = st
	}
	return res, BaseOf(after[Here], after[There]), nil
}
