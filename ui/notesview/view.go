// Package notesview is the Notes page: the user's pages on the connected
// host, nested like a notebook's, each edited a block at a time. The app
// gives it the pages and how to read and write them through Options.
package notesview

import (
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// Options are what the page shows and does: the host's pages, and calls
// to it, whose done funcs run on the main thread (the app's Update).
type Options struct {
	Host   string // its name, said where notes are kept
	Notes  []proto.NoteInfo
	Loaded bool   // Notes came at least once
	Err    string // why they didn't
	Load   func(id string, done func(proto.Note, error))
	Save   func(n proto.Note, done func(proto.NoteInfo, error))
	Delete func(id string, done func(error))
	Move   func(id, parent, before string, done func(error))
}

// saveAfter is how long after the last change a page is saved.
const saveAfter = 700 * time.Millisecond

// View is the Notes page's state. It belongs to the main thread.
type View struct {
	selected string
	page     *page
	filter   string
	folded   map[string]bool // pages whose pages under them are hidden
	menu     widget.ContextMenu
	deleting *proto.NoteInfo
	err      string
}

// page is the page open: as last read, and as being edited.
type page struct {
	id      string
	note    proto.Note // as the host last had it
	title   string
	icon    string
	ed      *editor
	loading bool
	saving  bool
	dirty   bool
	edited  time.Time
	icons   bool // the icon picker is open
	listed  bool // the host's list had it: one made now isn't yet
}

// Select opens page id, as a link to it does.
func (v *View) Select(o Options, id string) {
	if v.page != nil && v.page.id == id {
		return
	}
	v.flush(o)
	v.selected = id
	v.page = &page{id: id, loading: true}
	p := v.page
	o.Load(id, func(n proto.Note, err error) {
		p.loading = false
		if err != nil {
			v.err = err.Error()
			return
		}
		p.note, p.title, p.icon, p.ed = n, n.Title, n.Icon, newEditor(n.Body)
	})
}

// Draw builds the page: the pages' tree beside the page open.
func (v *View) Draw(c *ui.Context, o Options) {
	v.follow(o)
	ui.Row(c).Grow(1).FillWidth().FillHeight().MinHeight(0).Children(func() {
		v.sidebar(c, o)
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Background(theme.Background).Children(func() {
			if v.page == nil {
				v.empty(c, o)
				return
			}
			v.pageView(c, o)
		})
	})
	v.deleteDialog(c, o)
	v.save(c, o)
}

// follow takes in what changed on the host: the page open, changed by
// another app or an agent while not being edited here, is read again; a
// page gone closes.
func (v *View) follow(o Options) {
	p := v.page
	if p == nil || p.loading || p.saving || p.dirty {
		return
	}
	for _, n := range o.Notes {
		if n.ID != p.id {
			continue
		}
		p.listed = true
		if n.Updated.After(p.note.Updated) && (p.ed == nil || p.ed.open < 0) {
			p.loading = true
			o.Load(p.id, func(full proto.Note, err error) {
				p.loading = false
				if err == nil && !p.dirty {
					p.note, p.title, p.icon, p.ed = full, full.Title, full.Icon, newEditor(full.Body)
				}
			})
		}
		return
	}
	if p.listed {
		v.page, v.selected = nil, "" // deleted
	}
}

// save writes the page open once it has rested from changes.
func (v *View) save(c *ui.Context, o Options) {
	p := v.page
	if p == nil || p.ed == nil {
		return
	}
	if p.ed.changed {
		p.ed.changed, p.dirty, p.edited = false, true, time.Now()
	}
	if !p.dirty || p.saving {
		return
	}
	if wait := saveAfter - time.Since(p.edited); wait > 0 {
		c.After(wait)
		return
	}
	v.write(o, p)
}

// flush saves the page open now, if it changed, as another opens.
func (v *View) flush(o Options) {
	if p := v.page; p != nil && p.ed != nil && (p.dirty || p.ed.changed) && !p.saving {
		v.write(o, p)
	}
}

func (v *View) write(o Options, p *page) {
	n := p.note
	n.Title, n.Icon, n.Body = strings.TrimSpace(p.title), p.icon, p.ed.markdown()
	p.saving, p.dirty, p.ed.changed = true, false, false
	o.Save(n, func(info proto.NoteInfo, err error) {
		p.saving = false
		if err != nil {
			v.err, p.dirty = err.Error(), true
			return
		}
		v.err = ""
		p.note.NoteInfo, p.note.Body = info, n.Body
	})
}

// newPage makes a page under parent ("" for the top) and opens it.
func (v *View) newPage(o Options, parent string) {
	if parent != "" {
		delete(v.folded, parent)
	}
	o.Save(proto.Note{NoteInfo: proto.NoteInfo{Parent: parent}}, func(info proto.NoteInfo, err error) {
		if err != nil {
			v.err = err.Error()
			return
		}
		v.Select(o, info.ID)
		if v.page != nil {
			v.page.note.NoteInfo = info
		}
	})
}

func (v *View) empty(c *ui.Context, o Options) {
	ui.Column(c).Grow(1).FillWidth().Center().Gap(12).Padding(32).Children(func() {
		ui.Icon(c, icon.Notebook).FontSize(30).TextColor(theme.Muted)
		msg := "Pages of notes, plans and checklists, kept on " + o.Host + ". Your agents read and write them too."
		if !o.Loaded && o.Err == "" {
			msg = "Reading the notes…"
		}
		ui.Text(c, msg).FontSize(12.5).TextColor(theme.Muted).TextAlign(ui.Center).MaxWidth(420)
		if o.Err != "" {
			ui.Text(c, o.Err).FontSize(12).TextColor(theme.Error).TextAlign(ui.Center)
		}
		if o.Loaded && widget.PrimaryButton(c, icon.Plus, "New page", "").Clicked() {
			v.newPage(o, "")
		}
	})
}

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
		more := widget.IconButton(c, icon.More, "More for this page", false)
		v.menu.OpenBelow(more, "page", 28)
		v.menu.Show(c, more, "page", func(done func()) { v.pageItems(c, o, p.note.NoteInfo, done) })
	})
	if p.ed == nil {
		ui.Text(c, "Opening…").FontSize(12).TextColor(theme.Muted).Padding(40)
		return
	}
	ui.Scroll(c).Grow(1).FillWidth().Children(func() {
		ui.Column(c).FillWidth().MaxWidth(760).AlignSelf(ui.Center).Padding(28, 48, 0, 48).Gap(6).Children(func() {
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

func titleOf(n proto.NoteInfo) string {
	if strings.TrimSpace(n.Title) == "" {
		return "Untitled"
	}
	return n.Title
}

// pageItems are a page's menu, in the tree and over the page.
func (v *View) pageItems(c *ui.Context, o Options, n proto.NoteInfo, done func()) {
	if widget.MenuItem(c, icon.Plus, "Add a page inside", false) {
		v.newPage(o, n.ID)
		done()
	}
	if n.Parent != "" && widget.MenuItem(c, icon.ArrowUp, "Move to the top level", false) {
		o.Move(n.ID, "", "", v.failed)
		done()
	}
	widget.MenuSeparator(c)
	if widget.MenuItem(c, icon.Trash, "Delete…", true) {
		v.deleting = &n
		done()
	}
}

func (v *View) failed(err error) {
	if err != nil {
		v.err = err.Error()
	}
}

// deleteDialog confirms deleting a page, and those under it.
func (v *View) deleteDialog(c *ui.Context, o Options) {
	if v.deleting == nil {
		return
	}
	n := *v.deleting
	under := 0
	for changed, gone := true, map[string]bool{n.ID: true}; changed; {
		changed = false
		for _, x := range o.Notes {
			if !gone[x.ID] && gone[x.Parent] {
				gone[x.ID], changed = true, true
				under++
			}
		}
	}
	msg := "It can't be brought back."
	if under > 0 {
		msg = "The pages under it go too. It can't be brought back."
	}
	open := true
	switch ui.AlertDialog(c, &open, "Delete “"+titleOf(n)+"”?", msg, "Cancel", "Delete") {
	case 1:
		if v.page != nil && v.page.id == n.ID {
			v.page, v.selected = nil, ""
		}
		o.Delete(n.ID, v.failed)
		v.deleting = nil
	case 0, ui.AlertDismissed:
		v.deleting = nil
	}
}
