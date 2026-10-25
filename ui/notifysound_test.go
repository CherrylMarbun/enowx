package ui

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/notify"
	"github.com/enowx/enowx/daemon"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// sounds records what plays instead of playing it.
type sounds struct {
	mu     sync.Mutex
	played [][]byte
}

func (s *sounds) take() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.played
	s.played = nil
	return out
}

func catchSounds(t *testing.T) *sounds {
	s := &sounds{}
	old := playSound
	playSound = func(data []byte, _ float64) error {
		s.mu.Lock()
		s.played = append(s.played, data)
		s.mu.Unlock()
		return nil
	}
	t.Cleanup(func() { playSound = old })
	return s
}

// Agents finishing, asking and failing play their sounds once each, a
// burst as one, none when muted, quiet or off; an agent's own ask plays
// too; and Settings' Notifications keep the user's choices.
func TestNotificationSounds(t *testing.T) {
	snd := catchSounds(t)
	srv, err := daemon.NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Close)
	app := New(Options{})
	var mu sync.Mutex
	var later []func()
	app.onMain = func(fn func()) { mu.Lock(); later = append(later, fn); mu.Unlock() }
	pa, pb := net.Pipe()
	go srv.ServeConn(pa)
	cl, err := client.New(pb, func(ev proto.Event) { app.handleEvent(app.gen, ev) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	app.client, app.host = cl, hosts.Host{ID: hosts.LocalID, Name: "This Mac"}
	app.page, app.settingsSection = pageSettings, sectionNotify
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(900).Height(900).Background(theme.Background).Children(func() { app.settingsBody(c) })
	}, 900, 900)
	run := func(d time.Duration) {
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			mu.Lock()
			fns := later
			later = nil
			mu.Unlock()
			for _, fn := range fns {
				fn()
			}
			tt.Frame()
			time.Sleep(10 * time.Millisecond)
		}
	}
	app.loadNotify()
	run(200 * time.Millisecond)
	if !app.ntf.loaded {
		t.Fatal("settings never read")
	}

	s := proto.SessionInfo{ID: "s1", Agent: "claude", State: proto.StateWorking}
	app.sessions = []proto.SessionInfo{s}
	change := func(info proto.SessionInfo) {
		old := s
		s = info
		app.soundFor(old, info, true)
	}
	working := s
	done := s
	done.State = proto.StateDone
	change(done)
	run(soundGather + 300*time.Millisecond)
	if got := snd.take(); len(got) != 1 || string(got[0]) != string(notify.Builtin("chime")) {
		t.Fatalf("a turn done played %d sounds", len(got))
	}

	// A burst: done, then asking, then failing, together: one sound, the
	// failure's.
	app.ntf.last = time.Time{}
	change(working)
	waiting := s
	waiting.State = proto.StateWaiting
	change(waiting)
	failed := waiting
	failed.Exited, failed.ExitCode = true, 1
	change(failed)
	run(soundGather + 300*time.Millisecond)
	if got := snd.take(); len(got) != 1 || string(got[0]) != string(notify.Builtin("alert")) {
		t.Fatalf("a burst played %d sounds", len(got))
	}

	// Muted, then quiet hours around now: nothing.
	app.muteSession("s1", true)
	run(200 * time.Millisecond)
	s = working
	change(done)
	run(soundGather + 300*time.Millisecond)
	if got := snd.take(); len(got) != 0 {
		t.Fatalf("a muted session played %d sounds", len(got))
	}
	app.muteSession("s1", false)
	st := app.notifySettings()
	now := time.Now()
	st.Quiet, st.QuietFrom, st.QuietTo = true, now.Add(-time.Hour).Format("15:04"), now.Add(time.Hour).Format("15:04")
	app.setNotify(st)
	run(200 * time.Millisecond)
	s = working
	change(done)
	run(soundGather + 300*time.Millisecond)
	if got := snd.take(); len(got) != 0 {
		t.Fatalf("quiet hours played %d sounds", len(got))
	}
	st.Quiet = false
	app.setNotify(st)
	run(200 * time.Millisecond)

	// An agent asks for a sound, from the command line: it plays.
	app.ntf.last = time.Time{}
	if err := cl.NotifySound(proto.NotifySoundParams{Kind: proto.NotifyInput}); err != nil {
		t.Fatal(err)
	}
	run(soundGather + 500*time.Millisecond)
	if got := snd.take(); len(got) != 1 || string(got[0]) != string(notify.Builtin("ping")) {
		t.Fatalf("an agent's ask played %d sounds", len(got))
	}

	// Settings: pick another sound for finished turns; it is kept.
	if !tt.HasText("Agent finished") {
		t.Fatalf("no section; shows %q", tt.Texts())
	}
	shot(t, tt, "notify-settings.png")
	if err := tt.Click("Bell"); err != nil {
		t.Fatal(err)
	}
	run(300 * time.Millisecond)
	kept, err := cl.NotifySettings()
	if err != nil || kept.Sounds[proto.NotifyDone] != "bell" {
		t.Fatalf("kept %+v, %v", kept.Sounds, err)
	}
	snd.take()
}

// An agent on a server the app watches finishing plays its sound once on
// this device, though the event comes twice.
func TestRemoteAgentDoneSoundsOnce(t *testing.T) {
	snd := catchSounds(t)
	app := New(Options{})
	var later []func()
	app.onMain = func(fn func()) { later = append(later, fn) }
	app.ntf.loaded, app.ntf.settings = true, notify.Defaults()
	w := &hostWatch{host: hosts.Host{ID: "vps", Name: "datalix-vps"}, stop: make(chan struct{})}
	w.status.sessions = []proto.SessionInfo{{ID: "r1", Agent: "claude", State: proto.StateWorking}}
	app.watches = map[string]*hostWatch{"vps": w}
	done, _ := json.Marshal(proto.SessionInfo{ID: "r1", Agent: "claude", State: proto.StateDone})
	for range 2 {
		app.watchEvent(w, proto.Event{Name: proto.EventSessionUpdated, Data: done})
	}
	deadline := time.Now().Add(soundGather + 500*time.Millisecond)
	for time.Now().Before(deadline) {
		fns := later
		later = nil
		for _, fn := range fns {
			fn()
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := snd.take(); len(got) != 1 {
		t.Fatalf("an agent done on a server played %d sounds", len(got))
	}
}
