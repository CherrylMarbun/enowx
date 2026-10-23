package ui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// fakeUpdates offers a version, installs it as told, and records a
// relaunch.
type fakeUpdates struct {
	enabled    bool
	offer      *newVersion
	installErr error
	relaunched bool
}

func (f *fakeUpdates) Enabled() bool { return f.enabled }
func (f *fakeUpdates) Check(context.Context) (*newVersion, error) {
	return f.offer, nil
}
func (f *fakeUpdates) Relaunch() { f.relaunched = true }

// updateApp is an app whose update work runs when the test drains it.
func updateApp(src *fakeUpdates) (*App, func()) {
	queue := make(chan func(), 64)
	a := &App{}
	a.updates.source, a.updates.post = src, func(f func()) { queue <- f }
	drain := func() {
		for {
			select {
			case f := <-queue:
				f()
			case <-time.After(200 * time.Millisecond):
				return
			}
		}
	}
	return a, drain
}

func TestUpdateIsFoundInstalledAndRestarted(t *testing.T) {
	src := &fakeUpdates{enabled: true}
	src.offer = &newVersion{Version: "9.9.9", Notes: "## What's new\n\n- Faster", Install: func(_ context.Context, progress func(int64, int64)) error {
		progress(50, 100)
		if src.installErr != nil {
			return src.installErr
		}
		progress(100, 100)
		return nil
	}}
	a, drain := updateApp(src)
	tt := ui.NewTester(func(c *ui.Context) {
		a.updateChip(c)
		a.updateDialog(c)
	}, 900, 600)

	a.checkForUpdates(false)
	drain()
	tt.Frame()
	if !tt.HasText("Update to 9.9.9") || tt.HasText("Faster") {
		t.Fatalf("after a check of its own: %q", tt.Texts())
	}
	if err := tt.Click("Update to 9.9.9"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("enowX 9.9.9 is available") || !tt.HasText("Faster") {
		t.Fatalf("dialog: %q", tt.Texts())
	}

	// A failed install says why and offers to try again.
	src.installErr = errors.New("signature mismatch")
	if err := tt.Click("Install update"); err != nil {
		t.Fatal(err)
	}
	drain()
	tt.Frame()
	if !tt.HasText("The update failed: signature mismatch") || !tt.HasText("Try again") {
		t.Fatalf("after a failed install: %q", tt.Texts())
	}
	src.installErr = nil
	if err := tt.Click("Try again"); err != nil {
		t.Fatal(err)
	}
	drain()
	tt.Frame()
	if !tt.HasText("Restart to update") || !tt.HasText("Restart now") {
		t.Fatalf("after installing: %q", tt.Texts())
	}
	if err := tt.Click("Restart now"); err != nil {
		t.Fatal(err)
	}
	if !src.relaunched {
		t.Fatal("didn't relaunch")
	}
}

func TestSkippedVersionStaysQuietUntilAsked(t *testing.T) {
	src := &fakeUpdates{enabled: true, offer: &newVersion{Version: "9.9.9"}}
	a, drain := updateApp(src)
	a.state.Updates.Skipped = "9.9.9"
	a.checkForUpdates(false)
	drain()
	if a.updateLabel() != "" {
		t.Fatalf("a skipped version shows: %q", a.updateLabel())
	}
	// Asked for, it shows anyway, in its dialog.
	a.checkForUpdates(true)
	drain()
	if a.updateLabel() != "Update to 9.9.9" || !a.updates.dialog {
		t.Fatalf("asked: label %q, dialog %v", a.updateLabel(), a.updates.dialog)
	}
}

func TestAskingWhenUpToDateOrDisabledSaysSo(t *testing.T) {
	src := &fakeUpdates{enabled: true}
	a, drain := updateApp(src)
	a.checkForUpdates(true)
	drain()
	if a.status == "" || a.updateLabel() != "" {
		t.Fatalf("up to date: status %q, label %q", a.status, a.updateLabel())
	}
	src.enabled = false
	a.status = ""
	a.checkForUpdates(true)
	if a.status == "" {
		t.Fatal("a build without updates says nothing")
	}
}
