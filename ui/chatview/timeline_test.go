package chatview

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// A turn is a timeline: the user's message, the work folded once done
// into a line that opens to each step, the answer; the turn under way
// shows its steps, the latest lit.
func TestTurnTimeline(t *testing.T) {
	at := time.Date(2026, 10, 30, 14, 2, 0, 0, time.Local)
	entries := []proto.TranscriptEntry{
		{Index: 0, Kind: proto.TranscriptUser, Text: "Proxy installer-nya suka hang. Tambahin timeout ya.", Time: at},
		{Index: 1, Kind: proto.TranscriptThinking, Text: "hmm", DurationMs: 8000, Time: at.Add(8 * time.Second)},
		{Index: 2, Kind: proto.TranscriptTool, Text: "Read", Detail: "/p/proxy/installer.go", Done: true},
		{Index: 3, Kind: proto.TranscriptAssistant, Text: "Hang-nya dari `http.Get` tanpa batas waktu."},
		{Index: 4, Kind: proto.TranscriptTool, Text: "Edit", Detail: "/p/proxy/installer.go", Added: 18, Removed: 6, Done: true, Time: at.Add(14 * time.Second)},
		{Index: 5, Kind: proto.TranscriptAssistant, Text: "Saya pasang context 30 detik dan retry 3×."},
		{Index: 6, Kind: proto.TranscriptUser, Text: "Jalankan tesnya", Time: at.Add(time.Minute)},
		{Index: 7, Kind: proto.TranscriptTool, Text: "Bash", Detail: "go test ./proxy/..."},
	}
	ch := New()
	turns := splitTurns(entries)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(760).Height(700).Padding(20).Background(theme.Background).TextColor(theme.Text).Children(func() {
			ch.turnTimeline(c.Key(0), turns[0], false)
			ch.turnTimeline(c.Key(1), turns[1], true)
		})
	}, 760, 700)
	tt.Frame()
	for _, want := range []string{"You · 14:02", "Worked for 14s", "· 2 tools · 1 file edited", "Saya pasang context 30 detik dan retry 3×.", "Run", "go test ./proxy/..."} {
		if !tt.HasText(want) {
			t.Fatalf("no %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("Thought for 8s") {
		t.Fatal("the done turn's work shows unfolded")
	}
	if err := tt.Click("Show the work"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Thought for 8s") || !tt.HasText("installer.go  /p/proxy") || !tt.HasText("+18") {
		t.Fatalf("the work opened: %q", tt.Texts())
	}
	timelineShot(t, tt, "chat-timeline.png")
	if err := tt.Click("Hide the work"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText("Thought for 8s") {
		t.Fatal("the work didn't fold again")
	}
}

// timelineShot saves what tt shows as name under $SHOTS, to look at.
func timelineShot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("SHOTS")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, tt.Image())
}

// Wide, the conversation has an outline of the user's messages, each a
// click away.
func TestOutline(t *testing.T) {
	ch := New()
	ch.turns = splitTurns([]proto.TranscriptEntry{
		{Index: 0, Kind: proto.TranscriptUser, Text: "Bikin endpoint health"},
		{Index: 1, Kind: proto.TranscriptAssistant, Text: "Done."},
		{Index: 2, Kind: proto.TranscriptUser, Text: "Ganti logger ke slog\nyang structured"},
		{Index: 3, Kind: proto.TranscriptAssistant, Text: "Done."},
	})
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(1280).Height(700).Background(theme.Background).TextColor(theme.Text).Children(func() {
			ch.conversation(c, Options{})
		})
	}, 1280, 700)
	tt.Frame()
	tt.Frame()
	if !tt.HasText("Your messages") || !tt.HasText("Ganti logger ke slog") {
		t.Fatalf("the outline: %q", tt.Texts())
	}
	timelineShot(t, tt, "chat-outline.png")
	if err := tt.Click("Go to: Bikin endpoint health"); err != nil {
		t.Fatal(err)
	}
	narrow := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(700).Height(700).Children(func() { ch.conversation(c, Options{}) })
	}, 700, 700)
	narrow.Frame()
	narrow.Frame()
	if narrow.HasText("Your messages") {
		t.Fatal("an outline in a narrow conversation")
	}
}
