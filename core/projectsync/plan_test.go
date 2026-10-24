package projectsync

import (
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func files(kv ...string) []proto.SyncFile {
	var out []proto.SyncFile
	for i := 0; i < len(kv); i += 2 {
		out = append(out, proto.SyncFile{Path: kv[i], Hash: kv[i+1]})
	}
	return out
}

// What changed on one side since the last sync goes to the other, a file
// deleted included; never synced, a file one side lacks is new to it.
func TestCompareTellsWhichSideChanged(t *testing.T) {
	here := proto.ProjectSyncState{Files: files("same", "1", "edited", "2", "kept", "1")}
	there := proto.ProjectSyncState{Files: files("same", "1", "edited", "1", "added", "1")}
	base := &Base{Files: map[string]map[string]string{"": {"same": "1", "edited": "1", "kept": "1"}}}
	got := map[string]File{}
	for _, f := range Compare(base, here, there).Files {
		got[f.Path] = f
	}
	want := map[string]File{
		"edited": {Path: "edited", From: Here},
		"kept":   {Path: "kept", From: There, Delete: true}, // deleted there
		"added":  {Path: "added", From: There},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: got %+v, want %+v", k, got[k], w)
		}
	}
	// Never synced: nothing is known deleted.
	for _, f := range Compare(nil, here, there).Files {
		if f.Path == "kept" && (f.From != Here || f.Delete) {
			t.Errorf("kept with no base: %+v", f)
		}
	}
}

// A file changed on both sides is a conflict, the newer winning until the
// user picks; a branch committed to on both comes here.
func TestCompareConflicts(t *testing.T) {
	now := time.Now()
	here := proto.ProjectSyncState{
		Files:    []proto.SyncFile{{Path: "a", Hash: "2", Modified: now}},
		Branches: map[string]string{"main": "c2", "feat": "f1"},
	}
	there := proto.ProjectSyncState{
		Files:    []proto.SyncFile{{Path: "a", Hash: "3", Modified: now.Add(time.Second)}},
		Branches: map[string]string{"main": "c3", "feat": "f2"},
	}
	base := &Base{Files: map[string]map[string]string{"": {"a": "1"}}, Branches: map[string]string{"main": "c1", "feat": "f1"}}
	p := Compare(base, here, there)
	if len(p.Files) != 1 || !p.Files[0].Conflict || p.Files[0].From != There {
		t.Errorf("files %+v", p.Files)
	}
	want := []Branch{{Name: "feat", From: There, Commit: "f2"}, {Name: "main", From: There, Commit: "c3"}}
	if len(p.Branches) != len(want) {
		t.Fatalf("branches %+v", p.Branches)
	}
	for i := range want {
		if p.Branches[i] != want[i] {
			t.Errorf("branch %d: %+v, want %+v", i, p.Branches[i], want[i])
		}
	}
}

// A session made on one side goes to the other; one synced before and
// deleted on a side since isn't made again.
func TestCompareSessions(t *testing.T) {
	here := proto.ProjectSyncState{Sessions: []proto.SyncSession{{Key: "new", Name: "n"}, {Key: "old"}}}
	there := proto.ProjectSyncState{}
	p := Compare(&Base{Sessions: map[string]string{"old": ""}}, here, there)
	if len(p.Sessions) != 1 || p.Sessions[0].Key != "new" || !p.Sessions[0].New || p.Sessions[0].From != Here {
		t.Errorf("sessions %+v", p.Sessions)
	}
}
