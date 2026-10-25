package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/enowx/enowx/proto"
)

// An agent uses a device: its screen as a PNG, its controls numbered, a
// control tapped by its number, text typed into another, a key pressed.
func TestAndroidUsedByAnAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	sdk := fakeSDK(t)
	t.Setenv("ANDROID_HOME", sdk)
	srv := startServer(t)
	c := connect(t, srv, nil)

	png, err := c.AndroidScreenshot("R58")
	if err != nil || !strings.HasPrefix(string(png), "\x89PNG") {
		t.Fatalf("screenshot %q %v", png, err)
	}
	ctl, err := c.AndroidControls("R58")
	if err != nil {
		t.Fatal(err)
	}
	want := "[1] EditText (Email) id=email clickable editable @540,460\n[2] Button \"Sign in\" id=login clickable @540,660\n"
	if ctl.Text != want {
		t.Fatalf("controls:\n%s\nwant:\n%s", ctl.Text, want)
	}
	for _, act := range []proto.AndroidAct{
		{Serial: "R58", Action: proto.AndroidType, Ref: 1, Text: "ada@example.com it's"},
		{Serial: "R58", Action: proto.AndroidTap, Ref: 2},
		{Serial: "R58", Action: proto.AndroidPress, Key: "back"},
		{Serial: "R58", Action: proto.AndroidSwipe, X: 500, Y: 1800, X2: 500, Y2: 600},
	} {
		if err := c.AndroidAct(act); err != nil {
			t.Fatalf("%+v: %v", act, err)
		}
	}
	if err := c.AndroidAct(proto.AndroidAct{Serial: "R58", Action: proto.AndroidTap, Ref: 9}); err == nil {
		t.Fatal("a control that isn't there was tapped")
	}
	calls, _ := os.ReadFile(filepath.Join(sdk, "platform-tools", "calls"))
	for _, w := range []string{"input tap 540 460", `input text ada@example.com%sit\'s`, "input tap 540 660", "input keyevent 4", "input swipe 500 1800 500 600 300"} {
		if !strings.Contains(string(calls), w) {
			t.Errorf("no %q in:\n%s", w, calls)
		}
	}
}
