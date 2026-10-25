package daemon

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/usersync"
	"github.com/enowx/enowx/proto"
)

// Notification settings changed on one host reach the apps of another
// through a sync, and a sound an agent asks for reaches the apps.
func TestNotifySettingsAndSounds(t *testing.T) {
	mac, vps := startServer(t), startServer(t)
	events := make(chan proto.Event, 16)
	cm := connect(t, mac, func(e proto.Event) {
		if e.Name == proto.EventNotifySettings || e.Name == proto.EventNotifySound {
			events <- e
		}
	})
	cv := connect(t, vps, nil)
	wait := func(name string) proto.Event {
		t.Helper()
		for {
			select {
			case e := <-events:
				if e.Name == name {
					return e
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("no %s event", name)
			}
		}
	}

	st, err := cv.NotifySettings()
	if err != nil || st.Volume != 0.7 {
		t.Fatalf("defaults %+v, %v", st, err)
	}
	st.Volume, st.Quiet = 0.3, true
	if _, err := cv.SetNotifySettings(st); err != nil {
		t.Fatal(err)
	}
	if _, err := usersync.Sync(client.SyncSide{C: cm}, client.SyncSide{C: cv}, &usersync.Base{}); err != nil {
		t.Fatal(err)
	}
	var got proto.NotifySettings
	if json.Unmarshal(wait(proto.EventNotifySettings).Data, &got) != nil || got.Volume != 0.3 || !got.Quiet {
		t.Fatalf("settings on the mac: %+v", got)
	}

	if err := cm.NotifySound(proto.NotifySoundParams{Kind: proto.NotifyError, Text: "tests failed"}); err != nil {
		t.Fatal(err)
	}
	var snd proto.NotifySoundParams
	if json.Unmarshal(wait(proto.EventNotifySound).Data, &snd) != nil || snd.Kind != proto.NotifyError {
		t.Fatalf("sound %+v", snd)
	}
	if err := cm.NotifySound(proto.NotifySoundParams{Kind: "fanfare"}); err == nil {
		t.Error("an unknown kind of sound asked for")
	}
	if data, err := cm.NotifySoundData("chime"); err != nil || len(data) < 1000 {
		t.Errorf("built-in sound: %d bytes, %v", len(data), err)
	}
}
