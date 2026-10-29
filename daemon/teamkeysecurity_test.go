package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/enowx/enowx/cloud/api"
	"github.com/enowx/enowx/cloud/seal"
	"github.com/enowx/enowx/proto"
)

// A device the cloud lists for the team but whose account the team
// doesn't have isn't sealed the team's key. (A cloud that lies about the
// members too still gets it: the design's limit, docs/cloud.md, Keys.)
func TestTeamKeyNotSealedToAStrayDevice(t *testing.T) {
	sc := newHookedCloud(t)
	team, hs := teamOfHosts(t, sc, []string{"ada"}, nil)
	ada := hs[0]
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Acquisition plan", Team: team}, Body: "confidential\n"}); err != nil {
		t.Fatal(err)
	}
	evil, _ := seal.NewDeviceKey()
	var mu sync.Mutex
	var sealedToEvil bool
	sc.setHook(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/v1/teams/") && strings.HasSuffix(r.URL.Path, "/devices"):
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			var list []api.TeamDevice
			json.Unmarshal(rec.Body.Bytes(), &list)
			list = append(list, api.TeamDevice{Account: "stranger", Name: "ada", Epochs: []int{},
				Device: api.Device{ID: "evil-device", Name: "ada's laptop", Kind: api.DeviceApp, PublicKey: evil.Public()}})
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(list)
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/keys/"):
			body, _ := io.ReadAll(r.Body)
			var p api.ShareKeyParams
			json.Unmarshal(body, &p)
			var keep []api.DeviceKey
			for _, k := range p.Keys {
				if k.Device == "evil-device" {
					mu.Lock()
					sealedToEvil = true
					mu.Unlock()
					continue
				}
				keep = append(keep, k)
			}
			p.Keys = keep
			body, _ = json.Marshal(p)
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			next.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
	ada.sync(t, team)
	mu.Lock()
	defer mu.Unlock()
	if sealedToEvil {
		t.Fatal("the host sealed the team's key to a device of an account the team doesn't have")
	}
}

// A device's token alone, without the device's key, sets no recovery key:
// a stolen token doesn't become the team's key.
func TestStolenTokenSetsNoRecoveryKey(t *testing.T) {
	ctx := context.Background()
	sc := newHookedCloud(t)
	team, hs := teamOfHosts(t, sc, []string{"ada", "bob"}, []string{"", api.RoleMember})
	ada, bob := hs[0], hs[1]
	ada.sync(t, team)
	bob.sync(t, team)

	thief := cloudAs(t, sc, bob) // bob's token, from a log or a backup
	evil, _ := seal.NewDeviceKey()
	if _, err := thief.SetRecovery(ctx, evil.Public(), evil.OpenKey); err == nil {
		t.Fatal("a recovery key was set without the device's key")
	}
	ada.sync(t, team)
	if held, err := thief.RecoveryKeys(ctx, team); err == nil && len(held.Keys) > 0 {
		t.Fatalf("the team's keys were sealed to the thief's recovery key: %d", len(held.Keys))
	}
}

// A member who seals another key to a new member's device first doesn't
// cut them off: the copy fails its epoch's check, the new device drops it
// and waits, a holder of the real key seals it again, and the new member
// then reads and writes with the team.
func TestPoisonedKeyCopyIsDroppedAndReplaced(t *testing.T) {
	ctx := context.Background()
	sc := newHookedCloud(t)
	team, hs := teamOfHosts(t, sc, []string{"ada", "bob"}, []string{"", api.RoleMember})
	ada, bob := hs[0], hs[1]
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Existing", Team: team}, Body: "x\n"}); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team)
	bob.sync(t, team)

	carol := signInHost(t, sc.db, sc.url, 200, "carol")
	joinTeam(t, sc, team, ada, carol, api.RoleMember)
	bc := cloudAs(t, sc, bob)
	keys, _ := bc.TeamKeys(ctx, team)
	bogus, _ := seal.NewTeamKey()
	dk, _ := carol.srv.cloud.store.DeviceKey()
	sealed, _ := seal.SealKey(bogus, dk.Public())
	if err := bc.ShareKey(ctx, team, keys.Epoch, []api.DeviceKey{{Device: deviceOf(t, carol), Sealed: sealed}}); err != nil {
		t.Fatal(err)
	}
	if _, err := carol.srv.syncSpace(team); err == nil {
		t.Fatal("carol's host synced with a key that fails its check")
	}
	ada.sync(t, team) // carol lacks the epoch's key again: sealed for her
	carol.sync(t, team)
	if _, ok := carol.noteTitled(t, "Existing"); !ok {
		t.Fatal("carol doesn't read the team's note")
	}
	if _, err := carol.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "From carol", Team: team}, Body: "hi\n"}); err != nil {
		t.Fatal(err)
	}
	carol.sync(t, team)
	ada.sync(t, team)
	if _, ok := ada.noteTitled(t, "From carol"); !ok {
		t.Fatal("ada doesn't read carol's note")
	}
}

// After a member's removal moves the key to a new epoch, a member who
// joins later is sealed the earlier epochs' keys too, and reads what was
// written before.
func TestNewMemberGetsEarlierEpochs(t *testing.T) {
	ctx := context.Background()
	sc := newHookedCloud(t)
	team, hs := teamOfHosts(t, sc, []string{"ada", "bob"}, []string{"", api.RoleMember})
	ada, bob := hs[0], hs[1]
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Old", Team: team}, Body: "x\n"}); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team)
	bob.sync(t, team)
	if err := sc.db.RemoveMember(ctx, team, ada.acc, bob.acc); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team) // starts epoch 2
	carol := signInHost(t, sc.db, sc.url, 300, "carol")
	joinTeam(t, sc, team, ada, carol, api.RoleMember)
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "New", Team: team}, Body: "y\n"}); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team)
	carol.sync(t, team)
	if _, ok := carol.noteTitled(t, "New"); !ok {
		t.Fatal("carol didn't get the current epoch's note")
	}
	if _, ok := carol.noteTitled(t, "Old"); !ok {
		t.Fatal("carol didn't get the note written under the epoch before")
	}
}

// A team whose epoch was started before key checks (by an older version)
// still syncs: its copies have no check to compare with.
func TestEpochWithoutCheckStillSyncs(t *testing.T) {
	ctx := context.Background()
	sc := newHookedCloud(t)
	team, hs := teamOfHosts(t, sc, []string{"ada", "bob"}, []string{"", api.RoleMember})
	ada, bob := hs[0], hs[1]
	key, _ := seal.NewTeamKey()
	dk, _ := ada.srv.cloud.store.DeviceKey()
	sealed, _ := seal.SealKey(key, dk.Public())
	if err := cloudAs(t, sc, ada).ShareKey(ctx, team, 1, []api.DeviceKey{{Device: deviceOf(t, ada), Sealed: sealed}}); err != nil {
		t.Fatal(err)
	}
	if _, err := ada.c.SaveNote(proto.Note{NoteInfo: proto.NoteInfo{Title: "Before checks", Team: team}, Body: "x\n"}); err != nil {
		t.Fatal(err)
	}
	ada.sync(t, team)
	bob.sync(t, team)
	if _, ok := bob.noteTitled(t, "Before checks"); !ok {
		t.Fatal("bob doesn't read a note of an epoch with no check")
	}
}
