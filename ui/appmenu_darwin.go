package ui

import "github.com/egoist/mygo"

// setAppMenu is macOS's menu bar with "Quit and Stop All Agents" beside
// Quit, where people look for it.
func setAppMenu(a *App) {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Label: "Quit and Stop All Agents", Accelerator: "CmdOrCtrl+Alt+Q", Click: func(*mygo.MenuItem, *mygo.Window) { a.quitAndStopAgents() }},
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleFileMenu},
		{Role: mygo.RoleEditMenu},
		{Role: mygo.RoleViewMenu},
		{Role: mygo.RoleWindowMenu},
	}))
}
