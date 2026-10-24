package projectsync

import (
	"slices"

	"github.com/enowx/enowx/proto"
)

// Requests are what side from packs for the other side, and what the
// other applies, for plan p; ok is false when nothing goes that way.
// states are both sides' as scanned; fromName names from's host in the
// other's notes.
func (p Plan) Requests(from Side, states [2]proto.ProjectSyncState, fromName string) (pack proto.ProjectSyncPackParams, apply proto.ProjectSyncApplyParams, ok bool) {
	src, dst := states[from], states[from.Other()]
	ids := map[string]string{} // session key → its id on from's side
	for _, s := range src.Sessions {
		ids[s.Key] = s.ID
	}
	pack.Project, apply.Project, apply.From = src.Path, dst.Path, fromName
	sessions := map[string]*proto.SyncSessionPack{}
	var order []string
	session := func(key string) *proto.SyncSessionPack {
		sp, ok := sessions[key]
		if !ok {
			sp = &proto.SyncSessionPack{ID: ids[key]}
			sessions[key] = sp
			order = append(order, key)
		}
		return sp
	}
	for _, s := range p.Sessions {
		if s.From == from {
			sp := session(s.Key)
			sp.New, sp.Talk = s.New, !s.New
		}
	}
	for _, f := range p.Files {
		switch {
		case f.From != from:
		case f.Root == "" && f.Delete:
			apply.Delete = append(apply.Delete, f.Path)
		case f.Root == "":
			pack.Files = append(pack.Files, f.Path)
		case f.Delete:
			if apply.DeleteIn == nil {
				apply.DeleteIn = map[string][]string{}
			}
			apply.DeleteIn[f.Root] = append(apply.DeleteIn[f.Root], f.Path)
		default:
			sp := session(f.Root)
			sp.Files = append(sp.Files, f.Path)
		}
	}
	for _, key := range order {
		pack.Sessions = append(pack.Sessions, *sessions[key])
	}
	for _, b := range p.Branches {
		if b.From != from {
			continue
		}
		if apply.Branches == nil {
			apply.Branches = map[string]string{}
		}
		pack.Branches = append(pack.Branches, b.Name)
		apply.Branches[b.Name] = b.Commit
	}
	if len(pack.Branches) > 0 {
		for _, c := range dst.Branches {
			pack.Have = append(pack.Have, c)
		}
		slices.Sort(pack.Have)
		pack.Have = slices.Compact(pack.Have)
	}
	ok = len(pack.Files) > 0 || len(pack.Sessions) > 0 || len(pack.Branches) > 0 || len(apply.Delete) > 0 || len(apply.DeleteIn) > 0
	return pack, apply, ok
}
