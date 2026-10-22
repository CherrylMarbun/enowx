package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/hostconn"
	"github.com/enowx/enowx/proto"
)

// syncState is the Sync environment dialog. It belongs to the main
// thread; its goroutines report back through win.Update and drop their
// results once the dialog is no longer a.sync.
type syncState struct {
	host     hosts.Host
	remote   *client.Client // the app's connection to host
	local    *client.Client // this device's daemon, opened for the dialog
	toRemote bool           // the direction: this device → host

	loading bool
	err     string
	scans   map[bool]proto.EnvScan // by toRemote: the source's scan is scans[true] for local
	rows    []syncRow

	running  bool
	done     int
	total    int
	finished bool
	warnings []string
	failures []string
}

type syncRow struct {
	item    proto.EnvItem
	status  string // syncNew, syncChanged or syncSame
	checked bool
}

const (
	syncNew     = "New"
	syncChanged = "Changed"
	syncSame    = "Same"
)

// openSync opens the dialog for the connected server.
func (a *App) openSync() {
	if a.client == nil || a.host.ID == hosts.LocalID {
		return
	}
	s := &syncState{host: a.host, remote: a.client, toRemote: true, loading: true}
	a.sync = s
	go a.loadSync(s)
}

// loadSync connects to this device's daemon and scans both homes.
func (a *App) loadSync(s *syncState) {
	if s.local == nil {
		local, err := hostconn.Connect(LocalHost(), hostconn.Options{DaemonCommand: a.opts.DaemonCommand})
		if err != nil {
			a.win.Update(func() { s.loading, s.err = false, "Couldn't start this device's daemon: "+err.Error() })
			return
		}
		kept := make(chan bool, 1)
		a.win.Update(func() {
			if a.sync != s {
				local.Close()
				kept <- false
				return
			}
			s.local = local
			kept <- true
		})
		if !<-kept {
			return
		}
	}
	localScan, lerr := s.local.EnvScan()
	remoteScan, rerr := s.remote.EnvScan()
	a.win.Update(func() {
		if a.sync != s {
			return
		}
		s.loading = false
		switch {
		case lerr != nil:
			s.err = "Couldn't read this device: " + lerr.Error()
		case rerr != nil:
			s.err = "Couldn't read " + s.host.Name + ": " + rerr.Error()
		default:
			s.scans = map[bool]proto.EnvScan{true: localScan, false: remoteScan}
			s.buildRows()
		}
	})
}

// buildRows compares the source's items with the destination's.
func (s *syncState) buildRows() {
	src, dst := s.scans[s.toRemote], s.scans[!s.toRemote]
	have := map[string]string{}
	for _, it := range dst.Items {
		have[it.ID] = it.Hash
	}
	s.rows = s.rows[:0]
	for _, it := range src.Items {
		r := syncRow{item: it, status: syncNew}
		if h, ok := have[it.ID]; ok {
			r.status = syncChanged
			if h == it.Hash {
				r.status = syncSame
			}
		}
		// Secrets go only when the user ticks them.
		r.checked = r.status != syncSame && len(it.Secrets) == 0 && len(it.Warnings) == 0
		s.rows = append(s.rows, r)
	}
	slices.SortStableFunc(s.rows, func(x, y syncRow) int {
		return agentRank(x.item.Agent) - agentRank(y.item.Agent)
	})
}

var syncAgentOrder = []string{"claude", "codex", "opencode", "gemini", "shared"}

func agentRank(id string) int {
	if i := slices.Index(syncAgentOrder, id); i >= 0 {
		return i
	}
	return len(syncAgentOrder)
}

func (s *syncState) checkedRows() []syncRow {
	var out []syncRow
	for _, r := range s.rows {
		if r.checked {
			out = append(out, r)
		}
	}
	return out
}

// runSync copies the ticked items one by one, so a large skill needs one
// frame, not the whole environment, and one failure stops nothing else.
func (a *App) runSync(s *syncState) {
	rows := s.checkedRows()
	src, dst := s.local, s.remote
	if !s.toRemote {
		src, dst = s.remote, s.local
	}
	dstName := s.destName()
	s.running, s.finished, s.done, s.total = true, false, 0, len(rows)
	s.warnings, s.failures = nil, nil
	go func() {
		for _, r := range rows {
			p, err := src.EnvExport(r.item.ID)
			var res proto.EnvImportResult
			if err == nil {
				res, err = dst.EnvImport(p)
			}
			a.win.Update(func() {
				if a.sync != s {
					return
				}
				s.done++
				if err != nil {
					s.failures = append(s.failures, itemTitle(r.item)+": "+strings.TrimPrefix(err.Error(), "envsync: "))
				}
				for _, w := range res.Warnings {
					s.warnings = append(s.warnings, strings.ReplaceAll(w, "here", "on "+dstName))
				}
			})
		}
		after, err := dst.EnvScan()
		a.win.Update(func() {
			if a.sync != s {
				return
			}
			s.running, s.finished = false, true
			if err == nil {
				s.scans[!s.toRemote] = after
				s.buildRows()
			}
		})
	}()
}

func (s *syncState) destName() string {
	if s.toRemote {
		return s.host.Name
	}
	return LocalHost().Name
}

func (a *App) closeSync() {
	if a.sync == nil {
		return
	}
	if a.sync.local != nil {
		a.sync.local.Close()
	}
	a.sync = nil
}

func (a *App) syncDialog(c *ui.Context) {
	s := a.sync
	if s == nil {
		return
	}
	open := true
	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(680).Padding(20).Gap(14).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi).
			Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.55))
		ui.Row(c).FillWidth().Children(func() {
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				ui.Text(c, "Sync environment").FontSize(16).Bold()
				ui.Text(c, "Skills, subagents, commands, memory files, settings and MCP servers of your agents, and your git identity.").
					FontSize(12).TextColor(colMuted)
			})
			if iconButton(c, iconClose, "Close", false).Clicked() {
				open = false
			}
		})
		a.syncDirection(c, s)
		switch {
		case s.loading:
			ui.Text(c, "Reading both sides…").TextColor(colMuted).Padding(24, 0)
		case s.err != "":
			ui.Text(c, s.err).FontSize(13).TextColor(colError)
		default:
			a.syncList(c, s)
		}
		a.syncFooter(c, s)
	})
	if !open && !s.running {
		a.closeSync()
	}
}

func (a *App) syncDirection(c *ui.Context, s *syncState) {
	local, remote := LocalHost().Name, s.host.Name
	ui.Grid(c).FillWidth().Columns(2).Gap(8).Children(func() {
		for _, toRemote := range []bool{true, false} {
			from, to := local, remote
			if !toRemote {
				from, to = remote, local
			}
			card := ui.ButtonBase(c).Gap(8).Padding(10, 12).Radius(9).Border(1, colBorderHi).Cursor(ui.CursorPointer).
				Label(from + " to " + to)
			if s.toRemote == toRemote {
				card.Background(colSelected).BorderColor(colAccent)
			} else if card.Hovered() {
				card.Background(colHover)
			}
			card.Children(func() {
				ui.Text(c, from).Bold().SingleLine()
				ui.Text(c, "→").TextColor(colMuted)
				ui.Text(c, to).Bold().SingleLine()
			})
			if card.Clicked() && !s.running && s.toRemote != toRemote {
				s.toRemote, s.finished = toRemote, false
				if s.scans != nil {
					s.buildRows()
				}
			}
		}
	})
}

func (a *App) syncList(c *ui.Context, s *syncState) {
	if len(s.rows) == 0 {
		ui.Text(c, "Nothing to sync: "+a.syncSourceName(s)+" has no skills, commands, memory files or MCP servers yet.").
			TextColor(colMuted).Padding(16, 0)
		return
	}
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		n := len(s.checkedRows())
		ui.Text(c, fmt.Sprintf("%d of %d selected", n, len(s.rows))).FontSize(12).TextColor(colMuted).Grow(1)
		if ghostButton(c, "Select changes").Clicked() && !s.running {
			for i := range s.rows {
				r := &s.rows[i]
				r.checked = r.status != syncSame && len(r.item.Secrets) == 0 && len(r.item.Warnings) == 0
			}
		}
		if ghostButton(c, "Clear").Clicked() && !s.running {
			for i := range s.rows {
				s.rows[i].checked = false
			}
		}
	})
	ui.Scroll(c).FillWidth().MaxHeight(380).Children(func() {
		ui.Column(c).FillWidth().Gap(2).Children(func() {
			agent := ""
			for i := range s.rows {
				r := &s.rows[i]
				if r.item.Agent != agent {
					agent = r.item.Agent
					ui.Row(c.Key("h"+agent)).Gap(8).Padding(10, 4, 4, 4).Children(func() {
						ui.Icon(c, syncAgentIcon(agent)).FontSize(13).TextColor(colTextSoft)
						ui.Text(c, a.syncAgentName(agent)).FontSize(12).Bold().TextColor(colTextSoft)
					})
				}
				a.syncRowView(c.Key(r.item.ID), s, r)
			}
		})
	})
}

func (a *App) syncRowView(c *ui.Context, s *syncState, r *syncRow) {
	row := ui.Row(c).FillWidth().Gap(10).Padding(8, 10).Radius(8).AlignItems(ui.Start)
	if r.checked {
		row.Background(colRaised)
	}
	row.Children(func() {
		box := ui.CheckboxBase(c, &r.checked).Label(itemTitle(r.item)).Size(16, 16).Radius(4).Center().
			Margin(2, 0, 0, 0).Shrink(0).Cursor(ui.CursorPointer).Border(1.5, colBorderHi)
		if r.checked {
			box.Background(colAccent).BorderColor(colAccent).TextColor(colOnAccent).Children(func() {
				ui.Icon(c, iconCheck).FontSize(11)
			})
		} else if box.Hovered() {
			box.BorderColor(colTextSoft)
		}
		if s.running {
			box.Disabled(true)
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, itemTitle(r.item)).Bold().SingleLine().Shrink(1)
				chip(c, kindLabel(r.item.Kind), colTextSoft, ui.Hex("#22262E"))
				ui.Box(c).Grow(1)
				switch r.status {
				case syncNew:
					chip(c, syncNew, colAccent, ui.RGBA(122, 162, 255, 0.12))
				case syncChanged:
					chip(c, syncChanged, ui.Hex("#F2C15B"), ui.RGBA(242, 193, 91, 0.12))
				default:
					chip(c, syncSame, colMuted, ui.Hex("#1C1F26"))
				}
			})
			if r.item.Detail != "" {
				ui.Text(c, r.item.Detail).Font(monoFont).FontSize(11).TextColor(colMuted).SingleLine()
			}
			if len(r.item.Secrets) > 0 {
				what := strings.Join(r.item.Secrets, ", ")
				msg, col := "Has secrets ("+what+"); tick it to send them to "+s.destName()+".", colMuted
				if r.checked {
					msg, col = "Sends "+what+" to "+s.destName()+".", ui.Hex("#F2C15B")
				}
				ui.Row(c).Gap(6).Children(func() {
					ui.Icon(c, iconLock).FontSize(11).TextColor(col)
					ui.Text(c, msg).FontSize(11).TextColor(col)
				})
			}
			for _, w := range r.item.Warnings {
				ui.Text(c, w).FontSize(11).TextColor(colError)
			}
		})
	})
}

func (a *App) syncFooter(c *ui.Context, s *syncState) {
	switch {
	case s.running:
		ui.Text(c, fmt.Sprintf("Syncing %d of %d…", min(s.done+1, s.total), s.total)).FontSize(12).TextColor(colTextSoft)
	case s.finished:
		summary := fmt.Sprintf("Synced %d of %d to %s.", s.total-len(s.failures), s.total, s.destName())
		ui.Text(c, summary+" Restart running agents there to pick up the changes.").FontSize(12).TextColor(colDone)
		ui.Column(c).FillWidth().Gap(3).Children(func() {
			for _, f := range s.failures {
				ui.Text(c, f).FontSize(11).TextColor(colError)
			}
			for _, w := range s.warnings {
				ui.Text(c, w).FontSize(11).TextColor(ui.Hex("#F2C15B"))
			}
		})
	default:
		ui.Text(c, "Agent logins aren't synced: sign in to each agent once on "+s.destName()+". Paths in your home folder move to theirs, and changed config files keep a .enowx.bak copy.").
			FontSize(11).TextColor(colMuted)
	}
	ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
		if ghostButton(c, "Close").Clicked() && !s.running {
			a.closeSync()
		}
		n := len(s.checkedRows())
		label := fmt.Sprintf("Sync %d to %s", n, s.destName())
		if n == 1 {
			label = "Sync 1 item to " + s.destName()
		}
		b := primaryButton(c, iconSync, label, "")
		if n == 0 || s.running || s.loading {
			b.Disabled(true).Opacity(0.5)
		} else if b.Clicked() {
			a.runSync(s)
		}
	})
}

func (a *App) syncSourceName(s *syncState) string {
	if s.toRemote {
		return LocalHost().Name
	}
	return s.host.Name
}

func (a *App) syncAgentName(id string) string {
	switch id {
	case "shared":
		return "Shared"
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "opencode":
		return "OpenCode"
	case "gemini":
		return "Gemini CLI"
	}
	return a.agentName(id)
}

func syncAgentIcon(id string) *ui.SVG {
	if id == "shared" {
		return iconFolder
	}
	return agentIcon(id)
}

// itemTitle names an item for people: commands and subagents without
// their .md, the git identity as such.
func itemTitle(it proto.EnvItem) string {
	switch it.Kind {
	case proto.EnvGit:
		return "Git identity"
	case proto.EnvCommand:
		return "/" + strings.TrimSuffix(strings.TrimSuffix(it.Name, ".md"), ".toml")
	case proto.EnvSubagent:
		return strings.TrimSuffix(it.Name, ".md")
	}
	return it.Name
}

func kindLabel(kind string) string {
	switch kind {
	case proto.EnvSkill:
		return "Skill"
	case proto.EnvSubagent:
		return "Subagent"
	case proto.EnvCommand:
		return "Command"
	case proto.EnvMemory:
		return "Memory"
	case proto.EnvSettings:
		return "Settings"
	case proto.EnvMCP:
		return "MCP server"
	case proto.EnvGit:
		return "Git"
	}
	return kind
}

func chip(c *ui.Context, label string, fg, bg ui.Color) {
	ui.Box(c).PaddingX(7).Height(18).Radius(5).Background(bg).Center().Shrink(0).Children(func() {
		ui.Text(c, label).FontSize(10.5).TextColor(fg).SingleLine()
	})
}
