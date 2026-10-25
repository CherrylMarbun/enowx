package notesview

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/enowx/enowx/proto"
	"github.com/enowx/enowx/ui/icon"
	"github.com/enowx/enowx/ui/theme"
	"github.com/enowx/enowx/ui/widget"
)

// noteDrag is what dragging a page in the tree carries.
// noteDrag is what dragging a page (id) or a group (group) in the tree
// carries: one type, as an element takes values of one.
type noteDrag struct{ id, group string }

// sidebar is the pages' tree: each under its parent, folding; a filter
// shows the pages whose titles match, with those they are under.
func (v *View) sidebar(c *ui.Context, o Options) {
	children := map[string][]proto.NoteInfo{}
	for _, n := range o.Notes {
		children[n.Parent] = append(children[n.Parent], n)
	}
	shown := v.matching(o)
	ui.Column(c.Key("notes-tree")).Width(248).Shrink(0).FillHeight().Background(theme.Panel).BorderWidth(0, 1, 0, 0).BorderColor(theme.Border).Children(func() {
		ui.Row(c).FillWidth().Height(44).Shrink(0).PaddingX(10).Gap(6).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(theme.Border).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, "Notes").FontSize(12.5).SingleLine()
				ui.Text(c, "On "+o.Host).FontSize(10.5).TextColor(theme.Muted).SingleLine().Shrink(1)
			})
			if o.SaveGroup != nil && o.Loaded && widget.IconButton(c, icon.FolderPlus, "New group", false).Clicked() {
				v.groupForm = &groupForm{open: true}
			}
			b := widget.Button(c, widget.Primary, icon.Plus, "New", "").Height(28).PaddingX(10).Tooltip("New page")
			if !o.Loaded {
				b.Disabled(true).Opacity(0.45)
			} else if b.Clicked() {
				v.newPage(o, "", v.groupFilter)
			}
		})
		ui.Row(c).FillWidth().Gap(6).Padding(8, 10, 4, 10).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() { widget.SearchField(c, &v.filter, "Filter pages", icon.Search) })
			v.groupFilterButton(c, o)
		})
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(6).Gap(1).Children(func() {
				if o.Loaded && len(o.Notes) == 0 {
					ui.Text(c, "No pages yet").FontSize(12).TextColor(theme.Muted).Padding(6)
				}
				var walk func(parent string, depth int, group string)
				walk = func(parent string, depth int, group string) {
					for _, n := range children[parent] {
						if shown != nil && !shown[n.ID] || depth == 0 && n.Group != group {
							continue
						}
						v.treeRow(c.Key(n.ID), o, n, depth, len(children[n.ID]) > 0, children[parent])
						if len(children[n.ID]) > 0 && (shown != nil || !v.folded[n.ID]) {
							walk(n.ID, depth+1, group)
						}
					}
				}
				if len(o.Groups) == 0 {
					walk("", 0, "")
					return
				}
				for _, g := range o.Groups {
					if v.groupFilter != "" && g.Name != v.groupFilter {
						continue
					}
					n := v.inGroup(o, g.Name, shown)
					if shown != nil && n == 0 {
						continue
					}
					v.groupHeader(c.Key("group:"+g.Name), o, g, n)
					if !g.Folded || shown != nil {
						ui.Column(c).FillWidth().Gap(1).Padding(0, 0, 0, 8).Children(func() { walk("", 0, g.Name) })
					}
				}
				if v.groupFilter == "" {
					v.ungroupedHeader(c, o, v.inGroup(o, "", shown))
					if !v.ungroupedFolded || shown != nil {
						walk("", 0, "")
					}
				}
			})
		})
	})
}

// matching is the pages a filter shows, nil without one.
func (v *View) matching(o Options) map[string]bool {
	q := strings.ToLower(strings.TrimSpace(v.filter))
	if q == "" {
		return nil
	}
	byID := map[string]proto.NoteInfo{}
	for _, n := range o.Notes {
		byID[n.ID] = n
	}
	shown := map[string]bool{}
	for _, n := range o.Notes {
		if strings.Contains(strings.ToLower(n.Title), q) {
			for x, ok := n, true; ok && !shown[x.ID]; x, ok = byID[x.Parent] {
				shown[x.ID] = true
			}
		}
	}
	return shown
}

// treeRow is a page in the tree: dragging it onto another's upper or
// lower edge puts it before or after that one, onto its middle under it.
func (v *View) treeRow(c *ui.Context, o Options, n proto.NoteInfo, depth int, hasChildren bool, siblings []proto.NoteInfo) {
	row := ui.ButtonBase(c).FillWidth().Height(30).Padding(0, 4, 0, 4+float32(depth)*14).Gap(4).Radius(6).
		Cursor(ui.CursorPointer).Label(titleOf(n)).AlignItems(ui.Center)
	row.Drag(noteDrag{id: n.ID})
	switch {
	case n.ID == v.selected:
		row.Background(theme.Selected)
	case row.Hovered():
		row.Background(theme.Hover)
	}
	v.dropOn(row, o, n, siblings)
	row.Children(func() {
		chev := ui.ButtonBase(c).Size(18, 18).Radius(4).Center().Shrink(0).Label("Fold " + titleOf(n))
		chev.Children(func() {
			if hasChildren {
				svg := icon.ChevronDown
				if v.folded[n.ID] {
					svg = icon.ChevronRight
				}
				ui.Icon(c, svg).FontSize(10.5).TextColor(theme.Muted)
			}
		})
		if hasChildren && chev.Clicked() {
			if v.folded == nil {
				v.folded = map[string]bool{}
			}
			v.folded[n.ID] = !v.folded[n.ID]
		}
		if n.Icon != "" {
			ui.Text(c, n.Icon).FontSize(13).Shrink(0)
		} else {
			ui.Icon(c, icon.File).FontSize(13).TextColor(theme.Muted).Shrink(0)
		}
		name := ui.Text(c, titleOf(n)).FontSize(12.5).TextColor(theme.Text).SingleLine().Shrink(1).Grow(1)
		if n.ID == v.selected {
			name.TextColor(theme.TextStrong)
		} else if strings.TrimSpace(n.Title) == "" {
			name.TextColor(theme.Muted)
		}
		if row.Hovered() {
			if widget.IconButton(c, icon.Plus, "Add a page inside", false).Clicked() {
				v.newPage(o, n.ID, "")
			}
		}
	})
	key := "note:" + n.ID
	v.menu.Open(row, key)
	v.menu.Show(c, row, key, func(done func()) { v.pageItems(c, o, n, done) })
	if row.Clicked() {
		v.Select(o, n.ID)
	}
}

// dropOn makes row take a page dropped on it, as treeRow says.
func (v *View) dropOn(row ui.Element, o Options, n proto.NoteInfo, siblings []proto.NoteInfo) {
	_, y, _ := row.PointerPosition()
	zone := 1 // 0 before, 1 inside, 2 after
	switch {
	case y < 8:
		zone = 0
	case y > 22:
		zone = 2
	}
	if d, ok := ui.Drop[noteDrag](row); ok && d.id != "" && d.id != n.ID {
		switch zone {
		case 0:
			o.Move(d.id, n.Parent, n.ID, v.failed)
		case 1:
			delete(v.folded, n.ID)
			o.Move(d.id, n.ID, "", v.failed)
		case 2:
			o.Move(d.id, n.Parent, after(siblings, n.ID, d.id), v.failed)
		}
	}
	if d, ok := ui.DragOver[noteDrag](row); ok && d.id != "" && d.id != n.ID {
		switch zone {
		case 0:
			row.BorderWidth(2, 0, 0, 0).BorderColor(theme.Accent)
		case 1:
			row.Background(theme.AccentSoft)
		case 2:
			row.BorderWidth(0, 0, 2, 0).BorderColor(theme.Accent)
		}
	}
}

// after is the sibling after id, which a page put after id goes before;
// "" when id is last. moving is left out, as it leaves its place.
func after(siblings []proto.NoteInfo, id, moving string) string {
	for i, s := range siblings {
		if s.ID != id {
			continue
		}
		for _, next := range siblings[i+1:] {
			if next.ID != moving {
				return next.ID
			}
		}
	}
	return ""
}
