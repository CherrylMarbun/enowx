package server

import (
	"slices"
	"testing"
	"time"

	"github.com/enowx/enowx/cloud/api"
)

// Members see who of their team is here and who types; a viewer is never
// shown typing, and one outside the team is told it isn't there.
func TestPresence(t *testing.T) {
	e := newTeamEnv(t, Config{})
	own := e.signIn(1, "own", "")
	mem := e.signIn(2, "mem", "")
	vie := e.signIn(3, "vie", "")
	out := e.signIn(4, "out", "")
	team := e.team(own, "t")
	e.join(own, team, api.RoleMember, mem)
	e.join(own, team, api.RoleViewer, vie)
	path := e.url + "/v1/teams/" + team + "/presence"
	var p api.Presence
	if code, msg := send(t, "POST", path, mem.token, api.PresenceParams{Typing: true}, &p); code != 200 {
		t.Fatalf("member: %d %s", code, msg)
	}
	send(t, "POST", path, vie.token, api.PresenceParams{Typing: true}, &p)
	if code, _ := send(t, "POST", path, own.token, api.PresenceParams{}, &p); code != 200 {
		t.Fatalf("owner: %d", code)
	}
	if want := []string{own.acc.ID, mem.acc.ID, vie.acc.ID}; !sameSet(p.Online, want) {
		t.Errorf("online %v, want %v", p.Online, want)
	}
	if !sameSet(p.Typing, []string{mem.acc.ID}) {
		t.Errorf("typing %v, want the member only", p.Typing)
	}
	if code, _ := send(t, "POST", path, out.token, api.PresenceParams{}, nil); code != 404 {
		t.Errorf("outsider: %d, want 404", code)
	}
}

// Presence is forgotten as members go quiet: typing in seconds, being here
// in a minute or so.
func TestPresenceFades(t *testing.T) {
	var p presences
	now := time.Now()
	p.see("t", "a", true, now)
	got := p.see("t", "b", false, now.Add(typingFor+time.Second))
	if len(got.Typing) != 0 || !sameSet(got.Online, []string{"a", "b"}) {
		t.Errorf("after typing's time: %+v", got)
	}
	got = p.see("t", "b", false, now.Add(presenceFor+time.Second))
	if !sameSet(got.Online, []string{"b"}) {
		t.Errorf("after presence's time: %+v", got)
	}
}

func sameSet(got, want []string) bool {
	got, want = slices.Clone(got), slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	return slices.Equal(got, want)
}
