package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

// newSessionForm is the state of the New session dialog.
type newSessionForm struct {
	open    bool
	project proto.ProjectDetail
	loaded  bool // project details arrived

	agent      string
	command    string // for the custom agent
	branchMode string // proto.BranchNew or proto.BranchExisting on git projects
	branch     string // name of the new branch
	base       string
	existing   string
	prompt     string

	busy bool
	err  string
}

func (a *App) openNewSession(p proto.ProjectInfo) {
	f := &a.newSession
	prevAgent := f.agent
	*f = newSessionForm{open: true, project: proto.ProjectDetail{ProjectInfo: p}, agent: prevAgent}
	if f.agent == "" {
		f.agent = a.defaultAgent()
	}
	if p.IsGit {
		f.branchMode = proto.BranchNew
	}
	go func() {
		d, err := a.client.InspectProject(p.Path)
		a.win.Update(func() {
			if !f.open || f.project.Path != p.Path {
				return
			}
			if err != nil {
				f.err = err.Error()
				return
			}
			f.project, f.loaded = d, true
			f.base = d.Branch
			if f.base == "" && len(d.Branches) > 0 {
				f.base = d.Branches[0]
			}
			if len(d.Branches) > 0 {
				f.existing = d.Branches[0]
			}
			if !d.IsGit {
				f.branchMode = proto.BranchNone
			}
		})
	}()
}

// defaultAgent is the first installed agent, else the shell.
func (a *App) defaultAgent() string {
	for _, ag := range a.agents {
		if ag.Installed && ag.Command != "" {
			return ag.ID
		}
	}
	return agent.Shell
}

func (a *App) agentInfo(id string) proto.AgentInfo {
	for _, ag := range a.agents {
		if ag.ID == id {
			return ag
		}
	}
	return proto.AgentInfo{ID: id, Name: id}
}

func (a *App) startSession() {
	f := &a.newSession
	params := proto.SessionCreateParams{
		Project:    f.project.Path,
		Agent:      f.agent,
		BranchMode: f.branchMode,
		Prompt:     f.prompt,
	}
	switch f.branchMode {
	case proto.BranchNew:
		params.Branch, params.Base = strings.TrimSpace(f.branch), f.base
		if params.Branch == "" {
			f.err = "Name the new branch."
			return
		}
	case proto.BranchExisting:
		params.Branch = f.existing
	}
	if f.agent == agent.Custom {
		params.Command = strings.Fields(f.command)
		if len(params.Command) == 0 {
			f.err = "Type the command to run."
			return
		}
	}
	f.busy, f.err = true, ""
	go func() {
		info, err := a.client.CreateSession(params)
		a.win.Update(func() {
			f.busy = false
			if err != nil {
				f.err = strings.TrimPrefix(err.Error(), proto.MethodSessionCreate+": ")
				return
			}
			f.open = false
			a.upsert(info)
			a.selectSession(info)
		})
	}()
}

func (a *App) newSessionDialog(c *ui.Context) {
	f := &a.newSession
	if !f.open {
		return
	}
	ui.DialogBase(c, &f.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(560).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi).
			Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.55))

		ui.Row(c).FillWidth().Padding(18, 20, 14, 20).Gap(10).Children(func() {
			ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, "New session").FontSize(16).Bold()
				ui.Text(c, f.project.Name+" · "+f.project.Path).FontSize(12).TextColor(colMuted).SingleLine()
			})
			if iconButton(c, iconClose, "Close", false).Clicked() {
				f.open = false
			}
		})

		ui.Column(c).FillWidth().Padding(4, 20, 20, 20).Gap(18).Children(func() {
			a.agentPicker(c)
			if f.agent == agent.Custom {
				field(c, "Command", func() {
					ui.TextInput(c, &f.command).Font(monoFont).Placeholder("npx some-agent --flag")
				})
			}
			if f.project.IsGit {
				a.branchFields(c)
			} else if f.loaded {
				a.folderNotice(c)
			}
			if f.agent != agent.Shell {
				field(c, "First prompt (optional)", func() {
					ui.TextArea(c, &f.prompt).Height(76).Placeholder("What should the agent do?")
				})
			}
			if f.err != "" {
				ui.Text(c, f.err).FontSize(12).TextColor(colError)
			}
		})

		ui.Row(c).FillWidth().Padding(14, 20).Gap(8).BorderWidth(1, 0, 0, 0).BorderColor(ui.Hex("#23262D")).Children(func() {
			ui.Text(c, modKey+"↩ to start · Esc to cancel").FontSize(12).TextColor(colMuted)
			ui.Spacer(c)
			if ghostButton(c, "Cancel").Clicked() {
				f.open = false
			}
			label := "Start session"
			if f.busy {
				label = "Starting…"
			}
			if primaryButton(c, iconCheck, label, "").Disabled(f.busy).Clicked() || c.Shortcut(ui.Cmd, ui.KeyEnter) {
				if !f.busy {
					a.startSession()
				}
			}
		})
	})
}

func field(c *ui.Context, label string, fn func()) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, label).FontSize(12).Bold().TextColor(colTextSoft)
		fn()
	})
}

// agentPicker is a drop-down of agents with their logos.
func (a *App) agentPicker(c *ui.Context) {
	f := &a.newSession
	field(c, "Agent", func() {
		sel := ui.SelectBase(c, &f.agent)
		cur := a.agentInfo(f.agent)
		trig := sel.Trigger.FillWidth().Height(44).PaddingX(12).Gap(10).Radius(10).
			Background(colBackground).Border(1, ui.Hex("#343944")).Cursor(ui.CursorPointer).Label("Agent")
		if sel.Open() {
			trig.Border(1, colAccent)
		}
		trig.Children(func() {
			agentTile(c, cur.ID)
			ui.Text(c, cur.Name).Bold().SingleLine()
			if cur.Command != "" {
				ui.Text(c, cur.Command).Font(monoFont).FontSize(12).TextColor(colMuted)
			}
			ui.Spacer(c)
			ui.Icon(c, iconChevronDown).FontSize(14).TextColor(colMuted)
		})
		sel.Popup(func(panel ui.Element) {
			panel.Margin(6, 0, 0, 0).Padding(6).Radius(12).Background(ui.Hex("#1B1E25")).
				Border(1, ui.Hex("#2E333C")).Shadow(0, 18, 40, 0, ui.RGBA(0, 0, 0, 0.55)).Width(518)
			for i, ag := range a.agents {
				if ag.ID == agent.Shell && i > 0 {
					ui.Box(c).Height(1).FillWidth().Margin(4, 6).Background(colBorderHi)
				}
				item := sel.Item(ag.ID).FillWidth().Gap(10).Padding(8, 10).Radius(8)
				if item.Highlighted() {
					item.Background(ui.Hex("#262B35"))
				}
				item.Children(func() {
					agentTile(c, ag.ID)
					ui.Column(c).Gap(1).Grow(1).Children(func() {
						ui.Text(c, ag.Name).Bold()
						sub := ag.Command
						switch ag.ID {
						case agent.Shell:
							sub = "your login shell"
						case agent.Custom:
							sub = "any program"
						}
						ui.Text(c, sub).Font(monoFont).FontSize(11).TextColor(colMuted)
					})
					switch {
					case !ag.Installed:
						ui.Text(c, "Not installed").FontSize(11).TextColor(colMuted)
					case ag.ID == f.agent:
						ui.Icon(c, iconCheck).FontSize(15).TextColor(colAccent)
					}
				})
			}
		})
	})
}

func agentTile(c *ui.Context, id string) {
	ui.Box(c).Size(26, 26).Radius(6).Background(ui.Hex("#22262E")).Center().TextColor(colText).Children(func() {
		ui.Icon(c, agentIcon(id)).FontSize(16)
	})
}

func (a *App) branchFields(c *ui.Context) {
	f := &a.newSession
	field(c, "Branch", func() {
		ui.Grid(c).FillWidth().Columns(2).Gap(8).Children(func() {
			modeCard(c, &f.branchMode, proto.BranchNew, "New branch", "In its own worktree, so it never touches your other sessions")
			modeCard(c, &f.branchMode, proto.BranchExisting, "Existing branch", "Continue work on a branch that already exists")
		})
		switch f.branchMode {
		case proto.BranchNew:
			ui.Grid(c).FillWidth().ColumnTracks(ui.Fr(1), ui.Fixed(180)).Gap(8).Margin(4, 0, 0, 0).Children(func() {
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, "Branch name").FontSize(12).TextColor(colMuted)
					ui.TextInput(c, &f.branch).Font(monoFont).Placeholder("feat/user-search").AutoFocus()
				})
				ui.Column(c).Gap(6).Children(func() {
					ui.Text(c, "From").FontSize(12).TextColor(colMuted)
					branchSelect(c, &f.base, f.project.Branches)
				})
			})
		case proto.BranchExisting:
			ui.Column(c).FillWidth().Gap(6).Margin(4, 0, 0, 0).Children(func() {
				ui.Text(c, "Branch").FontSize(12).TextColor(colMuted)
				branchSelect(c, &f.existing, f.project.Branches)
				if f.existing == f.project.Branch && f.existing != "" {
					ui.Text(c, "Checked out in the project folder, so the session runs there.").FontSize(12).TextColor(colMuted)
				}
			})
		}
	})
}

func modeCard(c *ui.Context, mode *string, value, title, body string) {
	on := *mode == value
	card := ui.ButtonBase(c).MinWidth(0).Padding(12).Radius(10).AlignItems(ui.Start).Cursor(ui.CursorPointer).Label(title)
	if on {
		card.Border(1, colAccent).Background(ui.Hex("#1C2233"))
	} else {
		card.Border(1, colBorderHi)
	}
	card.Children(func() {
		ui.Column(c).Gap(4).Grow(1).MinWidth(0).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ring := ui.Box(c).Size(14, 14).Radius(7).Border(1.5, colMuted).Center()
				if on {
					ring.Border(1.5, colAccent).Children(func() {
						ui.Box(c).Size(6, 6).Radius(3).Background(colAccent)
					})
				}
				ui.Text(c, title).Bold()
			})
			ui.Text(c, body).FontSize(12).TextColor(ui.Hex("#A3A8B2")).Margin(0, 0, 0, 22)
		})
	})
	if card.Clicked() {
		*mode = value
	}
}

func branchSelect(c *ui.Context, value *string, branches []string) {
	if len(branches) == 0 {
		ui.Text(c, "No branches yet").FontSize(12).TextColor(colMuted)
		return
	}
	ui.Select(c, value, branches).FillWidth().Font(monoFont).Label("Branch")
}

func (a *App) folderNotice(c *ui.Context) {
	ui.Row(c).FillWidth().Gap(12).Padding(14).Radius(10).Background(colPanel).Border(1, ui.Hex("#23262D")).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, iconFolder).FontSize(18).TextColor(colTextSoft)
		ui.Column(c).Gap(6).Grow(1).Children(func() {
			ui.Text(c, "This folder isn't a git repository").Bold()
			ui.Text(c, "The session runs directly in the folder. Sessions here share the same files, so two agents can overwrite each other's changes. Run git init in it to get branches and isolated worktrees.").
				FontSize(12).TextColor(ui.Hex("#A3A8B2"))
		})
	})
}

// addProjectForm asks for a path on a remote host, where the system's
// folder dialog can't browse.
type addProjectForm struct {
	open bool
	path string
}

func (a *App) addProjectDialog(c *ui.Context) {
	f := &a.addProject
	if !f.open {
		return
	}
	ui.DialogBase(c, &f.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(5, 6, 8, 0.62))
		panel.Width(480).Padding(20).Gap(14).Radius(14).Background(ui.Hex("#16181D")).Border(1, colBorderHi)
		ui.Text(c, "Open a folder on "+a.host.Name).FontSize(16).Bold()
		submit := false
		field(c, "Path on the server", func() {
			submit = ui.TextInput(c, &f.path).Font(monoFont).Placeholder("~/code/my-project").AutoFocus().Submitted()
		})
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if ghostButton(c, "Cancel").Clicked() {
				f.open = false
			}
			if primaryButton(c, iconFolderPlus, "Open", "").Clicked() {
				submit = true
			}
		})
		if submit && strings.TrimSpace(f.path) != "" {
			path := strings.TrimSpace(f.path)
			f.open, f.path = false, ""
			go a.addProjectPath(path)
		}
	})
}
