package notify

import (
	"testing"
	"time"

	"github.com/enowx/enowx/proto"
)

func TestSettingsKeptAndChecked(t *testing.T) {
	s := Open(t.TempDir())
	st, err := s.Get()
	if err != nil || st.Volume != 0.7 || st.Sounds[proto.NotifyDone] != "chime" {
		t.Fatalf("defaults %+v, %v", st, err)
	}
	st.Volume, st.Quiet, st.QuietFrom, st.QuietTo = 0.4, true, "22:30", "06:00"
	st.Sounds[proto.NotifyError] = "bell"
	if _, err := s.Set(st); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get()
	if got.Volume != 0.4 || got.Sounds[proto.NotifyError] != "bell" || !got.Quiet {
		t.Fatalf("kept %+v", got)
	}
	st.QuietTo = "25:00"
	if _, err := s.Set(st); err == nil {
		t.Error("a time of 25:00 kept")
	}
	st.QuietTo, st.Sounds[proto.NotifyDone] = "06:00", "trumpet"
	if _, err := s.Set(st); err == nil {
		t.Error("an unknown sound kept")
	}
}

func TestQuietHoursOverMidnight(t *testing.T) {
	st := Defaults()
	st.Quiet, st.QuietFrom, st.QuietTo = true, "22:00", "07:00"
	at := func(h, m int) time.Time { return time.Date(2026, 10, 26, h, m, 0, 0, time.Local) }
	for _, c := range []struct {
		h, m  int
		quiet bool
	}{{23, 0, true}, {3, 0, true}, {6, 59, true}, {7, 0, false}, {12, 0, false}, {21, 59, false}} {
		if Quiet(st, at(c.h, c.m)) != c.quiet {
			t.Errorf("%02d:%02d quiet = %v", c.h, c.m, !c.quiet)
		}
	}
	st.Quiet = false
	if Quiet(st, at(23, 0)) {
		t.Error("quiet with quiet hours off")
	}
	st.Off = true
	if !Quiet(st, at(12, 0)) {
		t.Error("sounds while off")
	}
}

func TestBuiltinsAndUserSounds(t *testing.T) {
	for _, name := range Builtins() {
		if b := Builtin(name); !isWAV(b) || len(b) < 1000 {
			t.Errorf("%s: %d bytes", name, len(b))
		}
	}
	s := Open(t.TempDir())
	if err := s.SaveSound("ding.wav", Builtin("pop")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSound("../x.wav", Builtin("pop")); err == nil {
		t.Error("a name out of the folder kept")
	}
	if err := s.SaveSound("text.wav", []byte("hello, not a sound")); err == nil {
		t.Error("text kept as a sound")
	}
	if b, err := s.Sound("custom:ding.wav"); err != nil || !isWAV(b) {
		t.Errorf("custom sound: %d bytes, %v", len(b), err)
	}
}
