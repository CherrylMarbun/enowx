package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enowx/enowx/core/usersync"
	"github.com/enowx/enowx/proto"
)

// Pausing through the daemon tells every app, and the preferences for it
// are kept with the synced settings, a change synced in taking effect.
func TestTrafficPausesThroughTheDaemon(t *testing.T) {
	srv := startServer(t)
	events := make(chan proto.TrafficCapture, 8)
	connect(t, srv, func(ev proto.Event) {
		var st proto.TrafficCapture
		if ev.Name == proto.EventTrafficCapture && json.Unmarshal(ev.Data, &st) == nil {
			events <- st
		}
	})
	c := connect(t, srv, nil)
	st, err := c.TrafficCapture()
	if err != nil || st.State != proto.CaptureRecording {
		t.Fatalf("at first: %+v, %v", st, err)
	}
	if _, err := c.SetTrafficCapture(proto.TrafficCaptureParams{Action: proto.CapturePause, Sources: []string{"browser"}}); err != nil {
		t.Fatal(err)
	}
	select {
	case st := <-events:
		if st.State != proto.CapturePaused || len(st.Sources) != 1 {
			t.Errorf("event %+v", st)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no capture event")
	}
	if _, err := c.SetTrafficCapture(proto.TrafficCaptureParams{Action: "nap"}); err == nil {
		t.Error("an unknown action was taken")
	}

	prefs, err := c.SetTrafficCapturePrefs(proto.TrafficCapturePrefs{Rules: "pause", AutoCount: 500})
	if err != nil || prefs.Breakpoints != "release" {
		t.Fatalf("prefs %+v, %v", prefs, err)
	}
	data, err := os.ReadFile(filepath.Join(srv.dataDir, "settings", "traffic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved proto.TrafficCapturePrefs
	json.Unmarshal(data, &saved)
	if saved.Rules != "pause" || saved.AutoCount != 500 {
		t.Errorf("saved %s", data)
	}
	// From another host, through a sync.
	b, _ := json.Marshal(proto.TrafficCapturePrefs{Rules: "keep", AutoMinutes: 30})
	if _, _, err := srv.userdata.Write([]proto.SyncChange{{Path: "settings/traffic.json", Data: b, Expect: usersync.Hash(data)}}); err != nil {
		t.Fatal(err)
	}
	srv.syncApplied([]string{"settings/traffic.json"}, "vps")
	if got, _ := c.TrafficCapturePrefs(); got.AutoMinutes != 30 || got.Rules != "keep" {
		t.Errorf("after a sync: %+v", got)
	}
}
