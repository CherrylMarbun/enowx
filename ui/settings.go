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
		crumb := ui.Row(c).Grow(1).MinWidth(0)
		crumb.Children(func() { widget.Breadcrumb(c, "Settings", a.host.Name) })
		pick := func(i int) {
			a.settingsSection = settingsSections[i]
			if a.hostSet != nil && i > 0 {
				a.loadHostSettings()
			}
		}
		// The sections' tabs while they fit beside the breadcrumb, else a
		// menu of them.
		if room := crumb.Bounds().W + a.settingsTabsW; a.settingsTabsW == 0 || room >= a.settingsTabsW+140 {
			tabs := ui.Row(c).Shrink(0)
			tabs.Children(func() {
				if i := widget.Segmented(c.Key("sections"), settingsSections, slices.Index(settingsSections, a.settingsSection), nil); i >= 0 {
					pick(i)
				}
			})
			a.settingsTabsW = tabs.Bounds().W
		} else {
			b := widget.Button(c, widget.Secondary, icon.ChevronDown, a.settingsSection, "").Height(30).Label("Settings section")
			a.menu.OpenBelow(b, "settings-sections", 30)
			a.menu.Show(c, b, "settings-sections", func(done func()) {
				for i, name := range settingsSections {
					if widget.MenuItem(c.Key(name), nil, name, false) {
						pick(i)
						done()
					}
				}
			})
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
