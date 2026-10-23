package libraryview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/client"
	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// kindPlugins is the library's Plugins: Claude Code's, read from its
// files, changed through its CLI on the host (env.plugins.run).
const kindPlugins = "plugins"

// pluginState is the Plugins panel's.
type pluginState struct {
	list    proto.ClaudePlugins
	loaded  bool
	running string // the action under way, as said to the user
	output  string // what the CLI said last
	failed  bool
	install string // a plugin to install: "name@marketplace"
	market  string // a marketplace to add: "owner/repo", a URL
}

func (v *View) loadPlugins(o Options) {
	cl := o.Client
	if cl == nil {
		return
	}
	go func() {
		list, err := cl.Plugins()
		o.Update(func() {
			v.plugins.loaded = true
			if err != nil {
				v.plugins.output, v.plugins.failed = client.Explain(err), true
				return
			}
			v.plugins.list = list
		})
	}()
}

// runPlugin has the CLI do action to arg, then reads the plugins again.
func (v *View) runPlugin(o Options, action, arg, doing string) {
	cl := o.Client
	if cl == nil || v.plugins.running != "" {
		return
	}
	v.plugins.running, v.plugins.output = doing, ""
	go func() {
		res, err := cl.PluginRun(action, arg)
		o.Update(func() {
			v.plugins.running = ""
			switch {
			case err != nil:
				v.plugins.output, v.plugins.failed = client.Explain(err), true
			default:
				v.plugins.output, v.plugins.failed = res.Output, !res.OK
			}
			v.loadPlugins(o)
		})
	}()
}

// pluginsPanel shows the plugins and marketplaces, in the list's and the
// detail's room.
func (v *View) pluginsPanel(c *ui.Context, o Options) {
	ps := &v.plugins
	if !ps.loaded {
		ps.loaded = true // asked once; a reply sets it again
		v.loadPlugins(o)
	}
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(820).Padding(16, 20).Gap(14).Children(func() {
			ui.Text(c, "Claude Code's plugins bundle skills, commands, agents, hooks and MCP servers. Installing and enabling them goes through its own CLI on "+o.Host+".").
				FontSize(12).TextColor(theme.Muted)
			if !ps.list.CLI && len(ps.list.Plugins) > 0 {
				ui.Text(c, "Claude Code isn't on "+o.Host+"'s PATH: plugins show, but can't be changed.").FontSize(12).TextColor(theme.Warning)
			}
			if ps.running != "" {
				ui.Row(c).Gap(8).Children(func() {
					widget.Spinner(c, 14)
					ui.Text(c, ps.running+"…").FontSize(12).TextColor(theme.Muted)
				})
			} else if ps.output != "" {
				col := theme.Muted
				if ps.failed {
					col = theme.Error
				}
				ui.Text(c, ps.output).Font(theme.MonoFont).FontSize(11).TextColor(col).Padding(8, 10).Radius(6).Background(theme.Sunken).FillWidth()
			}
			widget.SectionLabel(c, "INSTALLED", theme.Muted)
			if len(ps.list.Plugins) == 0 {
				ui.Text(c, "None yet.").FontSize(12).TextColor(theme.Muted)
			}
			for _, p := range ps.list.Plugins {
				v.pluginRow(c.Key(p.ID), o, p)
			}
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				add := widget.Input(c, &ps.install, "plugin@marketplace").FillWidth().Font(theme.MonoFont).Submitted()
				if (widget.Button(c, widget.Secondary, icon.Plus, "Install", "").Clicked() || add) && strings.TrimSpace(ps.install) != "" {
					v.runPlugin(o, proto.PluginInstall, ps.install, "Installing "+ps.install)
					ps.install = ""
				}
			})
			widget.SectionLabel(c, "MARKETPLACES", theme.Muted).Margin(8, 0, 0, 0)
			for _, m := range ps.list.Marketplaces {
				ui.Row(c.Key("m:"+m.Name)).FillWidth().Gap(10).Padding(6, 10).Radius(6).Background(theme.Sunken).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
						ui.Text(c, m.Name).FontSize(12.5)
						ui.Text(c, m.Source).Font(theme.MonoFont).FontSize(11).TextColor(theme.Muted).SingleLine()
					})
					if widget.IconButton(c, icon.Sync, "Update it", false).Clicked() {
						v.runPlugin(o, proto.PluginMarketplaceUpdate, m.Name, "Updating "+m.Name)
					}
					if widget.IconButton(c, icon.Trash, "Remove it", false).Clicked() {
						v.runPlugin(o, proto.PluginMarketplaceRemove, m.Name, "Removing "+m.Name)
					}
				})
			}
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				add := widget.Input(c, &ps.market, "owner/repo, or a URL").FillWidth().Font(theme.MonoFont).Submitted()
				if (widget.Button(c, widget.Secondary, icon.Plus, "Add", "").Clicked() || add) && strings.TrimSpace(ps.market) != "" {
					v.runPlugin(o, proto.PluginMarketplaceAdd, ps.market, "Adding "+ps.market)
					ps.market = ""
				}
			})
		})
	})
}

func (v *View) pluginRow(c *ui.Context, o Options, p proto.ClaudePlugin) {
	ui.Row(c).FillWidth().Gap(10).Padding(8, 10).Radius(6).Background(theme.Sunken).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Text(c, p.Name).FontSize(12.5)
				ui.Text(c, p.Marketplace+" · "+p.Version).FontSize(11).TextColor(theme.Muted).SingleLine()
			})
			if p.Description != "" {
				ui.Text(c, p.Description).FontSize(11.5).TextColor(theme.TextSoft).MaxLines(2)
			}
		})
		on := p.Enabled
		if widget.Switch(c, &on, "Enabled") {
			action, doing := proto.PluginDisable, "Disabling "+p.Name
			if on {
				action, doing = proto.PluginEnable, "Enabling "+p.Name
			}
			v.runPlugin(o, action, p.ID, doing)
		}
		if widget.IconButton(c, icon.Sync, "Update it", false).Clicked() {
			v.runPlugin(o, proto.PluginUpdate, p.ID, "Updating "+p.Name)
		}
		if widget.IconButton(c, icon.Trash, "Uninstall it", false).Clicked() {
			v.runPlugin(o, proto.PluginUninstall, p.ID, "Uninstalling "+p.Name)
		}
	})
}
