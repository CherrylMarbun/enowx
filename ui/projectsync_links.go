package ui

import (
	"encoding/json"
	"path"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/proto"
)

// A sync the user runs between two copies keeps their link, with what
// both have alike after it, on both hosts: the next sync, run by hand or
// on its own, from this device or another, starts from it.

// findLink is the link of the copies scanned in states on here's host and
// other's, with its base when one was kept; it runs off the main thread.
func findLink(here, other *client.Client, states [2]proto.ProjectSyncState) (proto.ProjectLink, *projectsync.Base) {
	hi, err1 := here.Info()
	oi, err2 := other.Info()
	if err1 != nil || err2 != nil || hi.ID == "" || oi.ID == "" {
		return proto.ProjectLink{}, nil
	}
	for _, cl := range []*client.Client{here, other} {
		list, err := cl.ProjectLinks("")
		if err != nil {
			continue
		}
		for _, l := range list {
			if !linksEnds(l.ProjectLink, hi.ID, states[0].Path, oi.ID, states[1].Path) {
				continue
			}
			raw, err := cl.ProjectLinkBase(l.ID)
			if err != nil || len(raw) == 0 {
				return l.ProjectLink, nil
			}
			var b projectsync.Base
			if json.Unmarshal(raw, &b) != nil {
				return l.ProjectLink, nil
			}
			return l.ProjectLink, &b
		}
	}
	return proto.ProjectLink{}, nil
}

// linksEnds reports whether l links the folder a on daemon da with b on db.
func linksEnds(l proto.ProjectLink, da, a, db, b string) bool {
	is := func(e proto.ProjectEnd, d, p string) bool { return e.Daemon == d && e.Path == p }
	return is(l.Source, da, a) && is(l.Target, db, b) || is(l.Source, db, b) && is(l.Target, da, a)
}

// keepLink records the link of the copies in states (here's the source
// when it's new) and base on both hosts; it runs off the main thread.
func keepLink(here, other *client.Client, names [2]string, states [2]proto.ProjectSyncState, l proto.ProjectLink, base projectsync.Base) error {
	if l.ID == "" {
		hi, err1 := here.Info()
		oi, err2 := other.Info()
		if err1 != nil || err2 != nil || hi.ID == "" || oi.ID == "" {
			return nil // daemons too old to keep links: nothing to keep
		}
		id, err := here.ProjectID(states[0].Path)
		if err != nil {
			return err
		}
		l = proto.ProjectLink{
			Project: id.ID, Name: path.Base(states[0].Path), Direction: proto.SyncTwoWay,
			Source: proto.ProjectEnd{Daemon: hi.ID, Host: names[0], Path: states[0].Path},
			Target: proto.ProjectEnd{Daemon: oi.ID, Host: names[1], Path: states[1].Path},
		}
		if l, err = here.SaveProjectLink(l); err != nil {
			return err
		}
		if _, err := other.SaveProjectLink(l); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return err
	}
	r := proto.ProjectLinkReport{Link: l.ID, Base: raw, Status: proto.ProjectLinkStatus{Last: time.Now().UTC()},
		Entry: proto.ProjectHistoryEntry{Project: l.Project, Kind: "sync", Link: l.ID, OK: true, Summary: "synced by hand"}}
	if err := here.ReportProjectLink(r); err != nil {
		return err
	}
	return other.ReportProjectLink(r)
}
