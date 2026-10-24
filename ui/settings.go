package ui

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/core/hosts"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/libraryview"
	"github.com/enowx/enowx/ui/widget"
)

// The Settings page is the library of what the agents on the connected
// host use (package libraryview): skills, MCP servers, subagents,
// commands, instructions, settings.

const pageSettings = "settings"

// libraryOptions connect the library to the connected host.
func (a *App) libraryOptions() libraryview.Options {
	ed := a.preferredEditor()
	return libraryview.Options{
		Client:       a.client,
		Host:         a.host.Name,
		Update:       a.onMain,
		OpenInEditor: func(path string) { a.openInEditor(ed, path, 0) },
	}
}

func (a *App) settingsPage(c *ui.Context) {
	if a.library == nil || a.libraryHost != a.host.ID {
		a.library, a.libraryHost = libraryview.New(), a.host.ID
	}
	if a.settingsSection == "" {
		a.settingsSection = sectionLibrary
	}
	a.viewHeader(c, func() {
		ui.Row(c).Grow(1).MinWidth(0).Children(func() { widget.Breadcrumb(c, "Settings", a.host.Name) })
		if i := widget.Segmented(c.Key("sections"), settingsSections, slices.Index(settingsSections, a.settingsSection), nil); i >= 0 {
			a.settingsSection = settingsSections[i]
			if a.hostSet != nil && i > 0 {
				a.loadHostSettings()
			}
		}
		if a.settingsSection == sectionLibrary && a.host.ID != hosts.LocalID && a.client != nil && widget.GhostButton(c, "Sync…").Tooltip("Copy items to another host").Clicked() {
			a.openSync()
		}
		if widget.IconButton(c, icon.Close, "Back to sessions", false).Clicked() {
			a.page = ""
		}
	})
	a.settingsBody(c)
}
