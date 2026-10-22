package ui

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/core/procstat"
	"github.com/enowx/enowx/proto"
)

// usageState is what enowX costs: the app on this device, and the daemon
// and sessions of the connected host. It belongs to the main thread; the
// poller of connection gen fills it.
type usageState struct {
	host    proto.DaemonStats
	app     procstat.Usage
	sampler procstat.Sampler // this app; used by the poller only
	gen     int
	open    bool
}

const usageEvery = 2 * time.Second

// pollUsage measures while connection gen lasts.
func (a *App) pollUsage(cl *client.Client, gen int) {
	u := &a.usage
	u.gen, u.host = gen, proto.DaemonStats{}
	go func() {
		for {
			st, err := cl.Stats()
			var app procstat.Usage
			if u.sampler.Sample() == nil {
				app = u.sampler.One(os.Getpid())
			}
			stop := make(chan bool, 1)
			a.win.Update(func() {
				if gen != a.gen {
					stop <- true
					return
				}
				stop <- false
				if err == nil {
					u.host = st
				}
				u.app = app
			})
			if <-stop {
				return
			}
			select {
			case <-cl.Done():
				return
			case <-time.After(usageEvery):
			}
		}
	}()
}

// usageTotal is what enowX uses on the connected host: its daemon and
// sessions, and the app itself when that host is this device.
func (a *App) usageTotal() proto.Usage {
	h := a.usage.host
	t := h.Daemon
	for _, s := range h.Sessions {
		t.RSS, t.CPU, t.Procs = t.RSS+s.RSS, t.CPU+s.CPU, t.Procs+s.Procs
	}
	if a.host.ID == hosts.LocalID {
		t.RSS, t.CPU, t.Procs = t.RSS+a.usage.app.RSS, t.CPU+a.usage.app.CPU, t.Procs+1
	}
	return t
}

// usageButton sits in the status bar and opens the details.
func (a *App) usageButton(c *ui.Context) {
	if !a.usage.host.Supported && a.usage.app.RSS == 0 {
		return
	}
	t := a.usageTotal()
	label := fmt.Sprintf("RAM %s · CPU %s", bytesLabel(t.RSS), cpuLabel(t.CPU))
	b := ui.ButtonBase(c).Gap(6).PaddingX(8).Height(22).Radius(5).Cursor(ui.CursorPointer).Label("Resource usage: " + label)
	if b.Hovered() {
		b.Background(colHover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(colTextSoft).SingleLine()
	})
	if b.Clicked() {
		a.usage.open = true
	}
}

func (a *App) usageDialog(c *ui.Context) {
	u := &a.usage
	if !u.open {
		return
	}
	ui.DialogBase(c, &u.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(620).Padding(20).Gap(16).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi).
			Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.55))
		ui.Row(c).FillWidth().Children(func() {
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				ui.Text(c, "Resource usage").FontSize(16).Bold()
				ui.Text(c, "Memory and CPU of enowX and of every session with all it started. Updates every 2 seconds.").FontSize(12).TextColor(colMuted)
			})
			if iconButton(c, iconClose, "Close", false).Clicked() {
				u.open = false
			}
		})
		if u.host.Supported {
			ui.Text(c, a.host.Name).FontSize(12).Bold().TextColor(colTextSoft)
			a.hostMeters(c, u.host.Host)
		} else {
			ui.Text(c, a.host.Name+" doesn't report its usage.").FontSize(12).TextColor(colMuted)
		}
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			usageHeader(c)
			if a.host.ID != hosts.LocalID {
				usageRow(c.Key("app"), iconLaptop, "enowX app", "on "+LocalHost().Name, proto.Usage{RSS: u.app.RSS, CPU: u.app.CPU, Procs: 1})
			} else {
				usageRow(c.Key("app"), iconLaptop, "enowX app", "", proto.Usage{RSS: u.app.RSS, CPU: u.app.CPU, Procs: 1})
			}
			if u.host.Supported {
				usageRow(c.Key("daemon"), iconServer, "Daemon", "on "+a.host.Name, u.host.Daemon)
				sessions := slices.Clone(u.host.Sessions)
				slices.SortFunc(sessions, func(x, y proto.SessionUsage) int {
					switch {
					case x.RSS > y.RSS:
						return -1
					case x.RSS < y.RSS:
						return 1
					}
					return 0
				})
				for _, su := range sessions {
					name, agentID, project := su.ID, "", ""
					for _, s := range a.sessions {
						if s.ID == su.ID {
							name, agentID, project = a.sessionName(s), s.Agent, projectName(s.Project)
						}
					}
					usageRow(c.Key(su.ID), agentIcon(agentID), name, project, su.Usage)
				}
				if len(sessions) == 0 {
					ui.Text(c, "No session is running.").FontSize(12).TextColor(colMuted).Padding(8, 10)
				}
			}
			t := a.usageTotal()
			ui.Box(c).Height(1).FillWidth().Margin(4, 0).Background(colBorderHi)
			usageCells(c, func() { ui.Text(c, "enowX on "+a.host.Name).Bold().FontSize(12).SingleLine() }, t, true)
		})
	})
}

func (a *App) hostMeters(c *ui.Context, h proto.HostUsage) {
	ui.Row(c).FillWidth().Gap(10).Children(func() {
		meter(c, "Memory", fmt.Sprintf("%s of %s", bytesLabel(h.MemUsed), bytesLabel(h.MemTotal)), ratio(float64(h.MemUsed), float64(h.MemTotal)))
		meter(c, "CPU", fmt.Sprintf("%s of %d cores", cpuLabel(h.CPU), h.CPUs), h.CPU/100)
	})
}

func meter(c *ui.Context, title, value string, fill float64) {
	fill = min(max(fill, 0), 1)
	col := colAccent
	switch {
	case fill > 0.9:
		col = colError
	case fill > 0.75:
		col = ui.Hex("#F2C15B")
	}
	// Half the dialog's content width each, so the bars can be sized.
	const width, barWidth = 285, 285 - 24
	ui.Column(c).Width(width).Shrink(0).Gap(8).Padding(12).Radius(10).Background(colRaised).Border(1, colBorder).Children(func() {
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, title).FontSize(12).TextColor(colTextSoft).Grow(1).Shrink(1).SingleLine()
			ui.Text(c, value).FontSize(12).Bold().SingleLine().Shrink(0)
		})
		ui.Row(c).Width(barWidth).Height(6).Radius(3).Background(ui.Hex("#23262D")).Children(func() {
			ui.Box(c).Height(6).Radius(3).Background(col).Width(float32(fill * barWidth))
		})
	})
}

func usageHeader(c *ui.Context) {
	ui.Row(c).FillWidth().Padding(0, 10, 4, 10).Gap(10).Children(func() {
		ui.Text(c, "").Grow(1)
		for _, h := range []string{"Memory", "CPU", "Processes"} {
			ui.Text(c, h).FontSize(11).TextColor(colMuted).Width(80).TextAlign(ui.End)
		}
	})
}

func usageRow(c *ui.Context, svg *ui.SVG, name, detail string, u proto.Usage) {
	usageCells(c, func() {
		ui.Icon(c, svg).FontSize(13).TextColor(colTextSoft)
		ui.Text(c, name).FontSize(12).SingleLine().Shrink(1)
		if detail != "" {
			ui.Text(c, detail).FontSize(11).TextColor(colMuted).SingleLine().Shrink(1)
		}
	}, u, false)
}

func usageCells(c *ui.Context, label func(), u proto.Usage, bold bool) {
	ui.Row(c).FillWidth().Padding(7, 10).Gap(10).Radius(7).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Gap(8).Children(label)
		for _, v := range []string{bytesLabel(u.RSS), cpuLabel(u.CPU), fmt.Sprint(u.Procs)} {
			t := ui.Text(c, v).FontSize(12).Width(80).TextAlign(ui.End).SingleLine()
			if bold {
				t.Bold()
			}
		}
	})
}

func projectName(path string) string {
	if path == "" {
		return ""
	}
	return lastPathPart(path)
}

func ratio(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func bytesLabel(n uint64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d KB", n>>10)
}

func cpuLabel(pct float64) string { return fmt.Sprintf("%.0f%%", pct) }
