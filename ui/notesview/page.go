package notesview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// pageView is the page open: where it is, its icon and title, its body.
func (v *View) pageView(c *ui.Context, o Options) {
	p := v.page
	ui.Row(c).FillWidth().Height(40).Shrink(0).PaddingX(16).Gap(6).AlignItems(ui.Center).Children(func() {
		v.breadcrumb(c, o, p.id)
		ui.Spacer(c)
		state := ""
		switch {
		case v.err != "":
			ui.Text(c, v.err).FontSize(11).TextColor(theme.Error).SingleLine().Shrink(1)
		case p.saving || p.dirty:
			state = "Saving…"
		case !p.note.Updated.IsZero():
			state = "Saved"
		}
		ui.Text(c, state).FontSize(11).TextColor(theme.Muted).SingleLine()
		if o.Assistant != nil {
			label := "Ask an agent"
			if v.assistant {
				label = "Hide the agent"
			}
			b := widget.Button(c, widget.Secondary, icon.Chat, label, "").Height(28).PaddingX(10)
			if v.assistant {
				b.Background(theme.Selected)
			}
			if b.Clicked() {
				v.assistant = !v.assistant
			}
		}
		more := widget.IconButton(c, icon.More, "More for this page", false)
		v.menu.OpenBelow(more, "page", 28)
		v.menu.Show(c, more, "page", func(done func()) { v.pageItems(c, o, p.note.NoteInfo, done) })
	})
	if p.ed == nil {
		ui.Text(c, "Opening…").FontSize(12).TextColor(theme.Muted).Padding(40)
		return
	}
	ui.Row(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Scroll(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			ui.Column(c).FillWidth().MaxWidth(760).AlignSelf(ui.Center).Padding(28, 48, 0, 48).Gap(6).Children(func() {
				v.undoBar(c, o)
				for _, pr := range v.proposalsFor(o, p.id) {
					v.proposalCard(c.Key(pr.ID), o, pr, p.ed.markdown())
				}
				v.iconButton(c, p)
				title := ui.TextAreaBase(c, &p.title).FillWidth().Lines(1, 4).PaddingX(6).FontSize(32).FontWeight(750).
					TextColor(theme.TextStrong).Placeholder("Untitled").Label("Title")
				title.OnTextKey(func(k ui.TextKey) bool {
					if k.Key != ui.KeyEnter && k.Key != ui.KeyDown {
						return false
					}
					p.ed.openAt(0, 0) // the body goes on from the title
					return true
				})
				if title.Changed() {
					p.ed.changed = true
				}
				ui.Box(c).Height(8)
				p.ed.draw(c)
			})
		})
		if v.assistant && o.Assistant != nil {
			ui.Column(c).Width(400).Shrink(0).FillHeight().Background(theme.Panel).BorderWidth(0, 0, 0, 1).BorderColor(theme.Border).Children(func() {
				o.Assistant(c, p.note.NoteInfo)
			})
		}
	})
}

// proposalsFor are the changes proposed to page id, and the new pages its
// agent proposed.
func (v *View) proposalsFor(o Options, id string) []Proposal {
	var out []Proposal
	for _, p := range o.Proposals {
		if p.Note.ID == id || p.Note.ID == "" && p.For == id {
			out = append(out, p)
		}
	}
	return out
}

// icons are the emoji a page's icon is picked from.
var icons = strings.Fields("📝 📌 📋 ✅ 🗒️ 📚 💡 🎯 🚀 🐛 🔧 ⚙️ 🧪 📦 🔒 🌐 📱 💻 🗂️ 📅 ⭐ 🔥 🧠 🎨 📈 💬 🤖 🏠 ✈️ ❤️")

// iconButton shows the page's icon, which a click picks.
func (v *View) iconButton(c *ui.Context, p *page) {
	label := p.icon
	b := ui.ButtonBase(c).Height(44).MinWidth(44).AlignSelf(ui.Start).Radius(8).Center().Cursor(ui.CursorPointer).Label("Page icon")
	if b.Hovered() {
		b.Background(theme.Hover)
	}
	b.Children(func() {
		if label != "" {
			ui.Text(c, label).FontSize(34)
		} else {
			ui.Text(c, "Add icon").FontSize(12).TextColor(theme.Muted).PaddingX(8)
		}
	})
	if b.Clicked() {
		p.icons = !p.icons
	}
	ui.Popover(c, b, &p.icons, func() {
		ui.Column(c).Width(276).Padding(8).Gap(6).Children(func() {
			ui.Row(c).FillWidth().Gap(2).Wrap().Children(func() {
				for _, e := range icons {
					btn := ui.ButtonBase(c.Key(e)).Size(32, 32).Radius(6).Center().Cursor(ui.CursorPointer).Label(e)
					if btn.Hovered() {
						btn.Background(theme.Hover)
					}
					btn.Children(func() { ui.Text(c, e).FontSize(18) })
					if btn.Clicked() {
						p.icon, p.icons, p.ed.changed = e, false, true
					}
				}
			})
			if p.icon != "" && widget.GhostButton(c, "Remove").Clicked() {
				p.icon, p.icons, p.ed.changed = "", false, true
			}
		})
	})
}

// breadcrumb is the pages the page is under, each opening it.
func (v *View) breadcrumb(c *ui.Context, o Options, id string) {
	byID := map[string]proto.NoteInfo{}
	for _, n := range o.Notes {
		byID[n.ID] = n
	}
	var path []proto.NoteInfo
	for n, ok := byID[id]; ok && len(path) < 16; n, ok = byID[n.Parent] {
		path = append([]proto.NoteInfo{n}, path...)
	}
	ui.Row(c).Gap(2).MinWidth(0).Shrink(1).AlignItems(ui.Center).Children(func() {
		for i, n := range path {
			if i > 0 {
				ui.Text(c, "/").FontSize(11.5).TextColor(theme.Muted)
			}
			b := ui.ButtonBase(c.Key(n.ID)).Height(24).PaddingX(6).Radius(5).Gap(4).Cursor(ui.CursorPointer).Label(titleOf(n)).Shrink(1).MinWidth(0)
			if b.Hovered() {
				b.Background(theme.Hover)
			}
			b.Children(func() {
				if n.Icon != "" {
					ui.Text(c, n.Icon).FontSize(12)
				}
				ui.Text(c, titleOf(n)).FontSize(11.5).TextColor(theme.TextSoft).SingleLine().Shrink(1)
			})
			if b.Clicked() {
				v.Select(o, n.ID)
			}
		}
	})
}
