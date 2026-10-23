package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/textfmt"
	"github.com/enowx/enowx/ui/usageview"
	"github.com/enowx/enowx/ui/widget"
)

// pollUsage measures while connection gen lasts (see package usageview).
func (a *App) pollUsage(cl *client.Client, gen int) {
	a.usage.Follow(cl, a.win.Update, func() bool { return gen == a.gen })
}

// usageHost describes the connected host to the usage views.
func (a *App) usageHost() usageview.Host {
	return usageview.Host{
		Name:      a.host.Name,
		Local:     a.host.ID == hosts.LocalID,
		LocalName: LocalHost().Name,
		Session: func(id string) (name, agent, project string) {
			for _, s := range a.sessions {
				if s.ID == id {
					if s.Project != "" {
						project = textfmt.LastPathPart(s.Project)
					}
					return a.sessionName(s), s.Agent, project
				}
			}
			return id, "", ""
		},
	}
}

// usagePage is the Resource usage page of the connected host.
func (a *App) usagePage(c *ui.Context) {
	a.viewHeader(c, func() {
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			widget.Breadcrumb(c, a.host.Name, "Resource usage")
		})
		if widget.IconButton(c, icon.Close, "Back to sessions", false).Clicked() {
			a.page = ""
		}
	})
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(16).Children(func() {
			a.usage.Details(c, a.usageHost())
		})
	})
}
