package ui

import (
	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/syncview"
	"github.com/enowx/enowx/ui/widget"
)

// openSync shows the Sync environment page for the connected server,
// reading both sides afresh unless a sync is under way.
func (a *App) openSync() {
	if a.client == nil || a.host.ID == hosts.LocalID {
		return
	}
	a.page = pageSync
	if a.sync != nil && a.sync.Running() {
		return
	}
	a.closeSync()
	a.page = pageSync
	a.sync = syncview.Open(syncview.Options{
		Host:          a.host,
		Remote:        a.client,
		Local:         LocalHost(),
		DaemonCommand: a.opts.DaemonCommand,
		Update:        a.onMain,
		AgentName:     a.agentName,
	})
}

func (a *App) closeSync() {
	if a.sync != nil {
		a.sync.Close()
		a.sync = nil
	}
	if a.page == pageSync {
		a.page = ""
	}
}

// syncPage is the Sync environment page; its Close button returns to the
// sessions.
func (a *App) syncPage(c *ui.Context) {
	a.viewHeader(c, func() {
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			widget.Breadcrumb(c, "Sync environment", LocalHost().Name+" and "+a.host.Name)
		})
		if widget.IconButton(c, icon.Close, "Back to sessions", false).Clicked() {
			a.page = ""
		}
	})
	a.sync.View(c)
	if a.sync.Closed() {
		a.sync = nil
		a.page = ""
	}
}
