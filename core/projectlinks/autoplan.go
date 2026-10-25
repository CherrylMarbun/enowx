package projectlinks

import (
	"path"
	"strings"
	"time"

	"github.com/enowx/enowx/core/projectsync"
	"github.com/enowx/enowx/proto"
)

// Sides of a link's sync: its source's copy, then its target's.
const (
	Source = projectsync.Here
	Target = projectsync.There
)

// DefaultLargeMB is the size from which a file is large, in MB, unless
// the user says.
const DefaultLargeMB = 50

// AutoPlan is what a sync of link l does on its own, from plan, which
// projectsync.Compare made of states (the source's, then the target's):
// the project's files only (sessions, branches and worktrees are for a
// sync the user runs), in l's direction, with its rules for deletes and
// large files. What changed on both sides stays as each side has it, in
// conflicts, unless the rules give it to the source.
func AutoPlan(l proto.ProjectLink, plan projectsync.Plan, states [2]proto.ProjectSyncState, now time.Time) (projectsync.Plan, []proto.ProjectSyncConflict) {
	files := [2]map[string]proto.SyncFile{byPath(states[0].Files), byPath(states[1].Files)}
	limit := int64(l.LargeMB)
	if limit <= 0 {
		limit = DefaultLargeMB
	}
	limit <<= 20
	// lead is the side whose version wins: none for two ways.
	lead := projectsync.Side(-1)
	switch l.Direction {
	case proto.SyncOneWay:
		lead = Source
	case proto.SyncReverse:
		lead = Target
	}
	var out projectsync.Plan
	var conflicts []proto.ProjectSyncConflict
	for _, f := range plan.Files {
		if f.Root != "" {
			continue
		}
		a, inA := files[Source][f.Path]
		b, inB := files[Target][f.Path]
		large := a.Size > limit || b.Size > limit
		if large && l.Large != proto.LargeSource {
			continue // left as each side has it
		}
		if large || f.Conflict && Binary(f.Path) && l.Large == proto.LargeSource {
			f = toSide(f, Source, inA)
		} else if lead >= 0 {
			has := [2]bool{inA, inB}
			if !has[lead] && f.From == lead.Other() && !f.Delete {
				continue // new on the other side: it keeps it
			}
			f = toSide(f, lead, has[lead])
		} else if f.Conflict {
			conflicts = append(conflicts, proto.ProjectSyncConflict{
				Path: f.Path, Sides: [2]proto.SyncFile{a, b}, Gone: [2]bool{!inA, !inB}, Binary: Binary(f.Path), Found: now,
			})
			continue
		}
		if f.Delete && l.Deletes == proto.DeletesNever {
			continue
		}
		out.Files = append(out.Files, f)
	}
	return out, conflicts
}

// toSide makes f bring side's version, which is no file when it has none.
func toSide(f projectsync.File, side projectsync.Side, has bool) projectsync.File {
	f.From, f.Delete, f.Conflict = side, !has, false
	return f
}

func byPath(files []proto.SyncFile) map[string]proto.SyncFile {
	m := make(map[string]proto.SyncFile, len(files))
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

// binaryExts are kinds of files no one merges by hand.
var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true, ".bmp": true, ".tiff": true,
	".psd": true, ".sketch": true, ".fig": true, ".pdf": true, ".zip": true, ".gz": true, ".tgz": true, ".xz": true,
	".7z": true, ".rar": true, ".jar": true, ".war": true, ".exe": true, ".dll": true, ".so": true, ".dylib": true,
	".a": true, ".o": true, ".class": true, ".wasm": true, ".mp3": true, ".wav": true, ".ogg": true, ".mp4": true,
	".mov": true, ".webm": true, ".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".sqlite": true,
	".db": true, ".bin": true, ".dmg": true, ".iso": true, ".apk": true, ".aab": true, ".ipa": true,
}

// Binary reports whether the file at p is of a binary kind, by its name.
func Binary(p string) bool { return binaryExts[strings.ToLower(path.Ext(p))] }
