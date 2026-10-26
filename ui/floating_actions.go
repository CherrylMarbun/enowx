package ui

import (
	"github.com/enowx/enowx/ui/icon"
)

// floatingActions are the pie's wedges, in the order they ring it from the
// top, clockwise: Active sessions first, then the six. The panel toggles are
// one wedge, which opens them in a fan: they share a slice, and seven read as
// a cleaner pie than ten. One the app can't do yet shows dimmed and takes no
// click, as the Sessions panel's New does.
func (a *App) floatingActions() []floatingAction {
	_, project := a.currentProject()
	connected := a.client != nil
	return []floatingAction{
		{key: floatingActiveFan, label: "Active sessions", short: "Active sessions",
			svg: icon.List, enabled: true, fan: floatingActiveFan},
		{key: "new", label: "New session", short: "New session", svg: icon.Plus,
			enabled: connected && project, run: func() {
				if p, ok := a.currentProject(); ok {
					a.openNewSession(p)
				}
			}},
		{key: "open", label: "Open folder", short: "Open folder", svg: icon.FolderPlus,
			enabled: connected, run: a.openFolder},
		{key: floatingPanelsFan, label: "Panels", short: "Panels", svg: icon.Split,
			enabled: anyEnabled(a.floatingPanels()), fan: floatingPanelsFan},
		{key: "palette", label: "Command palette", short: "Palette", svg: icon.Search,
			enabled: true, run: a.openPalette},
		{key: "settings", label: "Settings", short: "Settings", svg: icon.Settings,
			enabled: connected, run: func() { a.page = pageSettings }},
		{key: "appearance", label: "Appearance", short: "Appearance", svg: icon.Palette,
			enabled: true, run: func() { a.page = pageAppearance }},
	}
}

// floatingSources are the children the Active sessions wedge fans out, in
// its order: this machine, then the servers. Both are always pickable: a
// source with nothing to show is an answer ("nothing is running there"), not
// a disabled control — dimming it made the wedge look dead on a machine whose
// sessions had all exited.
func (a *App) floatingSources() []floatingAction {
	return []floatingAction{
		{key: "source:pc", label: "This PC", short: "This PC", svg: icon.Laptop, enabled: true},
		{key: "source:vps", label: "VPS", short: "VPS", svg: icon.Server, enabled: true},
	}
}

// floatingPanels are the three panel toggles the one wedge opens, in the
// order they fan: the Sessions panel, the side panel, then the split view.
// Their conditions are the ones the pie gave them before it held seven.
func (a *App) floatingPanels() []floatingAction {
	_, project := a.currentProject()
	connected := a.client != nil
	return []floatingAction{
		{key: "sessions", label: "Show or hide the Sessions panel", short: "Sessions",
			svg: icon.Sidebar, enabled: true, run: a.toggleSessionsPanel},
		{key: "panel", label: "Show or hide the side panel", short: "Side panel",
			svg: icon.PanelRight, enabled: connected, run: a.togglePanel},
		{key: "split", label: "Sessions side by side", short: "Split",
			svg: icon.Split, enabled: project, run: func() { a.setSplit(!a.splitOn()) }},
	}
}

// floatingFanActs are the children of the fan key names, or none.
func (a *App) floatingFanActs(key string) []floatingAction {
	switch key {
	case floatingActiveFan:
		return a.floatingSources()
	case floatingPanelsFan:
		return a.floatingPanels()
	}
	return nil
}

// anyEnabled reports whether one of acts can be run: the wedge that holds
// them is usable as long as one is.
func anyEnabled(acts []floatingAction) bool {
	for _, act := range acts {
		if act.enabled {
			return true
		}
	}
	return false
}
