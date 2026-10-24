package usageview

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/theme"
)

// The chip says how full the host's memory is; clicking it lists the
// biggest sessions, and a session row selects it.
func TestMemoryChipListsTheBiggestSessions(t *testing.T) {
	const gb = 1 << 30
	u := &Meter{host: proto.DaemonStats{Supported: true,
		Host:   proto.HostUsage{MemTotal: 16 * gb, MemUsed: 12 * gb, CPUs: 10, CPU: 30},
		Daemon: proto.Usage{RSS: 37 << 20, Procs: 1},
		Sessions: []proto.SessionUsage{
			{ID: "kiro", Usage: proto.Usage{RSS: 603 << 20, CPU: 2, Procs: 11}},
			{ID: "halo", Usage: proto.Usage{RSS: 854 << 20, CPU: 7, Procs: 9}},
		}}}
	h := Host{Name: "This Mac", Local: true, LocalName: "This Mac", Session: func(id string) (string, string, string) {
		if id == "halo" {
			return "halo", "claude", "cryptgo"
		}
		return "Kiro CLI", "kiro", "enxapi"
	}}
	selected := ""
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Width(600).Height(420).Background(theme.Background).TextColor(theme.Text).Padding(12).AlignItems(ui.End).Children(func() {
			u.Chip(c, h, ChipActions{OpenPage: func() {}, StopAgents: func() {}, Select: func(id string) { selected = id }})
		})
	}, 600, 420)
	tt.Frame()
	if !tt.HasText("75%") {
		t.Fatalf("chip: %q", tt.Texts())
	}
	if err := tt.Click("Memory"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Memory on This Mac") || !tt.HasText("halo") || !tt.HasText("Kiro CLI") {
		t.Fatalf("popover: %q", tt.Texts())
	}
	if dir := os.Getenv("SHOTS"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "memory-chip.png"))
		if err == nil {
			_ = png.Encode(f, tt.Image())
			f.Close()
		}
	}
	if err := tt.Click("halo"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if selected != "halo" || u.Open() {
		t.Fatalf("selected %q, open %v", selected, u.Open())
	}
}
