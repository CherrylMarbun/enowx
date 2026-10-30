package canvasview

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// A redesign under way (Options.UnslopRun), over its design: the score
// counting to its latest read, from where it started; the twelve passes,
// those done checked, the one under way pulsing, each with what the
// latest read still finds of it; and, when it is done, the score it went
// from and to. The page shows the reads and what each pass changes
// (unslop_web.go).

// hudState is what the panel animates.
type hudState struct {
	shown  float64 // the score as it shows, counting to the run's
	at     time.Time
	frame  string // the run's design, its own count
	closed string // the run (its design) the user closed when done
}

// unslopHUD draws o.UnslopRun's panel over the stage, when it is about
// design f.
func (v *View) unslopHUD(c *ui.Context, o Options, f proto.CanvasFrame) {
	run := o.UnslopRun
	if o.Web != nil {
		if o.Web.pass = ""; run != nil && run.Frame == f.ID && !run.Done {
			o.Web.pass = run.Pass
		}
		o.Web.scan(run, f.ID)
		name := func(string) string { return "" }
		if o.AgentName != nil {
			name = o.AgentName
		}
		o.Web.outlineRegions(run, f.ID, name)
	}
	if run == nil || run.Frame != f.ID || v.hud.closed == run.Frame+fmt.Sprint(run.Done) {
		return
	}
	now := time.Now()
	h := &v.hud
	if h.frame != run.Frame {
		h.frame, h.shown, h.at = run.Frame, float64(run.From), now
	}
	// Counts toward the latest score, a tenth of the way each frame.
	if d := float64(run.Score) - h.shown; math.Abs(d) > 0.4 {
		h.shown += d * min(1, now.Sub(h.at).Seconds()*8)
		c.After(16 * time.Millisecond)
	} else {
		h.shown = float64(run.Score)
	}
	h.at = now
	counts := map[string]int{}
	for _, fd := range run.Findings {
		counts[fd.Pass]++
	}
	col := scoreColor(int(math.Round(h.shown)))
	ui.Column(c.Key("unslop-hud")).Absolute().Right(12).Top(12).Width(264).Padding(12).Gap(10).Radius(12).
		Background(theme.Dialog).Border(1, theme.BorderStrong).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icon.Wand).FontSize(14).TextColor(theme.Accent)
			label := "Unslop"
			switch {
			case run.Done:
				label = "Unslopped"
			case run.Pass != "":
				label = run.Pass
			case regionsAtWork(run) > 0:
				n := regionsAtWork(run)
				label = textfmt.Plural(n, "1 agent at work", fmt.Sprintf("%d agents at work", n))
			case run.Scans <= 1:
				label = "Reading the design…"
			}
			ui.Text(c, label).FontSize(12.5).FontWeight(600).SingleLine().Grow(1).Shrink(1)
			if run.Done && widget.IconButton(c, icon.Close, "Close", false).Clicked() {
				h.closed = run.Frame + "true"
			}
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.End).Children(func() {
			ui.Text(c, fmt.Sprint(int(math.Round(h.shown)))).FontSize(30).FontWeight(300).TextColor(col).Font(theme.MonoFont)
			ui.Text(c, "/100").FontSize(12).TextColor(theme.Muted).Margin(0, 0, 6, 0)
			ui.Spacer(c)
			if run.Score != run.From || run.Done {
				ui.Text(c, fmt.Sprintf("from %d", run.From)).FontSize(11.5).TextColor(theme.Muted).Margin(0, 0, 6, 0)
			}
		})
		// How far through the passes.
		ui.Box(c).FillWidth().Height(4).Radius(2).Background(theme.Sunken).Children(func() {
			done := float32(len(run.Finished)) / float32(max(len(run.Passes), 1))
			if run.Done {
				done = 1
			}
			ui.Box(c).Width(244 * done).Height(4).Radius(2).Background(col)
		})
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			for _, p := range run.Passes {
				v.hudPass(c, run, p, counts[p], now)
			}
		})
		if len(run.Regions) > 0 {
			v.hudRegions(c, o, run)
		}
	})
}

// hudRegions lists a fanned-out run's regions: each in its color, its
// agent, the pass it is at or done.
func (v *View) hudRegions(c *ui.Context, o Options, run *proto.CanvasUnslopRun) {
	ui.Column(c).FillWidth().Gap(2).Padding(8, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(theme.Border).Children(func() {
		ui.Text(c, fmt.Sprintf("%d agents at once", len(run.Regions))).FontSize(10.5).FontWeight(600).TextColor(theme.Muted).Margin(0, 6, 4, 6)
		for i, r := range run.Regions {
			state := AgentWorking
			if o.AgentState != nil {
				state = o.AgentState(r.Session)
			}
			row := ui.ButtonBase(c.Key("region:" + r.ID)).FillWidth().Height(24).PaddingX(6).Gap(8).Radius(6).AlignItems(ui.Center).
				Label("Show the agent of #" + r.ID).Tooltip("Show what this agent does")
			if o.OpenSession != nil {
				row.Cursor(ui.CursorPointer)
				switch {
				case o.Viewing == r.Session:
					row.Background(theme.Fade(theme.Accent, 0.16))
				case row.Hovered():
					row.Background(theme.Hover)
				}
				if row.Clicked() {
					o.OpenSession(r.Session)
				}
			}
			row.Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(ui.Hex(RegionColors[i%len(RegionColors)]))
				name := "#" + r.ID
				if o.AgentName != nil {
					if n := o.AgentName(r.Session); n != "" {
						name = n + " · #" + r.ID
					}
				}
				ui.Text(c, name).FontSize(11.5).TextColor(theme.TextSoft).Grow(1).Shrink(1).SingleLine()
				switch {
				case r.Done || run.Done:
					ui.Icon(c, icon.Check).FontSize(11).TextColor(theme.Done)
				case state == AgentStopped:
					ui.Text(c, "stopped").FontSize(10.5).TextColor(theme.Error)
				case state == AgentWaiting:
					ui.Text(c, "needs you").FontSize(10.5).FontWeight(600).TextColor(theme.Warning)
				case state == AgentIdle:
					ui.Text(c, "idle").FontSize(10.5).TextColor(theme.Muted).Tooltip("Not working: open it to see why")
				default:
					if r.Pass != "" {
						num, _, _ := strings.Cut(r.Pass, " ")
						ui.Text(c, num).FontSize(10.5).TextColor(theme.Accent).Font(theme.MonoFont)
					}
					widget.Spinner(c, 10)
				}
			})
		}
	})
}

// hudPass is a pass's line: checked when done, pulsing while under way,
// with what the latest read finds of it.
func (v *View) hudPass(c *ui.Context, run *proto.CanvasUnslopRun, pass string, found int, now time.Time) {
	done := run.Done || contains(run.Finished, pass)
	active := pass == run.Pass && !run.Done
	for _, r := range run.Regions {
		active = active || (!r.Done && !run.Done && r.Pass == pass)
	}
	row := ui.Row(c.Key(pass)).FillWidth().Height(22).PaddingX(6).Gap(8).Radius(6).AlignItems(ui.Center)
	if active {
		// A slow pulse while the agent works on it.
		a := 0.12 + 0.08*math.Sin(float64(now.UnixMilli())/300)
		row.Background(theme.Fade(theme.Accent, a))
		c.After(33 * time.Millisecond)
	}
	row.Children(func() {
		switch {
		case done:
			ui.Icon(c, icon.Check).FontSize(11).TextColor(theme.Done)
		case active:
			widget.Spinner(c, 10)
		default:
			ui.Box(c).Size(6, 6).Radius(3).Background(theme.Border).Margin(0, 2)
		}
		num, name, _ := strings.Cut(pass, " ")
		ui.Text(c, num).FontSize(10.5).TextColor(theme.Idle).Font(theme.MonoFont)
		tc := theme.TextSoft
		if !done && !active && found == 0 {
			tc = theme.Muted
		}
		ui.Text(c, name).FontSize(11.5).TextColor(tc).Grow(1).SingleLine()
		if found > 0 {
			ui.Text(c, fmt.Sprint(found)).FontSize(10.5).FontWeight(600).TextColor(theme.Warning)
		}
	})
}

// What a region's agent is at, as Options.AgentState says.
const (
	AgentWorking = "working"
	AgentWaiting = "waiting" // asking the user something
	AgentIdle    = "idle"    // neither working nor done: stuck, maybe
	AgentStopped = "stopped"
)

// regionsAtWork are run's regions not done yet.
func regionsAtWork(run *proto.CanvasUnslopRun) int {
	n := 0
	for _, r := range run.Regions {
		if !r.Done {
			n++
		}
	}
	return n
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// scoreColor is a score's: red low, amber middling, green high.
func scoreColor(score int) ui.Color {
	switch {
	case score >= 85:
		return theme.Done
	case score >= 60:
		return theme.Warning
	}
	return theme.Error
}
