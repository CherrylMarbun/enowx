package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/agent"
	"github.com/enowx/enowx/proto"
)

// Agents missing on a host are installed in a session there, so the
// installer's output and questions stay in view, local or remote alike.

// needsInstall reports whether the chosen agent is missing on the host.
func needsInstall(ag proto.AgentInfo) bool { return ag.Command != "" && !ag.Installed }

// installCard tells what installing the agent runs.
func (a *App) installCard(c *ui.Context, ag proto.AgentInfo) {
	ui.Column(c).FillWidth().Gap(8).Padding(12, 14).Radius(10).Background(ui.RGBA(242, 193, 91, 0.06)).
		Border(1, ui.RGBA(242, 193, 91, 0.25)).Children(func() {
		ui.Text(c, ag.Name+" isn't installed on "+a.host.Name+".").Bold()
		if ag.Install == "" {
			ui.Text(c, "Install "+ag.Command+" there, then pick it again.").FontSize(12).TextColor(colTextSoft)
			return
		}
		ui.Text(c, "Install runs its official installer in a session here, as the user enowX runs as:").FontSize(12).TextColor(colTextSoft)
		ui.Text(c, ag.Install).Font(monoFont).FontSize(11).Padding(8, 10).Radius(7).Background(colBackground).FillWidth()
	})
}

// installAgent starts the installer as a session in the project's folder.
func (a *App) installAgent(ag proto.AgentInfo) {
	f := &a.newSession
	cl := a.client
	if cl == nil || ag.Install == "" {
		return
	}
	params := proto.SessionCreateParams{
		Project:    f.project.Path,
		Name:       "Install " + ag.Name,
		Agent:      agent.Custom,
		BranchMode: proto.BranchNone,
		Command:    []string{"sh", "-c", ag.Install},
	}
	f.busy, f.err = true, ""
	go func() {
		info, err := cl.CreateSession(params)
		a.win.Update(func() {
			f.busy = false
			if err != nil {
				f.err = strings.TrimPrefix(err.Error(), proto.MethodSessionCreate+": ")
				return
			}
			f.open = false
			if a.installs == nil {
				a.installs = map[string]proto.AgentInfo{}
			}
			a.installs[info.ID] = ag
			a.upsert(info)
			a.selectSession(info)
		})
	}()
}

// installEnded checks, once an install session exits, whether the agent
// is there now.
func (a *App) installEnded(info proto.SessionInfo) {
	ag, ok := a.installs[info.ID]
	if !ok || !info.Exited {
		return
	}
	delete(a.installs, info.ID)
	cl, gen := a.client, a.gen
	if cl == nil {
		return
	}
	go func() {
		agents, err := cl.Agents()
		a.win.Update(func() {
			if gen != a.gen || err != nil {
				return
			}
			a.agents = agents
			if a.agentInfo(ag.ID).Installed {
				a.notify(ag.Name + " is installed. Start a session with it from New session.")
			} else {
				a.status = ag.Name + " didn't install; its session shows why."
			}
		})
	}()
}
